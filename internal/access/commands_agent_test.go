package access_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/access"
	"github.com/werbot/werbot/internal/platform/db/dbtest"
)

// agentRunOf inserts an agent of sponsor and a running run of it.
func agentRunOf(t *testing.T, pool *pgxpool.Pool, f dbtest.Fixture, sponsor uuid.UUID, name string) *uuid.UUID {
	t.Helper()
	run := dbtest.AgentRun(t, pool, f.OrgID, dbtest.Agent(t, pool, f.OrgID, sponsor, name), 100, 4)
	return &run
}

// An agent's command approval is reused for the same run, resource and
// command while it is pending or approved within AgentApprovalReuse; the
// reused one moves to the asking instance. Anything else gets a new approval,
// and a person's request never reuses one.
func TestCommandApprovalReuse(t *testing.T) {
	svc, pool, f, q, _ := newTestService(t)
	run := agentRunOf(t, pool, f, f.MemberID, "a1")
	in := commandInput(f)
	in.AgentRunID = run
	first, err := svc.RequestCommandApproval(bg, in)
	require.NoError(t, err)
	assert.Equal(t, access.StatusPending, first.Status)
	q.Reset()

	again := in
	again.InstanceID = f.OtherInstanceID
	got, err := svc.RequestCommandApproval(bg, again)
	require.NoError(t, err)
	assert.Equal(t, first.ID, got.ID, "pending: reused")
	assert.Equal(t, access.StatusPending, got.Status)
	assert.Equal(t, &f.OtherInstanceID, got.InstanceID, "the decision goes to the asking instance")
	assert.Empty(t, q.Sent(), "a reused approval mails no one")

	_, err = svc.Decide(bg, f.Principal(f.OwnerID), first.ID, true, "")
	require.NoError(t, err)
	_, err = pool.Exec(bg, "UPDATE approval SET decided_at = now() - interval '9 minutes' WHERE id = $1", first.ID)
	require.NoError(t, err)
	got, err = svc.RequestCommandApproval(bg, in)
	require.NoError(t, err)
	assert.Equal(t, first.ID, got.ID, "approved 9 minutes ago: reused")
	assert.Equal(t, access.StatusApproved, got.Status)

	_, err = pool.Exec(bg, "UPDATE approval SET decided_at = now() - interval '11 minutes' WHERE id = $1", first.ID)
	require.NoError(t, err)
	fresh := func(in access.CommandApprovalInput, why string) access.Approval {
		t.Helper()
		a, err := svc.RequestCommandApproval(bg, in)
		require.NoError(t, err, why)
		assert.NotEqual(t, first.ID, a.ID, why)
		assert.Equal(t, access.StatusPending, a.Status, why)
		return a
	}
	denied := fresh(in, "approved 11 minutes ago")
	_, err = svc.Decide(bg, f.Principal(f.OwnerID), denied.ID, false, "")
	require.NoError(t, err)
	fresh(in, "denied")

	other := in
	other.AgentRunID = agentRunOf(t, pool, f, f.MemberID, "a2")
	fresh(other, "another run")
	other = in
	other.Command = "psql -c 'drop table orders'"
	fresh(other, "another command")
	other = in
	other.ResourceID = dbtest.Resource(t, pool, f.OrgID, "db", nil)
	fresh(other, "another resource")
	other = in
	other.UserID = f.OtherMemberID
	fresh(other, "another requester")

	human := commandInput(f)
	h1, err := svc.RequestCommandApproval(bg, human)
	require.NoError(t, err)
	h2, err := svc.RequestCommandApproval(bg, human)
	require.NoError(t, err)
	assert.NotEqual(t, h1.ID, h2.ID, "a person's request never reuses")
}

// An agent token sees as requestable only the resources of policies whose
// requesters include all agents.
func TestRequestableAgent(t *testing.T) {
	svc, pool, f, _, _ := newTestService(t)
	r2 := dbtest.Resource(t, pool, f.OrgID, "db", nil)
	f.ApprovalPolicy(t, pool, "people", access.ModeManual, onResource(f.ResourceID), allUsers, noOne)
	f.ApprovalPolicy(t, pool, "agents", access.ModeManual, onResource(r2), `{"all_agents": true}`, noOne)
	p := f.Principal(f.MemberID)
	p.AgentID, p.RunID = new(uuid.New()), agentRunOf(t, pool, f, f.MemberID, "a1")

	got, err := svc.Requestable(bg, p, []uuid.UUID{f.ResourceID, r2})
	require.NoError(t, err)
	assert.Equal(t, map[uuid.UUID]bool{f.ResourceID: false, r2: true}, got)
	got, err = svc.Requestable(bg, f.Principal(f.MemberID), []uuid.UUID{f.ResourceID, r2})
	require.NoError(t, err)
	assert.Equal(t, map[uuid.UUID]bool{f.ResourceID: true, r2: false}, got, "a person: policies of people only")
}
