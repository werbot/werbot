package access_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/access"
	"github.com/werbot/werbot/internal/platform/apperr"
	"github.com/werbot/werbot/internal/platform/db/dbtest"
)

// TestCreateAgentLocks: an owner or admin locks an agent, a run or the
// agents of a member of the organization, or all agents (no id); a target of
// another organization is not found, an all_agents lock with an id or an
// agent lock without one is invalid, a member never locks.
func TestCreateAgentLocks(t *testing.T) {
	svc, pool, f, _, _ := newTestService(t)
	admin := f.Principal(f.AdminID)
	agent := dbtest.Agent(t, pool, f.OrgID, f.MemberID, "claude")
	targets := map[string]uuid.UUID{
		"agent":          agent,
		"agent_run":      dbtest.AgentRun(t, pool, f.OrgID, agent, 10, 4),
		"agents_of_user": f.MemberID,
	}
	for typ, id := range targets {
		l, err := svc.CreateLock(bg, admin, access.LockInput{TargetType: typ, TargetID: &id, Reason: "agent incident"})
		require.NoError(t, err, typ)
		assert.Equal(t, typ, l.TargetType)
		assert.Equal(t, &id, l.TargetID, typ)
	}
	all, err := svc.CreateLock(bg, admin, access.LockInput{TargetType: "all_agents", Reason: "kill switch"})
	require.NoError(t, err)
	assert.Nil(t, all.TargetID, "all_agents has no target id")
	assert.Equal(t, []map[string]any{{"target_type": "all_agents", "target_id": nil}}, eventData(t, pool, "lock.create", all.ID))

	other := dbtest.Seed(t, pool)
	otherAgent := dbtest.Agent(t, pool, other.OrgID, other.MemberID, "claude")
	for typ, id := range map[string]uuid.UUID{
		"agent":          otherAgent,
		"agent_run":      dbtest.AgentRun(t, pool, other.OrgID, otherAgent, 10, 4),
		"agents_of_user": other.MemberID,
	} {
		_, err := svc.CreateLock(bg, admin, access.LockInput{TargetType: typ, TargetID: &id, Reason: "x"})
		require.ErrorIs(t, err, apperr.ErrNotFound, "target of another org: "+typ)
	}

	_, err = svc.CreateLock(bg, admin, access.LockInput{TargetType: "all_agents", TargetID: &agent, Reason: "x"})
	require.ErrorIs(t, err, apperr.ErrInvalidArgument, "all_agents with an id")
	for typ := range targets {
		_, err := svc.CreateLock(bg, admin, access.LockInput{TargetType: typ, Reason: "x"})
		require.ErrorIs(t, err, apperr.ErrInvalidArgument, typ+" without an id")
	}
	_, err = svc.CreateLock(bg, f.Principal(f.MemberID), access.LockInput{TargetType: "all_agents", Reason: "x"})
	require.ErrorIs(t, err, apperr.ErrPermissionDenied, "a member never locks agents")

	var n int
	require.NoError(t, pool.QueryRow(bg, "SELECT count(*) FROM lock").Scan(&n))
	assert.Equal(t, 4, n, "only the four valid agent locks are stored")
}
