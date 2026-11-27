package app

import (
	"context"
	"errors"
	"fmt"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/werbot/werbot/internal/agentic"
	"github.com/werbot/werbot/internal/platform/apperr"
	"github.com/werbot/werbot/internal/platform/sshca"
	"github.com/werbot/werbot/internal/tunnel"
)

// agentOf is the agent run of an agent certificate of sponsor: the run must
// be able to act (agentic.CheckRun) and be the certificate's own agent and
// sponsor's, else the certificate is ErrUnauthenticated.
func agentOf(ctx context.Context, agents *agentic.Service, orgID, sponsor uuid.UUID, id sshca.Identity) (*tunnel.AgentRef, error) {
	run, err := agents.CheckRun(ctx, orgID, *id.RunID)
	if err != nil {
		return nil, err
	}
	if run.AgentID != *id.AgentID || run.SponsorID != sponsor {
		return nil, fmt.Errorf("agent certificate of another run: %w", apperr.ErrUnauthenticated)
	}
	return &tunnel.AgentRef{AgentID: run.AgentID, RunID: run.ID, Limits: tunnel.AgentLimits{MaxRows: run.MaxRows,
		MaxResultBytes: run.MaxResultBytes, StatementTimeout: run.StatementTimeout, MaxQueries: run.MaxQueries}}, nil
}

// actionGuard counts an action of the agent run of st in its start
// transaction; nil for a person's session.
func actionGuard(agents *agentic.Service, st tunnel.SessionStart) func(context.Context, pgx.Tx) error {
	if st.AgentRunID == nil {
		return nil
	}
	in := agentic.ActionInput{OrgID: st.OrgID, RunID: *st.AgentRunID, UserID: st.UserID}
	return func(ctx context.Context, tx pgx.Tx) error { return agents.CountAction(ctx, tx, in) }
}

// actionError is the tunnel error of a refused start of an agent: the gateway
// tells a run limit and an inactive run from a policy denial by these texts.
// A frozen or locked agent, another user's run and a missing run or member
// are the run's state too, so they read as an inactive run, never as a denial
// that counts toward the pause.
func actionError(err error) error {
	switch {
	case errors.Is(err, agentic.ErrLimitExceeded):
		return connect.NewError(connect.CodeFailedPrecondition, errors.New("agent limit exceeded"))
	case errors.Is(err, agentic.ErrRunNotActive), errors.Is(err, apperr.ErrPermissionDenied), errors.Is(err, apperr.ErrNotFound):
		return connect.NewError(connect.CodeFailedPrecondition, errors.New("agent run is not active"))
	}
	return err
}

// tunnelAgents records the policy denials of agent runs for the tunnel.
type tunnelAgents struct{ agents *agentic.Service }

func (a tunnelAgents) RecordDenial(ctx context.Context, orgID uuid.UUID, d tunnel.AgentDenial) (bool, error) {
	return a.agents.RecordDenial(ctx, orgID, d.RunID, agentic.Denial{Resource: d.Resource, Command: d.Command,
		Classes: d.Classes, Reasons: d.Reasons})
}
