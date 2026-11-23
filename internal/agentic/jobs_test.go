package agentic_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/riverqueue/river"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/agentic"
	"github.com/werbot/werbot/internal/platform/db/dbtest"
	"github.com/werbot/werbot/internal/platform/jobs"
)

func TestExpireRuns(t *testing.T) {
	svc, pool, f := newTestService(t)
	agent := dbtest.Agent(t, pool, f.OrgID, f.MemberID, "claude")
	running, paused, future, ended := dbtest.AgentRun(t, pool, f.OrgID, agent, 10, 2), dbtest.AgentRun(t, pool, f.OrgID, agent, 10, 2),
		dbtest.AgentRun(t, pool, f.OrgID, agent, 10, 2), dbtest.AgentRun(t, pool, f.OrgID, agent, 10, 2)
	other := dbtest.Seed(t, pool)
	otherRun := dbtest.AgentRun(t, pool, other.OrgID, dbtest.Agent(t, pool, other.OrgID, other.MemberID, "claude"), 10, 2)
	pause(t, pool, paused, 3)
	end(t, pool, ended, "1 minute")
	exec(t, pool, "UPDATE agent_run SET expires_at = now() - interval '1 second' WHERE id = ANY($1)",
		[]uuid.UUID{running, paused, ended, otherRun})

	n, err := svc.ExpireRuns(bg)
	require.NoError(t, err)
	assert.EqualValues(t, 3, n, "running and paused runs past expires_at in every organization")
	expired := "expired"
	for _, id := range []uuid.UUID{running, paused, otherRun} {
		s := stateOf(t, pool, id)
		assert.Equal(t, "ended", s.Status)
		assert.Equal(t, &expired, s.EndReason)
		assert.True(t, s.Ended)
	}
	assert.Equal(t, "running", stateOf(t, pool, future).Status)
	assert.Equal(t, "client_exit", *stateOf(t, pool, ended).EndReason)

	got := events(t, pool, "agent.run.end")
	require.Len(t, got, 3)
	for _, e := range got {
		assert.Nil(t, e.Actor)
		assert.Equal(t, "system", e.ActorKind)
		assert.Equal(t, map[string]any{"reason": "expired"}, e.Data)
	}

	n, err = svc.ExpireRuns(bg)
	require.NoError(t, err)
	assert.Zero(t, n, "second run finds nothing")
}

func TestRegisterJobs(t *testing.T) {
	periodic := agentic.RegisterJobs(river.NewWorkers(), agentic.New(nil, nil, agentic.Config{}))
	assert.Len(t, periodic, 1)
	assert.Equal(t, "agent_run.expire", agentic.ExpireArgs{}.Kind())
	assert.Equal(t, jobs.QueueMaintenance, agentic.ExpireArgs{}.InsertOpts().Queue)
}
