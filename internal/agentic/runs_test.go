package agentic_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/agentic"
	"github.com/werbot/werbot/internal/platform/apperr"
	"github.com/werbot/werbot/internal/platform/db/dbtest"
	"github.com/werbot/werbot/internal/platform/principal"
)

func TestCheckRun(t *testing.T) {
	svc, pool, f := newTestService(t)
	agent := dbtest.Agent(t, pool, f.OrgID, f.MemberID, "claude")
	newRun := func() uuid.UUID { return dbtest.AgentRun(t, pool, f.OrgID, agent, 10, 2) }

	id := newRun()
	run, err := svc.CheckRun(bg, f.OrgID, id)
	require.NoError(t, err)
	assert.Equal(t, agentic.Run{ID: id, OrgID: f.OrgID, AgentID: agent, SponsorID: f.MemberID, Client: "test",
		Status: "running", MaxActions: 10, MaxConcurrent: 2, ExpiresAt: run.ExpiresAt, StartedAt: run.StartedAt}, run)

	paused, ended, expired := newRun(), newRun(), newRun()
	pause(t, pool, paused, 5)
	end(t, pool, ended, "1 minute")
	exec(t, pool, "UPDATE agent_run SET expires_at = now() - interval '1 second' WHERE id = $1", expired)
	for name, id := range map[string]uuid.UUID{"paused": paused, "ended": ended, "expired": expired} {
		_, err := svc.CheckRun(bg, f.OrgID, id)
		require.ErrorIs(t, err, agentic.ErrRunNotActive, name)
		assert.ErrorIs(t, err, apperr.ErrFailedPrecondition, name)
		assert.ErrorContains(t, err, "agent run is not active", name)
	}

	_, err = svc.CheckRun(bg, dbtest.Seed(t, pool).OrgID, id)
	require.ErrorIs(t, err, apperr.ErrNotFound, "run of another organization")

	locked := newRun()
	f.Lock(t, pool, "agent_run", &locked)
	_, err = svc.CheckRun(bg, f.OrgID, locked)
	require.ErrorIs(t, err, apperr.ErrPermissionDenied)
	assert.ErrorContains(t, err, "agent is locked")
	_, err = svc.CheckRun(bg, f.OrgID, id)
	require.NoError(t, err, "a lock of another run does not apply")

	exec(t, pool, "UPDATE agent SET status = 'frozen' WHERE id = $1", agent)
	_, err = svc.CheckRun(bg, f.OrgID, id)
	require.ErrorIs(t, err, apperr.ErrPermissionDenied)
	assert.ErrorContains(t, err, "agent is frozen")
}

func TestEndRunIdempotent(t *testing.T) {
	svc, pool, f := newTestService(t)
	run := dbtest.AgentRun(t, pool, f.OrgID, dbtest.Agent(t, pool, f.OrgID, f.MemberID, "claude"), 10, 2)
	ctx := principal.With(bg, f.Principal(f.MemberID))

	require.NoError(t, svc.EndRun(ctx, f.OrgID, run, "client_exit"))
	require.NoError(t, svc.EndRun(ctx, f.OrgID, run, "ended_by_user"), "an ended run stays ended")
	reason := "client_exit"
	assert.Equal(t, runState{Status: "ended", EndReason: &reason, Ended: true}, stateOf(t, pool, run))
	ended := events(t, pool, "agent.run.end")
	require.Len(t, ended, 1)
	assert.Equal(t, event{Actor: &f.MemberID, ActorKind: "user", TargetID: run.String(),
		Data: map[string]any{"reason": "client_exit"}}, ended[0])

	assert.ErrorIs(t, svc.EndRun(ctx, f.OrgID, uuid.New(), "client_exit"), apperr.ErrNotFound)
	other := dbtest.Seed(t, pool)
	otherRun := dbtest.AgentRun(t, pool, other.OrgID, dbtest.Agent(t, pool, other.OrgID, other.MemberID, "claude"), 10, 2)
	assert.ErrorIs(t, svc.EndRun(ctx, f.OrgID, otherRun, "client_exit"), apperr.ErrNotFound, "run of another organization")
	assert.Equal(t, "running", stateOf(t, pool, otherRun).Status)
}

func TestResumeRun(t *testing.T) {
	svc, pool, f := newTestService(t)
	run := dbtest.AgentRun(t, pool, f.OrgID, dbtest.Agent(t, pool, f.OrgID, f.MemberID, "claude"), 10, 2)

	pause(t, pool, run, 5)
	require.NoError(t, svc.ResumeRun(bg, f.Principal(f.MemberID), run))
	assert.Equal(t, runState{Status: "running"}, stateOf(t, pool, run))
	resumed := events(t, pool, "agent.run.resume")
	require.Len(t, resumed, 1)
	assert.Equal(t, event{Actor: &f.MemberID, ActorKind: "user", TargetID: run.String(), Data: map[string]any{}}, resumed[0])

	assert.ErrorIs(t, svc.ResumeRun(bg, f.Principal(f.MemberID), run), apperr.ErrFailedPrecondition, "running run")

	pause(t, pool, run, 5)
	assert.ErrorIs(t, svc.ResumeRun(bg, f.Principal(f.OtherMemberID), run), apperr.ErrNotFound, "member on another sponsor's run")
	other := dbtest.Seed(t, pool)
	assert.ErrorIs(t, svc.ResumeRun(bg, other.Principal(other.OwnerID), run), apperr.ErrNotFound, "owner of another organization")
	assert.Equal(t, "paused", stateOf(t, pool, run).Status)

	require.NoError(t, svc.ResumeRun(bg, f.Principal(f.AdminID), run), "admin resumes any run")
	assert.Equal(t, runState{Status: "running"}, stateOf(t, pool, run))
	assert.Len(t, events(t, pool, "agent.run.resume"), 2)

	end(t, pool, run, "1 minute")
	assert.ErrorIs(t, svc.ResumeRun(bg, f.Principal(f.MemberID), run), apperr.ErrFailedPrecondition, "ended run")
}

func TestAgentsForSnapshot(t *testing.T) {
	svc, pool, f := newTestService(t)
	active := dbtest.Agent(t, pool, f.OrgID, f.MemberID, "claude")
	frozen := dbtest.Agent(t, pool, f.OrgID, f.OtherMemberID, "codex")
	exec(t, pool, "UPDATE agent SET status = 'frozen' WHERE id = $1", frozen)
	paused, recent, old := dbtest.AgentRun(t, pool, f.OrgID, active, 10, 2),
		dbtest.AgentRun(t, pool, f.OrgID, active, 10, 2), dbtest.AgentRun(t, pool, f.OrgID, active, 10, 2)
	dbtest.AgentRun(t, pool, f.OrgID, active, 10, 2) // running
	pause(t, pool, paused, 5)
	end(t, pool, recent, "10 minutes")
	end(t, pool, old, "2 hours")
	other := dbtest.Seed(t, pool)
	otherRun := dbtest.AgentRun(t, pool, other.OrgID, dbtest.Agent(t, pool, other.OrgID, other.MemberID, "claude"), 10, 2)
	pause(t, pool, otherRun, 5)

	agents, blocked, err := svc.AgentsForSnapshot(bg, f.OrgID)
	require.NoError(t, err)
	assert.ElementsMatch(t, []agentic.SnapshotAgent{
		{ID: active, SponsorID: f.MemberID, Status: "active"},
		{ID: frozen, SponsorID: f.OtherMemberID, Status: "frozen"},
	}, agents)
	assert.ElementsMatch(t, []uuid.UUID{paused, recent}, blocked)
}
