package agentic

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/werbot/werbot/internal/agentic/dbq"
	"github.com/werbot/werbot/internal/platform/apperr"
	"github.com/werbot/werbot/internal/platform/principal"
)

// ListRuns paging.
const (
	defaultRunPage = 50
	maxRunPage     = 100
	maxPage        = 10_000_000 // (maxPage-1)*maxRunPage fits int32
)

// RunView is a run with its agent's name.
type RunView struct {
	Run
	AgentName string
}

// RunFilter narrows ListRuns: AgentID nil and Status "" match any. Page 0
// is 1; PageSize 0 is 50, at most 100.
type RunFilter struct {
	AgentID        *uuid.UUID
	Status         string
	Page, PageSize int
}

// Timeline is a run with its tool calls, sessions, approvals and denials,
// each oldest first and at most 1000 long.
type Timeline struct {
	Run       RunView
	ToolCalls []dbq.RunToolCallsRow // Arguments is JSON
	Sessions  []dbq.RunSessionsRow
	Approvals []dbq.RunApprovalsRow
	Denials   []RunDenial
}

// RunDenial is a policy denial of a run (audit session.deny).
type RunDenial struct {
	Resource, Command string
	Classes, Reasons  []string
	At                time.Time
}

// sees reports whether p may see run runID of sponsor: a run token only its
// own run, a member own runs, owner and admin any.
func sees(p principal.Principal, runID, sponsor uuid.UUID) bool {
	if p.RunID != nil {
		return *p.RunID == runID
	}
	return sponsor == p.UserID || p.Manager()
}

// ListRuns returns one page of the runs p may see, newest first, and the next
// page number (0 after the last); a run token never lists runs.
func (s *Service) ListRuns(ctx context.Context, p principal.Principal, f RunFilter) ([]RunView, int, error) {
	if err := person(p); err != nil {
		return nil, 0, err
	}
	if f.Page < 0 || f.Page > maxPage || f.PageSize < 0 {
		return nil, 0, fmt.Errorf("invalid run filter: %w", apperr.ErrInvalidArgument)
	}
	page, size := max(f.Page, 1), min(f.PageSize, maxRunPage)
	if size == 0 {
		size = defaultRunPage
	}
	arg := dbq.ListRunsParams{OrgID: p.OrgID, SponsorUserID: ownSponsor(p), AgentID: f.AgentID, Status: f.Status,
		RowOffset: int32((page - 1) * size), RowLimit: int32(size + 1)} //nolint:gosec // page <= maxPage, size <= maxRunPage
	var out []RunView
	err := s.tx(ctx, p.OrgID, p.UserID, func(st Store, _ pgx.Tx) (err error) {
		out, err = listRuns(ctx, st, arg)
		return err
	})
	next := 0
	if len(out) > size { // the extra row proves a next page
		out, next = out[:size], page+1
	}
	return out, next, err
}

// Timeline returns run runID with everything it did, if p may see it (else
// ErrNotFound).
func (s *Service) Timeline(ctx context.Context, p principal.Principal, runID uuid.UUID) (Timeline, error) {
	var out Timeline
	err := s.tx(ctx, p.OrgID, p.UserID, func(st Store, _ pgx.Tx) error {
		run, err := visibleRun(ctx, st, p, runID)
		if err != nil {
			return err
		}
		out, err = timeline(ctx, st, p.OrgID, run)
		return err
	})
	return out, err
}

// StopRun ends a run p may see: as client_exit for its own run token, else
// as ended_by_user.
func (s *Service) StopRun(ctx context.Context, p principal.Principal, runID uuid.UUID) error {
	err := s.tx(ctx, p.OrgID, p.UserID, func(st Store, _ pgx.Tx) error {
		_, err := visibleRun(ctx, st, p, runID)
		return err
	})
	if err != nil {
		return err
	}
	reason := "ended_by_user"
	if p.RunID != nil {
		reason = "client_exit"
	}
	return s.EndRun(ctx, p.OrgID, runID, reason)
}

// visibleRun reads run runID of p's organization; ErrNotFound when it is
// missing or p may not see it.
func visibleRun(ctx context.Context, st Store, p principal.Principal, runID uuid.UUID) (RunView, error) {
	rs, err := listRuns(ctx, st, dbq.ListRunsParams{OrgID: p.OrgID, ID: &runID, RowLimit: 1})
	if err != nil {
		return RunView{}, err
	}
	if len(rs) == 0 || !sees(p, runID, rs[0].SponsorID) {
		return RunView{}, errRunNotFound
	}
	return rs[0], nil
}

func listRuns(ctx context.Context, st Store, arg dbq.ListRunsParams) ([]RunView, error) {
	rows, err := st.ListRuns(ctx, arg)
	if err != nil {
		return nil, err
	}
	out := make([]RunView, len(rows))
	for i, r := range rows {
		out[i] = RunView{Run: toRun(r.AgentRun), AgentName: r.AgentName}
	}
	return out, nil
}

// timeline reads the tool calls, sessions, approvals and denials of run.
func timeline(ctx context.Context, st Store, orgID uuid.UUID, run RunView) (Timeline, error) {
	out := Timeline{Run: run}
	var err error
	if out.ToolCalls, err = st.RunToolCalls(ctx, dbq.RunToolCallsParams{OrgID: orgID, RunID: run.ID}); err != nil {
		return out, err
	}
	if out.Sessions, err = st.RunSessions(ctx, dbq.RunSessionsParams{OrgID: orgID, RunID: &run.ID}); err != nil {
		return out, err
	}
	if out.Approvals, err = st.RunApprovals(ctx, dbq.RunApprovalsParams{OrgID: orgID, RunID: &run.ID}); err != nil {
		return out, err
	}
	out.Denials, err = runDenials(ctx, st, orgID, run.ID)
	return out, err
}

// runDenials reads the session.deny events of run runID.
func runDenials(ctx context.Context, st Store, orgID, runID uuid.UUID) ([]RunDenial, error) {
	rows, err := st.RunDenials(ctx, dbq.RunDenialsParams{OrgID: &orgID, RunID: runID.String()})
	if err != nil {
		return nil, err
	}
	out := make([]RunDenial, len(rows))
	for i, r := range rows {
		var d struct {
			Target           string
			Classes, Reasons []string
		}
		if err := json.Unmarshal(r.Data, &d); err != nil {
			return nil, fmt.Errorf("agentic: denial data: %w", err)
		}
		out[i] = RunDenial{Resource: r.Resource, Command: d.Target, Classes: d.Classes, Reasons: d.Reasons, At: r.CreatedAt}
	}
	return out, nil
}
