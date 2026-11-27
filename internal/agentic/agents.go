package agentic

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/werbot/werbot/internal/agentic/dbq"
	"github.com/werbot/werbot/internal/platform/apperr"
	"github.com/werbot/werbot/internal/platform/principal"
)

// AgentView is an agent with its sponsor's login and its live runs.
type AgentView struct {
	Agent
	SponsorLogin string
	ActiveRuns   int
}

// CreateAgentInput is a new agent; a nil SponsorID is the caller.
type CreateAgentInput struct {
	Name      string
	SponsorID *uuid.UUID
	Limits    Limits
}

// UpdateAgentInput changes an agent: Limits nil and Status "" keep them;
// Status "frozen" freezes it, "active" unfreezes it.
type UpdateAgentInput struct {
	ID     uuid.UUID
	Limits *Limits
	Status string
}

var (
	errAgentToken  = fmt.Errorf("not allowed for agent tokens: %w", apperr.ErrPermissionDenied)
	errAgentExists = fmt.Errorf("agent name already exists: %w", apperr.ErrAlreadyExists)
	errForeignNew  = fmt.Errorf("a member creates only own agents: %w", apperr.ErrPermissionDenied)
)

// person refuses a run token: agents never manage agents or runs.
func person(p principal.Principal) error {
	if p.IsAgent() {
		return errAgentToken
	}
	return nil
}

// ownSponsor narrows a list to p's own agents or runs: nil (every sponsor)
// for owner and admin.
func ownSponsor(p principal.Principal) *uuid.UUID {
	if p.Manager() {
		return nil
	}
	return &p.UserID
}

// ListAgents lists the agents p manages: a member's own, every agent of the
// organization for owner and admin; by sponsor login and name.
func (s *Service) ListAgents(ctx context.Context, p principal.Principal) ([]AgentView, error) {
	if err := person(p); err != nil {
		return nil, err
	}
	var out []AgentView
	err := s.tx(ctx, p.OrgID, p.UserID, func(st Store, _ pgx.Tx) (err error) {
		out, err = listAgents(ctx, st, dbq.ListAgentsParams{OrgID: p.OrgID, SponsorUserID: ownSponsor(p)})
		return err
	})
	return out, err
}

// CreateAgent creates an agent (audit agent.create). A member creates only
// own agents; owner and admin name any member as sponsor. A taken name is
// ErrAlreadyExists, a sponsor outside the organization ErrNotFound.
func (s *Service) CreateAgent(ctx context.Context, p principal.Principal, in CreateAgentInput) (AgentView, error) {
	if err := person(p); err != nil {
		return AgentView{}, err
	}
	sponsor := *cmp.Or(in.SponsorID, &p.UserID)
	if sponsor != p.UserID && !p.Manager() {
		return AgentView{}, errForeignNew
	}
	if err := checkName(in.Name); err != nil {
		return AgentView{}, err
	}
	limits, err := marshalLimits(in.Limits)
	if err != nil {
		return AgentView{}, err
	}
	var out AgentView
	err = s.tx(ctx, p.OrgID, p.UserID, func(st Store, tx pgx.Tx) error {
		row, err := st.InsertAgent(ctx, dbq.InsertAgentParams{OrgID: p.OrgID, SponsorUserID: sponsor, Name: in.Name,
			Limits: limits, CreatedBy: &p.UserID})
		if errors.Is(err, pgx.ErrNoRows) {
			return errAgentExists
		}
		if err != nil {
			return err
		}
		if err := audit(ctx, tx, p.OrgID, "user", &p.UserID, "agent.create", "agent", row.ID,
			map[string]any{"name": in.Name, "sponsor_user_id": sponsor}); err != nil {
			return err
		}
		out, err = agentView(ctx, st, p.OrgID, row.ID)
		return err
	})
	if isForeignKeyViolation(err) {
		return AgentView{}, errSponsorNotFound
	}
	return out, err
}

// UpdateAgent changes the limits or status of an agent p manages (audit
// agent.update); limits apply to new runs only. Freezing revokes the agent's
// profiles in the same transaction. Another sponsor's agent is
// ErrNotFound for a member.
func (s *Service) UpdateAgent(ctx context.Context, p principal.Principal, in UpdateAgentInput) (AgentView, error) {
	if err := person(p); err != nil {
		return AgentView{}, err
	}
	limits, err := in.check()
	if err != nil {
		return AgentView{}, err
	}
	var out AgentView
	err = s.tx(ctx, p.OrgID, p.UserID, func(st Store, tx pgx.Tx) error {
		row, err := lockAgent(ctx, st, p, in.ID)
		if err != nil {
			return err
		}
		stored, status := limits, cmp.Or(in.Status, row.Status)
		if stored == nil {
			stored = row.Limits
		}
		if err := st.UpdateAgent(ctx, dbq.UpdateAgentParams{Limits: stored, Status: status, OrgID: p.OrgID, ID: in.ID}); err != nil {
			return err
		}
		if err := audit(ctx, tx, p.OrgID, "user", &p.UserID, "agent.update", "agent", in.ID,
			map[string]any{"limits": json.RawMessage(stored), "status": status}); err != nil {
			return err
		}
		if err := s.revokeProfiles(ctx, tx, p, in); err != nil {
			return err
		}
		out, err = agentView(ctx, st, p.OrgID, in.ID)
		return err
	})
	return out, err
}

// revokeProfiles ends the agent's profiles in tx when in freezes it; an
// unfreeze restores none.
func (s *Service) revokeProfiles(ctx context.Context, tx pgx.Tx, p principal.Principal, in UpdateAgentInput) error {
	if in.Status != "frozen" || s.cfg.RevokeProfiles == nil {
		return nil
	}
	return s.cfg.RevokeProfiles(ctx, tx, p.OrgID, in.ID, p.UserID, "agent_frozen")
}

// check validates in and returns its limits as agent.limits; nil: unchanged.
func (in UpdateAgentInput) check() ([]byte, error) {
	if in.Status != "" && in.Status != "active" && in.Status != "frozen" {
		return nil, fmt.Errorf("status must be active or frozen: %w", apperr.ErrInvalidArgument)
	}
	if in.Limits == nil {
		return nil, nil
	}
	return marshalLimits(*in.Limits)
}

// DeleteAgent deletes an agent p manages with its runs and tool calls (audit
// agent.delete); sessions and approvals of its runs stay without the run.
func (s *Service) DeleteAgent(ctx context.Context, p principal.Principal, id uuid.UUID) error {
	if err := person(p); err != nil {
		return err
	}
	return s.tx(ctx, p.OrgID, p.UserID, func(st Store, tx pgx.Tx) error {
		row, err := lockAgent(ctx, st, p, id)
		if err != nil {
			return err
		}
		if err := st.DeleteAgent(ctx, dbq.DeleteAgentParams{OrgID: p.OrgID, ID: id}); err != nil {
			return err
		}
		return audit(ctx, tx, p.OrgID, "user", &p.UserID, "agent.delete", "agent", id,
			map[string]any{"name": row.Name, "sponsor_user_id": row.SponsorUserID})
	})
}

// lockAgent reads an agent of p's organization FOR UPDATE; a missing agent
// and, for a member, another sponsor's agent are ErrNotFound.
func lockAgent(ctx context.Context, st Store, p principal.Principal, id uuid.UUID) (dbq.Agent, error) {
	row, err := st.LockAgent(ctx, dbq.LockAgentParams{OrgID: p.OrgID, ID: id})
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return row, errAgentNotFound
	case err != nil:
		return row, err
	case row.SponsorUserID != p.UserID && !p.Manager():
		return row, errAgentNotFound
	}
	return row, nil
}

func listAgents(ctx context.Context, st Store, arg dbq.ListAgentsParams) ([]AgentView, error) {
	rows, err := st.ListAgents(ctx, arg)
	if err != nil {
		return nil, err
	}
	out := make([]AgentView, 0, len(rows))
	for _, r := range rows {
		a, err := toAgent(r.Agent)
		if err != nil {
			return nil, err
		}
		out = append(out, AgentView{Agent: a, SponsorLogin: r.SponsorLogin, ActiveRuns: int(r.ActiveRuns)})
	}
	return out, nil
}

// agentView reads agent id of orgID with its sponsor's login and live runs.
func agentView(ctx context.Context, st Store, orgID, id uuid.UUID) (AgentView, error) {
	vs, err := listAgents(ctx, st, dbq.ListAgentsParams{OrgID: orgID, ID: &id})
	if err != nil {
		return AgentView{}, err
	}
	if len(vs) == 0 {
		return AgentView{}, errAgentNotFound
	}
	return vs[0], nil
}

// marshalLimits checks l and returns it as agent.limits.
func marshalLimits(l Limits) ([]byte, error) {
	if err := l.Validate(); err != nil {
		return nil, err
	}
	return json.Marshal(l.toJSON())
}
