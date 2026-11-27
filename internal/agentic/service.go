package agentic

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/werbot/werbot/internal/agentic/dbq"
	"github.com/werbot/werbot/internal/platform/apperr"
	"github.com/werbot/werbot/internal/platform/dbx"
	"github.com/werbot/werbot/internal/platform/principal"
)

// nameRe is the agent name rule of migration 00160 and agent.proto.
var nameRe = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,62}$`)

// checkName refuses an agent name outside nameRe.
func checkName(name string) error {
	if !nameRe.MatchString(name) {
		return fmt.Errorf("agent name must match %s: %w", nameRe, apperr.ErrInvalidArgument)
	}
	return nil
}

const (
	maxClient = 128  // characters of Run.Client
	maxReason = 1024 // characters of Run.Reason
)

var (
	errAgentNotFound   = fmt.Errorf("agent not found: %w", apperr.ErrNotFound)
	errRunNotFound     = fmt.Errorf("agent run not found: %w", apperr.ErrNotFound)
	errSponsorNotFound = fmt.Errorf("sponsor not found: %w", apperr.ErrNotFound)
)

// EnsureAgent returns the sponsor's agent of that name, creating it on first
// use (audit agent.create only then). The sponsor must be a member of orgID.
func (s *Service) EnsureAgent(ctx context.Context, orgID, sponsorID uuid.UUID, name string) (Agent, error) {
	if err := checkName(name); err != nil {
		return Agent{}, err
	}
	var out Agent
	err := s.tx(ctx, orgID, sponsorID, func(st Store, tx pgx.Tx) error {
		row, err := st.InsertAgent(ctx, dbq.InsertAgentParams{OrgID: orgID, SponsorUserID: sponsorID, Name: name,
			Limits: []byte("{}"), CreatedBy: &sponsorID})
		created := err == nil
		if errors.Is(err, pgx.ErrNoRows) { // the name exists: another call created it
			row, err = st.GetAgentByName(ctx, dbq.GetAgentByNameParams{OrgID: orgID, SponsorUserID: sponsorID, Name: name})
		}
		if err != nil {
			return err
		}
		if out, err = toAgent(row); err != nil || !created {
			return err
		}
		return audit(ctx, tx, orgID, "user", &sponsorID, "agent.create", "agent", out.ID,
			map[string]any{"name": name, "sponsor_user_id": sponsorID})
	})
	if isForeignKeyViolation(err) { // the sponsor is not a member of orgID
		return Agent{}, errSponsorNotFound
	}
	return out, err
}

// StartRun starts a run of agent a for its sponsor (audit agent.run.start).
// The agent's current status, limits and locks are read from the database: a
// frozen or locked agent is ErrPermissionDenied. client is 1–128 printable
// characters, reason at most 1024.
func (s *Service) StartRun(ctx context.Context, a Agent, client, reason string) (Run, error) {
	if err := validateRunInput(client, reason); err != nil {
		return Run{}, err
	}
	var out Run
	err := s.tx(ctx, a.OrgID, a.SponsorID, func(st Store, tx pgx.Tx) error {
		g, err := st.AgentGate(ctx, dbq.AgentGateParams{OrgID: a.OrgID, AgentID: a.ID})
		if errors.Is(err, pgx.ErrNoRows) {
			return errAgentNotFound
		}
		if err != nil {
			return err
		}
		if err := gate(g.Status, g.Locked); err != nil {
			return err
		}
		l, err := parseLimits(g.Limits)
		if err != nil {
			return fmt.Errorf("agentic: limits of agent %s: %w", a.ID, err)
		}
		l = l.WithDefaults()
		maxActions, maxConcurrent := int32(l.MaxActions), int32(l.MaxConcurrent) //nolint:gosec // parseLimits bounds them: <= 10000, <= 32
		row, err := st.InsertRun(ctx, dbq.InsertRunParams{OrgID: a.OrgID, AgentID: a.ID, SponsorUserID: g.SponsorUserID,
			Client: client, Reason: reason, MaxActions: maxActions, MaxConcurrent: maxConcurrent,
			MaxDurationSecs: l.MaxDuration.Seconds()})
		if err != nil {
			return err
		}
		out = toRun(row)
		return audit(ctx, tx, a.OrgID, "user", &g.SponsorUserID, "agent.run.start", "agent_run", out.ID,
			map[string]any{"agent_id": a.ID, "client": client, "reason": reason})
	})
	return out, err
}

func validateRunInput(client, reason string) error {
	n := utf8.RuneCountInString(client)
	if n == 0 || n > maxClient || !utf8.ValidString(client) || strings.ContainsFunc(client, func(r rune) bool { return !unicode.IsPrint(r) }) {
		return fmt.Errorf("client must be 1 to %d printable characters: %w", maxClient, apperr.ErrInvalidArgument)
	}
	if utf8.RuneCountInString(reason) > maxReason || !utf8.ValidString(reason) {
		return fmt.Errorf("reason must be at most %d characters: %w", maxReason, apperr.ErrInvalidArgument)
	}
	return nil
}

// gate refuses a frozen or locked agent.
func gate(status string, locked bool) error {
	switch {
	case status != "active":
		return errFrozen
	case locked:
		return errLocked
	}
	return nil
}

// CheckRun returns the run when it may act: running, not expired, its agent
// active and no lock of the run, agent, sponsor or all agents. A frozen or
// locked agent is ErrPermissionDenied, an inactive run ErrRunNotActive.
func (s *Service) CheckRun(ctx context.Context, orgID, runID uuid.UUID) (Run, error) {
	var g dbq.RunGateRow
	err := dbx.ReadInOrg(s.pool, orgID, uuid.Nil, func(rd *dbx.Read) (err error) {
		g, err = newStore(rd).RunGate(ctx, dbq.RunGateParams{OrgID: orgID, ID: runID})
		return err
	})
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return Run{}, errRunNotFound
	case err != nil:
		return Run{}, err
	}
	if err := gate(g.AgentStatus, g.Locked); err != nil {
		return Run{}, err
	}
	if !g.Active {
		return Run{}, ErrRunNotActive
	}
	return withDBLimits(toRun(g.AgentRun), g.AgentLimits)
}

// withDBLimits sets the database limits of run from the agent's limits, the
// defaults applied; stored limits out of range refuse the run.
func withDBLimits(run Run, raw []byte) (Run, error) {
	l, err := parseLimits(raw)
	if err != nil {
		return Run{}, fmt.Errorf("agentic: limits of agent %s: %w", run.AgentID, err)
	}
	l = l.WithDefaults()
	run.MaxRows, run.MaxResultBytes, run.StatementTimeout, run.MaxQueries = l.MaxRows, l.MaxResultBytes, l.StatementTimeout, l.MaxQueries
	return run, nil
}

// EndRun ends a run with reason (audit agent.run.end); an ended run stays as
// it is. The actor is the principal in ctx, else the system.
func (s *Service) EndRun(ctx context.Context, orgID, runID uuid.UUID, reason string) error {
	kind, actor, user := "system", (*uuid.UUID)(nil), uuid.Nil
	if p, ok := principal.From(ctx); ok {
		kind, actor, user = "user", &p.UserID, p.UserID
	}
	return s.tx(ctx, orgID, user, func(st Store, tx pgx.Tx) error {
		run, err := lockRun(ctx, st, orgID, runID)
		if err != nil || run.Status == "ended" {
			return err
		}
		if err := st.EndRun(ctx, dbq.EndRunParams{OrgID: orgID, ID: runID, EndReason: &reason}); err != nil {
			return err
		}
		return audit(ctx, tx, orgID, kind, actor, "agent.run.end", "agent_run", runID, map[string]any{"reason": reason})
	})
}

// ResumeRun moves a paused run of p's organization back to running and
// clears its denial counter (audit agent.run.resume). A member resumes only
// own runs: another sponsor's run is ErrNotFound; a run token never resumes.
func (s *Service) ResumeRun(ctx context.Context, p principal.Principal, runID uuid.UUID) error {
	if err := person(p); err != nil {
		return err
	}
	return s.tx(ctx, p.OrgID, p.UserID, func(st Store, tx pgx.Tx) error {
		run, err := lockRun(ctx, st, p.OrgID, runID)
		if err != nil {
			return err
		}
		if !sees(p, runID, run.SponsorUserID) {
			return errRunNotFound
		}
		if run.Status != "paused" {
			return fmt.Errorf("agent run is not paused: %w", apperr.ErrFailedPrecondition)
		}
		if err := st.ResumeRun(ctx, dbq.ResumeRunParams{OrgID: p.OrgID, ID: runID}); err != nil {
			return err
		}
		return audit(ctx, tx, p.OrgID, "user", &p.UserID, "agent.run.resume", "agent_run", runID, nil)
	})
}

// lockRun reads a run of orgID FOR UPDATE; a missing run is ErrNotFound.
func lockRun(ctx context.Context, st Store, orgID, runID uuid.UUID) (dbq.AgentRun, error) {
	run, err := st.LockRun(ctx, dbq.LockRunParams{OrgID: orgID, ID: runID})
	if errors.Is(err, pgx.ErrNoRows) {
		return run, errRunNotFound
	}
	return run, err
}

// AgentsForSnapshot lists the agents of orgID and its blocked runs (paused,
// or ended within the last hour) for the policy snapshot.
func (s *Service) AgentsForSnapshot(ctx context.Context, orgID uuid.UUID) ([]SnapshotAgent, []uuid.UUID, error) {
	var agents []SnapshotAgent
	var blocked []uuid.UUID
	err := s.tx(ctx, orgID, uuid.Nil, func(st Store, _ pgx.Tx) error {
		rows, err := st.SnapshotAgents(ctx, orgID)
		if err != nil {
			return err
		}
		for _, r := range rows {
			agents = append(agents, SnapshotAgent{ID: r.ID, SponsorID: r.SponsorUserID, Status: r.Status})
		}
		blocked, err = st.BlockedRuns(ctx, orgID)
		return err
	})
	return agents, blocked, err
}
