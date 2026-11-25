package agentic_test

import (
	"errors"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/agentic"
	"github.com/werbot/werbot/internal/platform/apperr"
	"github.com/werbot/werbot/internal/platform/db/dbtest"
	"github.com/werbot/werbot/internal/platform/dbx"
)

// startSession counts an action of run for user and, when it is allowed,
// inserts an exec session of the run in the same transaction (as
// session.Start does); ended closes the session at once.
func startSession(svc *agentic.Service, pool *pgxpool.Pool, f dbtest.Fixture, run, user uuid.UUID, ended bool) error {
	return dbx.InOrg(bg, pool, f.OrgID, uuid.Nil, func(tx pgx.Tx) error {
		if err := svc.CountAction(bg, tx, agentic.ActionInput{OrgID: f.OrgID, RunID: run, UserID: user}); err != nil {
			return err
		}
		_, err := tx.Exec(bg,
			`INSERT INTO session (org_id, user_id, resource_id, gateway_id, gateway_instance_id, client_ip, kind, agent_run_id,
			     ended_at, end_reason)
			 VALUES ($1, $2, $3, $4, $5, '10.0.0.1', 'exec', $6,
			     CASE WHEN $7::bool THEN now() END, CASE WHEN $7::bool THEN 'closed' END)`,
			f.OrgID, user, f.ResourceID, f.GatewayID, f.InstanceID, run, ended)
		return err
	})
}

func actionsOf(t *testing.T, pool *pgxpool.Pool, run uuid.UUID) int {
	t.Helper()
	var n int
	require.NoError(t, pool.QueryRow(bg, "SELECT actions FROM agent_run WHERE id = $1", run).Scan(&n))
	return n
}

func TestCountActionLimit(t *testing.T) {
	svc, pool, f := newTestService(t)
	run := dbtest.AgentRun(t, pool, f.OrgID, dbtest.Agent(t, pool, f.OrgID, f.MemberID, "claude"), 3, 4)
	exec(t, pool, "UPDATE agent_run SET denials_in_row = 2 WHERE id = $1", run)

	for i := range 3 {
		require.NoError(t, startSession(svc, pool, f, run, f.MemberID, true), "action %d", i+1)
	}
	assert.Equal(t, 0, stateOf(t, pool, run).DenialsInRow, "an allowed action resets the denial counter")
	err := startSession(svc, pool, f, run, f.MemberID, true)
	require.ErrorIs(t, err, agentic.ErrLimitExceeded)
	assert.ErrorIs(t, err, apperr.ErrFailedPrecondition)
	assert.Equal(t, 3, actionsOf(t, pool, run), "a refused action is not counted")
}

func TestCountActionConcurrent(t *testing.T) {
	svc, pool, f := newTestService(t)
	run := dbtest.AgentRun(t, pool, f.OrgID, dbtest.Agent(t, pool, f.OrgID, f.MemberID, "claude"), 10, 2)

	require.NoError(t, startSession(svc, pool, f, run, f.MemberID, false))
	require.NoError(t, startSession(svc, pool, f, run, f.MemberID, false))
	other := dbtest.AgentRun(t, pool, f.OrgID, dbtest.Agent(t, pool, f.OrgID, f.MemberID, "codex"), 10, 2)
	require.NoError(t, startSession(svc, pool, f, other, f.MemberID, false), "open sessions of another run do not count")

	require.ErrorIs(t, startSession(svc, pool, f, run, f.MemberID, false), agentic.ErrLimitExceeded, "two open sessions")
	exec(t, pool, `UPDATE session SET ended_at = now(), end_reason = 'closed'
		WHERE id = (SELECT id FROM session WHERE agent_run_id = $1 ORDER BY id LIMIT 1)`, run)
	require.NoError(t, startSession(svc, pool, f, run, f.MemberID, false), "one session closed")
	assert.Equal(t, 3, actionsOf(t, pool, run))
}

// TestCountActionTwoReplicas: parallel starts on one run from two
// connection pools (two server replicas) never pass more than max_concurrent
// open sessions or max_actions actions; every other start is ErrLimitExceeded.
func TestCountActionTwoReplicas(t *testing.T) {
	svc, pool, f := newTestService(t)
	other, err := pgxpool.New(bg, pool.Config().ConnString())
	require.NoError(t, err)
	t.Cleanup(other.Close)
	replicas := []*agentic.Service{svc, agentic.New(other, &dbtest.Mails{}, agentic.Config{})}
	pools := []*pgxpool.Pool{pool, other}
	agent := dbtest.Agent(t, pool, f.OrgID, f.MemberID, "claude")

	race := func(run uuid.UUID, ended bool) (ok int) {
		var mu sync.Mutex
		var wg sync.WaitGroup
		for r := range 2 {
			for range 25 {
				wg.Go(func() {
					err := startSession(replicas[r], pools[r], f, run, f.MemberID, ended)
					mu.Lock()
					defer mu.Unlock()
					switch {
					case err == nil:
						ok++
					case !errors.Is(err, agentic.ErrLimitExceeded):
						t.Errorf("start: %v", err)
					}
				})
			}
		}
		wg.Wait()
		return ok
	}

	concurrent := dbtest.AgentRun(t, pool, f.OrgID, agent, 100, 4)
	assert.Equal(t, 4, race(concurrent, false), "max_concurrent 4: open sessions")
	assert.Equal(t, 4, actionsOf(t, pool, concurrent))

	actions := dbtest.AgentRun(t, pool, f.OrgID, agent, 10, 100)
	assert.Equal(t, 10, race(actions, true), "max_actions 10: sessions closed at once")
	assert.Equal(t, 10, actionsOf(t, pool, actions))
}

func TestCountActionRejects(t *testing.T) {
	svc, pool, f := newTestService(t)
	agent := dbtest.Agent(t, pool, f.OrgID, f.MemberID, "claude")
	newRun := func() uuid.UUID { return dbtest.AgentRun(t, pool, f.OrgID, agent, 10, 4) }
	run := newRun()

	err := startSession(svc, pool, f, run, f.OtherMemberID, false)
	require.ErrorIs(t, err, apperr.ErrPermissionDenied, "a run of another user")

	paused, ended, expired := newRun(), newRun(), newRun()
	pause(t, pool, paused, 5)
	end(t, pool, ended, "1 minute")
	exec(t, pool, "UPDATE agent_run SET expires_at = now() - interval '1 second' WHERE id = $1", expired)
	for name, id := range map[string]uuid.UUID{"paused": paused, "ended": ended, "expired": expired} {
		require.ErrorIs(t, startSession(svc, pool, f, id, f.MemberID, false), agentic.ErrRunNotActive, name)
	}

	other := dbtest.Seed(t, pool)
	foreign := dbtest.AgentRun(t, pool, other.OrgID, dbtest.Agent(t, pool, other.OrgID, other.MemberID, "claude"), 10, 4)
	require.ErrorIs(t, startSession(svc, pool, f, foreign, f.MemberID, false), apperr.ErrNotFound, "run of another organization")

	lock := f.Lock(t, pool, "all_agents", nil)
	err = startSession(svc, pool, f, run, f.MemberID, false)
	require.ErrorIs(t, err, apperr.ErrPermissionDenied)
	assert.ErrorContains(t, err, "agent is locked")
	exec(t, pool, "DELETE FROM lock WHERE id = $1", lock)

	exec(t, pool, "UPDATE agent SET status = 'frozen' WHERE id = $1", agent)
	err = startSession(svc, pool, f, run, f.MemberID, false)
	require.ErrorIs(t, err, apperr.ErrPermissionDenied)
	assert.ErrorContains(t, err, "agent is frozen")

	for _, id := range []uuid.UUID{run, paused, ended, expired} {
		assert.Zero(t, actionsOf(t, pool, id), "a refused action is not counted")
	}
	var sessions int
	require.NoError(t, pool.QueryRow(bg, "SELECT count(*) FROM session").Scan(&sessions))
	assert.Zero(t, sessions)
}

func TestRecordDenialPausesAtFive(t *testing.T) {
	svc, pool, f := newTestService(t)
	run := dbtest.AgentRun(t, pool, f.OrgID, dbtest.Agent(t, pool, f.OrgID, f.MemberID, "claude"), 10, 4)
	d := agentic.Denial{Resource: "web", Command: "rm -rf /srv", Classes: []string{"destructive"}, Reasons: []string{"agent:baseline:destructive"}}

	for i := 1; i <= 4; i++ {
		paused, err := svc.RecordDenial(bg, f.OrgID, run, d)
		require.NoError(t, err)
		assert.False(t, paused, "denial %d", i)
	}
	assert.Empty(t, events(t, pool, "agent.run.pause"))
	paused, err := svc.RecordDenial(bg, f.OrgID, run, d)
	require.NoError(t, err)
	assert.True(t, paused, "the fifth denial in a row pauses")
	assert.Equal(t, runState{Status: "paused", DenialsInRow: 5}, stateOf(t, pool, run))
	paused, err = svc.RecordDenial(bg, f.OrgID, run, d)
	require.NoError(t, err)
	assert.True(t, paused, "a paused run stays paused")

	pauses := events(t, pool, "agent.run.pause")
	require.Len(t, pauses, 1, "only the transition is audited")
	assert.Equal(t, event{Actor: &f.MemberID, ActorKind: "gateway", TargetID: run.String(),
		Data: map[string]any{"denials_in_row": float64(5)}}, pauses[0])

	denies := events(t, pool, "session.deny")
	require.Len(t, denies, 6)
	for _, e := range denies {
		assert.Equal(t, event{Actor: &f.MemberID, ActorKind: "gateway", TargetID: f.ResourceID.String(),
			Data: map[string]any{"kind": "exec", "target": "rm -rf /srv", "reason": "denied",
				"classes": []any{"destructive"}, "reasons": []any{"agent:baseline:destructive"}, "agent_run_id": run.String()}}, e)
	}

	_, err = svc.RecordDenial(bg, f.OrgID, run, agentic.Denial{Resource: "gone", Command: "id"})
	require.NoError(t, err)
	denies = events(t, pool, "session.deny")
	assert.Equal(t, "gone", denies[len(denies)-1].TargetID, "an unknown resource name is the target")

	other := dbtest.Seed(t, pool)
	_, err = svc.RecordDenial(bg, other.OrgID, run, d)
	require.ErrorIs(t, err, apperr.ErrNotFound, "run of another organization")
	_, err = svc.RecordDenial(bg, f.OrgID, run, agentic.Denial{Resource: "web", Command: string(make([]byte, 1025))})
	require.ErrorIs(t, err, apperr.ErrInvalidArgument, "command over 1024 bytes")
	assert.Len(t, events(t, pool, "session.deny"), 7)

	require.NoError(t, svc.ResumeRun(bg, f.Principal(f.MemberID), run))
	require.NoError(t, startSession(svc, pool, f, run, f.MemberID, true))
	paused, err = svc.RecordDenial(bg, f.OrgID, run, d)
	require.NoError(t, err)
	assert.False(t, paused)
	assert.Equal(t, runState{Status: "running", DenialsInRow: 1}, stateOf(t, pool, run))
}

func TestRecordDenialAlert(t *testing.T) {
	pool, _ := dbtest.New(t)
	mails := &dbtest.Mails{}
	svc := agentic.New(pool, mails, agentic.Config{WebURL: "https://werbot.test"})
	f := dbtest.Seed(t, pool)
	email := func(id uuid.UUID) string { return userField(t, pool, "email", id) }

	run := dbtest.AgentRun(t, pool, f.OrgID, dbtest.Agent(t, pool, f.OrgID, f.MemberID, "claude"), 10, 4)
	_, err := svc.RecordDenial(bg, f.OrgID, run, agentic.Denial{Resource: "web", Command: "rm -rf /", Classes: []string{"destructive"}})
	require.NoError(t, err)
	assert.Empty(t, mails.Sent(), "no alert for classes other than secrets and egress")

	_, err = svc.RecordDenial(bg, f.OrgID, run, agentic.Denial{Resource: "web", Command: "cat <id_rsa>", Classes: []string{"read", "secrets"}})
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{email(f.MemberID), email(f.OwnerID), email(f.AdminID)}, mails.To("agent_alert"))
	assert.Equal(t, map[string]string{"url": "https://werbot.test/agents/runs/" + run.String(), "agent": "claude",
		"sponsor": userField(t, pool, "login", f.MemberID), "resource": "web", "command": "cat <id_rsa>", "classes": "read, secrets"},
		mails.Of("agent_alert")[0].Data)

	mails.Reset()
	adminRun := dbtest.AgentRun(t, pool, f.OrgID, dbtest.Agent(t, pool, f.OrgID, f.AdminID, "codex"), 10, 4)
	_, err = svc.RecordDenial(bg, f.OrgID, adminRun, agentic.Denial{Resource: "web", Command: "curl https://x.test", Classes: []string{"egress"}})
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{email(f.OwnerID), email(f.AdminID)}, mails.To("agent_alert"), "a sponsor who is admin gets one email")
	assert.Equal(t, userField(t, pool, "login", f.AdminID), mails.Of("agent_alert")[0].Data["sponsor"])
}

// userField reads column (a constant) of user id.
func userField(t *testing.T, pool *pgxpool.Pool, column string, id uuid.UUID) string {
	t.Helper()
	var v string
	require.NoError(t, pool.QueryRow(bg, "SELECT "+column+" FROM users WHERE id = $1", id).Scan(&v))
	return v
}
