package access_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/access"
	"github.com/werbot/werbot/internal/platform/apperr"
	"github.com/werbot/werbot/internal/platform/db/dbtest"
	"github.com/werbot/werbot/internal/platform/principal"
)

// runPrincipal is the run token principal of a new run of a new agent of
// sponsor in f's organization: the sponsor's principal with AgentID and RunID.
func runPrincipal(t *testing.T, pool *pgxpool.Pool, f dbtest.Fixture, sponsor uuid.UUID, name string) principal.Principal {
	t.Helper()
	agent := dbtest.Agent(t, pool, f.OrgID, sponsor, name)
	run := dbtest.AgentRun(t, pool, f.OrgID, agent, 10, 4)
	p := f.Principal(sponsor)
	p.AgentID, p.RunID = &agent, &run
	return p
}

// TestAgentRequestAccess: an access request of a run token stores the run
// and matches only policies of all agents; with none it is refused and
// nothing is stored.
func TestAgentRequestAccess(t *testing.T) {
	svc, pool, f, _, _ := newTestService(t)
	f.ApprovalPolicy(t, pool, "people", access.ModeManual, onResource(f.ResourceID), allUsers, noOne)
	p := runPrincipal(t, pool, f, f.MemberID, "claude")
	in := access.CreateRequestInput{ResourceID: &f.ResourceID, Duration: time.Hour, Reason: "deploy"}

	_, err := svc.CreateRequest(bg, p, in)
	require.ErrorIs(t, err, apperr.ErrPermissionDenied, "a policy of people only does not match an agent run")
	assert.ErrorContains(t, err, "no approval policy matches this request")
	var n int
	require.NoError(t, pool.QueryRow(bg, "SELECT count(*) FROM approval").Scan(&n))
	assert.Zero(t, n)

	f.ApprovalPolicy(t, pool, "agents", access.ModeManual, onResource(f.ResourceID), `{"all_agents": true}`, noOne)
	a, err := svc.CreateRequest(bg, p, in)
	require.NoError(t, err)
	assert.Equal(t, p.RunID, a.AgentRunID)
	assert.Equal(t, f.MemberID, a.RequesterID, "the sponsor requests")
	var stored *uuid.UUID
	require.NoError(t, pool.QueryRow(bg, "SELECT agent_run_id FROM approval WHERE id = $1", a.ID).Scan(&stored))
	assert.Equal(t, p.RunID, stored)
}

// TestAgentRequestsScopedToRun: a run token lists, reads and cancels only the
// requests of its own run, never its sponsor's own requests or other runs',
// whatever the sponsor's role.
func TestAgentRequestsScopedToRun(t *testing.T) {
	svc, pool, f, _, _ := newTestService(t)
	f.ApprovalPolicy(t, pool, "all", access.ModeManual, onResource(f.ResourceID), `{"all_users": true, "all_agents": true}`, noOne)
	in := access.CreateRequestInput{ResourceID: &f.ResourceID, Duration: time.Hour, Reason: "x"}
	mine := runPrincipal(t, pool, f, f.AdminID, "claude")
	sibling := runPrincipal(t, pool, f, f.AdminID, "codex")
	own, err := svc.CreateRequest(bg, mine, in)
	require.NoError(t, err)
	human, err := svc.CreateRequest(bg, f.Principal(f.AdminID), in)
	require.NoError(t, err)
	other, err := svc.CreateRequest(bg, sibling, in)
	require.NoError(t, err)

	for _, scope := range []string{access.ScopeMine, access.ScopeAll, access.ScopeToApprove} {
		vs, _, err := svc.ListRequests(bg, mine, access.RequestFilter{Scope: scope})
		require.NoError(t, err, scope)
		for _, v := range vs {
			assert.Equal(t, own.ID, v.ID, "scope %s lists only the run's requests", scope)
		}
	}
	vs, _, err := svc.ListRequests(bg, mine, access.RequestFilter{Scope: access.ScopeMine})
	require.NoError(t, err)
	require.Len(t, vs, 1)

	_, err = svc.GetRequest(bg, mine, own.ID)
	require.NoError(t, err)
	for _, id := range []uuid.UUID{human.ID, other.ID} {
		_, err = svc.GetRequest(bg, mine, id)
		require.ErrorIs(t, err, apperr.ErrNotFound, "a request of the sponsor or another run")
		require.ErrorIs(t, svc.Cancel(bg, mine, id), apperr.ErrNotFound, "a request of the sponsor or another run")
	}
	require.NoError(t, svc.Cancel(bg, mine, own.ID))

	vs, _, err = svc.ListRequests(bg, f.Principal(f.AdminID), access.RequestFilter{Scope: access.ScopeMine})
	require.NoError(t, err)
	assert.Len(t, vs, 3, "the sponsor's own token sees every request it made")
}
