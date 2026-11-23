package app

import (
	"context"
	"crypto/sha256"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
	"github.com/werbot/werbot/internal/mcpserver"
)

// TestE2EAgentExecRead: a read command of an agent runs through the real
// gateway; its session carries the run, the tool call, the exit code and
// the SHA-256 of the recorded output; the timeline lists the call and the
// session; closing the client ends the run as client_exit.
func TestE2EAgentExecRead(t *testing.T) {
	s, _, p := agentStack(t)
	m := s.startMCP(t, mcpserver.Options{Profile: p, Agent: "e2e"})

	out := m.exec(t, "uname")
	assert.Equal(t, "ok", out.Status)
	assert.Equal(t, "Linux\n", out.Stdout)
	require.NotNil(t, out.ExitCode)
	assert.Zero(t, *out.ExitCode)

	ctx, run := context.Background(), s.agentRun(t)
	var call uuid.UUID
	require.NoError(t, s.pool.QueryRow(ctx, "SELECT id FROM agent_tool_call WHERE run_id = $1 AND tool = 'ssh_exec' AND status = 'ok'",
		run).Scan(&call))
	want := sha256.Sum256([]byte("Linux\n"))
	require.EventuallyWithT(t, func(c *assert.CollectT) { // the gateway ends the session after the client saw the result
		var toolCall uuid.UUID
		var code *int
		var sum []byte
		require.NoError(c, s.pool.QueryRow(ctx, `SELECT tool_call_id, exit_code, output_sha256 FROM session
			WHERE agent_run_id = $1 AND command = 'uname'`, run).Scan(&toolCall, &code, &sum))
		assert.Equal(c, call, toolCall)
		if assert.NotNil(c, code) {
			assert.Zero(c, *code)
		}
		assert.Equal(c, want[:], sum)
	}, 10*time.Second, 100*time.Millisecond)

	tl := s.runTimeline(t, run)
	assert.Equal(t, "running", tl.GetRun().GetStatus())
	assert.Equal(t, int32(1), tl.GetRun().GetActions())
	require.Len(t, tl.GetToolCalls(), 1)
	assert.Equal(t, "ssh_exec", tl.GetToolCalls()[0].GetTool())
	require.Len(t, tl.GetSessions(), 1)
	assert.Equal(t, call.String(), tl.GetSessions()[0].GetToolCallId())
	assert.Equal(t, "srv", tl.GetSessions()[0].GetResourceName())

	m.close(t)
	var status, reason string
	require.NoError(t, s.pool.QueryRow(ctx, "SELECT status, end_reason FROM agent_run WHERE id = $1", run).Scan(&status, &reason))
	assert.Equal(t, "ended", status)
	assert.Equal(t, "client_exit", reason)
}

// TestE2EAgentExecApproval: a write command of an agent under an approval
// rule for agents waits for a person; once approved it runs, and the same
// command of the run reuses the approval; the timeline lists it.
func TestE2EAgentExecApproval(t *testing.T) {
	s, srv, p := agentStack(t)
	carol := s.addAdmin(t, "carol")
	s.classRule(t, "hostname", "write")
	s.agentCommandRule(t, srv, "approval", "write")
	requestPolicy(t, signIn(t, s.url, "alice@example.com"), "srv-commands", false, srv)
	s.denyDestructive(t, srv) // the rules above have reached the gateway snapshot
	m := s.startMCP(t, mcpserver.Options{Profile: p, Agent: "e2e"})

	done := make(chan mcpserver.ExecOutput, 1)
	go func() { done <- m.exec(t, "hostname") }()
	id := s.pendingCommand(t)
	_, err := carol.req.Approve(context.Background(), connect.NewRequest(&werbotv1.ApproveRequest{Id: id}))
	require.NoError(t, err)
	var out mcpserver.ExecOutput
	select {
	case out = <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("the approved command did not run within 5s of the decision")
	}
	assert.Equal(t, "ok", out.Status)
	assert.Equal(t, "target\n", out.Stdout)

	again := m.exec(t, "hostname")
	assert.Equal(t, "ok", again.Status, "the run reuses its approval")
	assert.Equal(t, "target\n", again.Stdout)

	tl := s.runTimeline(t, s.agentRun(t))
	require.Len(t, tl.GetApprovals(), 1)
	assert.Equal(t, id, tl.GetApprovals()[0].GetId())
	assert.Equal(t, "approved", tl.GetApprovals()[0].GetStatus())
	assert.Equal(t, "carol", tl.GetApprovals()[0].GetDecidedByLogin())
	assert.Len(t, tl.GetSessions(), 2)
}

// TestE2EAgentLimitExceeded: past max_actions the gateway refuses the
// command before any dial; a limit refusal is not a policy denial.
func TestE2EAgentLimitExceeded(t *testing.T) {
	s, _, p := agentStack(t)
	s.createAgent(t, "e2e", &werbotv1.AgentLimits{MaxActions: 2})
	m := s.startMCP(t, mcpserver.Options{Profile: p, Agent: "e2e"})

	for range 2 {
		assert.Equal(t, "ok", m.exec(t, "uname").Status)
	}
	out := m.exec(t, "uname")
	assert.Equal(t, "error", out.Status)
	assert.Equal(t, "Agent limit exceeded", out.Message)

	var actions, denials int
	require.NoError(t, s.pool.QueryRow(context.Background(), "SELECT actions, denials_in_row FROM agent_run WHERE id = $1",
		s.agentRun(t)).Scan(&actions, &denials))
	assert.Equal(t, 2, actions)
	assert.Zero(t, denials, "a limit refusal is not a denial")
}
