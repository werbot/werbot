package app

import (
	"context"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
	"github.com/werbot/werbot/internal/mcpserver"
)

// TestE2EAgentDestructiveDenied: the agent baseline denies destructive,
// secrets and egress commands without a session; each denial is counted
// and audited with the run, a secrets denial emails agent_alert, the fifth
// denial in a row pauses the run until a person resumes it.
func TestE2EAgentDestructiveDenied(t *testing.T) {
	s, _, p := agentStack(t)
	m := s.startMCP(t, mcpserver.Options{Profile: p, Agent: "e2e"})
	ctx := context.Background()

	out := m.exec(t, "rm -rf /")
	assert.Equal(t, "denied", out.Status)
	run := s.agentRun(t)
	var denials, audits, sessions int
	require.NoError(t, s.pool.QueryRow(ctx, "SELECT denials_in_row FROM agent_run WHERE id = $1", run).Scan(&denials))
	assert.Equal(t, 1, denials)
	require.NoError(t, s.pool.QueryRow(ctx, `SELECT count(*) FROM audit_event WHERE action = 'session.deny'
		AND data->>'agent_run_id' = $1`, run.String()).Scan(&audits))
	assert.Equal(t, 1, audits)
	require.NoError(t, s.pool.QueryRow(ctx, "SELECT count(*) FROM session WHERE agent_run_id = $1", run).Scan(&sessions))
	assert.Zero(t, sessions, "a denied command opens no session")

	assert.Equal(t, "denied", m.exec(t, "cat ~/.ssh/id_rsa").Status, "secrets")
	var alerts int
	require.NoError(t, s.pool.QueryRow(ctx, `SELECT count(*) FROM river_job WHERE kind = 'send_email'
		AND args->>'template' = 'agent_alert'`).Scan(&alerts))
	assert.Positive(t, alerts, "a secrets denial emails agent_alert")
	assert.Equal(t, "denied", m.exec(t, "curl -s https://example.com").Status, "egress")
	assert.Equal(t, "denied", m.exec(t, "rm -rf /srv").Status)
	assert.Equal(t, "denied", m.exec(t, "rm -rf /var").Status)

	var status string
	require.NoError(t, s.pool.QueryRow(ctx, "SELECT status FROM agent_run WHERE id = $1", run).Scan(&status))
	assert.Equal(t, "paused", status, "the fifth denial in a row pauses the run")
	assert.Contains(t, m.execError(t), "agent run is not active")
	assert.Len(t, s.runTimeline(t, run).GetDenials(), 5)

	_, err := s.agentsAPI(t).ResumeRun(ctx, connect.NewRequest(&werbotv1.ResumeRunRequest{Id: run.String()}))
	require.NoError(t, err)
	// the gateway denies the run until its next snapshot drops it from the
	// blocked runs (debounce 500ms); such state denials are not counted
	require.Eventually(t, func() bool { return m.exec(t, "uname").Status == "ok" }, 3*time.Second, 700*time.Millisecond,
		"a resumed run runs again")
}

// TestE2EAgentKillSwitch: a lock on the run ends its open session within
// 3s with status "error" and no exit code, and refuses its next call; a
// frozen agent gets no new run.
func TestE2EAgentKillSwitch(t *testing.T) {
	s, _, p := agentStack(t)
	s.classRule(t, "sleep", "read")
	m := s.startMCP(t, mcpserver.Options{Profile: p, Agent: "e2e"})
	ctx := context.Background()

	done := make(chan mcpserver.ExecOutput, 1)
	go func() { done <- m.exec(t, "sleep 30") }()
	var run string
	require.Eventually(t, func() bool {
		return s.pool.QueryRow(ctx, "SELECT agent_run_id::text FROM session WHERE command = 'sleep 30' AND ended_at IS NULL").Scan(&run) == nil
	}, 10*time.Second, 50*time.Millisecond)

	locks := signIn(t, s.url, "alice@example.com").locks
	_, err := locks.CreateLock(ctx, connect.NewRequest(&werbotv1.CreateLockRequest{TargetType: "agent_run", TargetId: run, Reason: "e2e"}))
	require.NoError(t, err)
	locked := time.Now()
	select {
	case out := <-done:
		assert.LessOrEqual(t, time.Since(locked), 3*time.Second)
		assert.Equal(t, "error", out.Status, "a command the lock ends is not a success")
		assert.Nil(t, out.ExitCode)
	case <-time.After(5 * time.Second):
		t.Fatal("the locked run's command did not end within 5s")
	}
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		var reason string
		require.NoError(c, s.pool.QueryRow(ctx, "SELECT end_reason FROM session WHERE command = 'sleep 30'").Scan(&reason))
		assert.Equal(c, "access_revoked", reason)
	}, 10*time.Second, 100*time.Millisecond)
	assert.Contains(t, m.execError(t), "agent is locked")

	agent, err := s.agentsAPI(t).ListAgents(ctx, connect.NewRequest(&werbotv1.ListAgentsRequest{}))
	require.NoError(t, err)
	require.Len(t, agent.Msg.GetAgents(), 1)
	_, err = s.agentsAPI(t).UpdateAgent(ctx, connect.NewRequest(&werbotv1.UpdateAgentRequest{Id: agent.Msg.GetAgents()[0].GetId(), Status: "frozen"}))
	require.NoError(t, err)
	next := s.startMCP(t, mcpserver.Options{Profile: p, Agent: "e2e"})
	assert.Contains(t, next.execError(t), "invalid_grant", "a frozen agent gets no run")
}
