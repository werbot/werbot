package app

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/netip"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/werbot/werbot/internal/agentic"
	"github.com/werbot/werbot/internal/identity/oauth"
	"github.com/werbot/werbot/internal/inventory"
	"github.com/werbot/werbot/internal/platform/apperr"
	"github.com/werbot/werbot/internal/platform/principal"
	"github.com/werbot/werbot/internal/policy"
)

// errRunNotStarted is what the token endpoint tells the client of a failed
// run start: the cause stays in the server log.
var errRunNotStarted = errors.New("agent run not started")

// oauthAgents starts agent runs for the token exchange (oauth.Agents).
type oauthAgents struct {
	agents *agentic.Service
	logger *slog.Logger
}

// StartAgentRun creates the sponsor's agent name on first use and starts a
// run. A frozen or locked agent stays apperr.ErrPermissionDenied (OAuth
// invalid_grant); any other error is logged and answered without details.
func (a oauthAgents) StartAgentRun(ctx context.Context, orgID, userID uuid.UUID, name, client, reason string) (agentID, runID uuid.UUID, expiresAt time.Time, err error) {
	agent, err := a.agents.EnsureAgent(ctx, orgID, userID, name)
	var run agentic.Run
	if err == nil {
		run, err = a.agents.StartRun(ctx, agent, client, reason)
	}
	switch {
	case errors.Is(err, apperr.ErrPermissionDenied):
		return uuid.Nil, uuid.Nil, time.Time{}, apperr.ErrPermissionDenied
	case err != nil:
		a.logger.WarnContext(ctx, "agent run start failed", "org_id", orgID, "err", err)
		return uuid.Nil, uuid.Nil, time.Time{}, errRunNotStarted
	}
	return agent.ID, run.ID, run.ExpiresAt, nil
}

// agentProfiles revokes the agent profiles of a frozen agent (agentic.ProfileRevoker).
type agentProfiles struct{}

func (agentProfiles) RevokeAgentProfiles(ctx context.Context, tx pgx.Tx, orgID, agentID uuid.UUID, actor *uuid.UUID, reason string) error {
	return oauth.RevokeAgentProfiles(ctx, tx, orgID, agentID, actor, reason)
}

// policyAgents is the agents of agentic as a policy.AgentSource.
type policyAgents struct{ agents *agentic.Service }

func (p policyAgents) PolicyAgents(ctx context.Context, orgID uuid.UUID) ([]policy.SnapshotAgent, []uuid.UUID, error) {
	as, blocked, err := p.agents.AgentsForSnapshot(ctx, orgID)
	if err != nil {
		return nil, nil, err
	}
	out := make([]policy.SnapshotAgent, len(as))
	for i, a := range as {
		out[i] = policy.SnapshotAgent{ID: a.ID, SponsorID: a.SponsorID, Status: a.Status}
	}
	return out, blocked, nil
}

// accessibleResources is the policy as inventory.Accessible.
type accessibleResources struct{ pol *policy.Service }

func (a accessibleResources) AccessibleResources(ctx context.Context, orgID, userID uuid.UUID, ip netip.Addr, at time.Time) ([]uuid.UUID, error) {
	return a.pol.AccessibleResources(ctx, orgID, userID, ip, at)
}

func (a accessibleResources) AgentResources(ctx context.Context, orgID, agentID uuid.UUID, ip netip.Addr, at time.Time) (map[uuid.UUID]inventory.AgentAccess, error) {
	rs, err := a.pol.AgentResources(ctx, orgID, agentID, ip, at)
	if err != nil {
		return nil, err
	}
	out := make(map[uuid.UUID]inventory.AgentAccess, len(rs))
	for id, c := range rs {
		out[id] = inventory.AgentAccess{Allowed: c.Allowed, Approval: c.Approval}
	}
	return out, nil
}

// agentRoutes answers 403 "not allowed for agent tokens" to a run token on
// every route that is not a Connect procedure (recordings, the SPA, OAuth
// endpoints); agentic.RestrictAgentTokens decides the procedures. It runs
// inside identity.Middleware, which sets the principal.
func agentRoutes(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if p, ok := principal.From(r.Context()); ok && (p.AgentID != nil || p.RunID != nil) && !strings.HasPrefix(r.URL.Path, "/werbot.v1.") {
			http.Error(w, "not allowed for agent tokens", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}
