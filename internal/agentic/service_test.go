package agentic_test

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/agentic"
	"github.com/werbot/werbot/internal/platform/apperr"
	"github.com/werbot/werbot/internal/platform/db/dbtest"
)

func TestEnsureAgentIdempotent(t *testing.T) {
	svc, pool, f := newTestService(t)

	a, err := svc.EnsureAgent(bg, f.OrgID, f.MemberID, "claude")
	require.NoError(t, err)
	again, err := svc.EnsureAgent(bg, f.OrgID, f.MemberID, "claude")
	require.NoError(t, err)
	assert.Equal(t, a, again)
	assert.Equal(t, agentic.Agent{ID: a.ID, OrgID: f.OrgID, SponsorID: f.MemberID, Name: "claude", Kind: "interactive",
		Status: "active", CreatedAt: a.CreatedAt}, a)

	created := events(t, pool, "agent.create")
	require.Len(t, created, 1, "only the insert is audited")
	assert.Equal(t, event{Actor: &f.MemberID, ActorKind: "user", TargetID: a.ID.String(),
		Data: map[string]any{"name": "claude", "sponsor_user_id": f.MemberID.String()}}, created[0])

	other, err := svc.EnsureAgent(bg, f.OrgID, f.OtherMemberID, "claude")
	require.NoError(t, err)
	assert.NotEqual(t, a.ID, other.ID, "another sponsor gets its own agent")

	for _, name := range []string{"Claude", strings.Repeat("a", 64), "", "-x", "a b"} {
		_, err := svc.EnsureAgent(bg, f.OrgID, f.MemberID, name)
		assert.ErrorIs(t, err, apperr.ErrInvalidArgument, name)
	}
	_, err = svc.EnsureAgent(bg, f.OrgID, uuid.New(), "claude")
	assert.ErrorIs(t, err, apperr.ErrNotFound, "sponsor outside the organization")
	assert.Len(t, events(t, pool, "agent.create"), 2)
}

func TestStartRunDefaultsAndOverrides(t *testing.T) {
	svc, pool, f := newTestService(t)
	a, err := svc.EnsureAgent(bg, f.OrgID, f.MemberID, "claude")
	require.NoError(t, err)

	run, err := svc.StartRun(bg, a, "claude-code", "deploy fix")
	require.NoError(t, err)
	assert.Equal(t, agentic.Run{ID: run.ID, OrgID: f.OrgID, AgentID: a.ID, SponsorID: f.MemberID, Client: "claude-code",
		Reason: "deploy fix", Status: "running", MaxActions: 100, MaxConcurrent: 4,
		ExpiresAt: run.ExpiresAt, StartedAt: run.StartedAt}, run)
	assert.WithinDuration(t, run.StartedAt.Add(time.Hour), run.ExpiresAt, time.Second)

	started := events(t, pool, "agent.run.start")
	require.Len(t, started, 1)
	assert.Equal(t, event{Actor: &f.MemberID, ActorKind: "user", TargetID: run.ID.String(),
		Data: map[string]any{"agent_id": a.ID.String(), "client": "claude-code", "reason": "deploy fix"}}, started[0])

	exec(t, pool, `UPDATE agent SET limits = '{"max_actions": 5, "max_duration": 600, "max_concurrent": 1}' WHERE id = $1`, a.ID)
	limited, err := svc.StartRun(bg, a, "claude-code", "")
	require.NoError(t, err)
	assert.Equal(t, 5, limited.MaxActions)
	assert.Equal(t, 1, limited.MaxConcurrent)
	assert.WithinDuration(t, limited.StartedAt.Add(10*time.Minute), limited.ExpiresAt, time.Second)

	exec(t, pool, `UPDATE agent SET limits = '{"max_actions": 20000}' WHERE id = $1`, a.ID)
	_, err = svc.StartRun(bg, a, "claude-code", "")
	require.Error(t, err, "stored limits out of range fail closed")
	assert.NotErrorIs(t, err, apperr.ErrInvalidArgument, "an internal error, not the caller's input")

	exec(t, pool, `UPDATE agent SET limits = '{"max_actions": 7}' WHERE id = $1`, a.ID)
	got, err := svc.CheckRun(bg, f.OrgID, limited.ID)
	require.NoError(t, err)
	assert.Equal(t, 5, got.MaxActions, "a limits change never alters a started run")
}

func TestStartRunRefusesFrozenOrLocked(t *testing.T) {
	svc, pool, f := newTestService(t)
	a, err := svc.EnsureAgent(bg, f.OrgID, f.MemberID, "claude")
	require.NoError(t, err)
	runs := func() int {
		var n int
		require.NoError(t, pool.QueryRow(bg, "SELECT count(*) FROM agent_run").Scan(&n))
		return n
	}

	exec(t, pool, "UPDATE agent SET status = 'frozen' WHERE id = $1", a.ID)
	_, err = svc.StartRun(bg, a, "claude-code", "")
	require.ErrorIs(t, err, apperr.ErrPermissionDenied)
	assert.ErrorContains(t, err, "agent is frozen")
	exec(t, pool, "UPDATE agent SET status = 'active' WHERE id = $1", a.ID)

	for target, id := range map[string]*uuid.UUID{"agent": &a.ID, "agents_of_user": &f.MemberID, "user": &f.MemberID, "all_agents": nil} {
		lock := f.Lock(t, pool, target, id)
		_, err = svc.StartRun(bg, a, "claude-code", "")
		require.ErrorIs(t, err, apperr.ErrPermissionDenied, target)
		assert.ErrorContains(t, err, "agent is locked", target)
		exec(t, pool, "DELETE FROM lock WHERE id = $1", lock)
	}
	assert.Zero(t, runs(), "a refused start leaves no run")

	expired := f.Lock(t, pool, "agent", &a.ID)
	exec(t, pool, "UPDATE lock SET expires_at = now() - interval '1 minute' WHERE id = $1", expired)
	otherAgent := dbtest.Agent(t, pool, f.OrgID, f.MemberID, "codex")
	f.Lock(t, pool, "agent", &otherAgent)
	f.Lock(t, pool, "user", &f.OtherMemberID)
	_, err = svc.StartRun(bg, a, "claude-code", "")
	require.NoError(t, err, "an expired lock and locks of other targets do not apply")

	for name, in := range map[string][2]string{
		"empty client":        {"", ""},
		"client of 129":       {strings.Repeat("c", 129), ""},
		"client with newline": {"claude\ncode", ""},
		"reason of 1025":      {"claude-code", strings.Repeat("r", 1025)},
		"reason not UTF-8":    {"claude-code", "\xff"},
	} {
		_, err := svc.StartRun(bg, a, in[0], in[1])
		assert.ErrorIs(t, err, apperr.ErrInvalidArgument, name)
	}
	_, err = svc.StartRun(bg, a, strings.Repeat("c", 128), strings.Repeat("r", 1024))
	require.NoError(t, err, "upper bounds")

	other := dbtest.Seed(t, pool)
	_, err = svc.StartRun(bg, agentic.Agent{ID: a.ID, OrgID: other.OrgID, SponsorID: other.MemberID}, "claude-code", "")
	assert.ErrorIs(t, err, apperr.ErrNotFound, "agent of another organization")
	assert.Equal(t, 2, runs())
}
