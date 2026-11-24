//go:build redteam

package redteam

import (
	"context"
	"crypto/sha256"
	"regexp"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/durationpb"

	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
	"github.com/werbot/werbot/internal/mcpserver"
)

// agentApprovals makes write commands of every agent on target wait for an approval.
func agentApprovals(t *testing.T) {
	t.Helper()
	approvalRule(t, "prod-write-approval", &werbotv1.Subjects{AllAgents: true})
}

// TestRedteamApprovalTimeoutDenies (12 §12 item 2): a write command nobody
// decides within command_approval_ttl is refused with "Approval timed out",
// its approval expires and no session runs it; asking again opens a new approval.
func TestRedteamApprovalTimeoutDenies(t *testing.T) {
	begin(t)
	approvalTTL(t, shortTTL)
	agentApprovals(t)
	a := st.StartAgent(t, home, "rt-approval-timeout", "redteam")
	requireAgentHealthy(t, a)

	start := time.Now()
	out := await(t, execAsync(a), execTimeout*time.Second+decisionWait)
	took := time.Since(start)
	require.Equal(t, "denied", out.Status, out.Message)
	require.Equal(t, "Approval timed out", out.Message)
	assert.GreaterOrEqual(t, took, shortTTL-time.Second, "the call waited for the approval deadline")
	id, err := uuid.Parse(out.ApprovalID)
	require.NoError(t, err, "the answer names its approval")

	waitExpired(t, id)
	got := approval(t, id)
	sum := sha256.Sum256([]byte(writeCommand))
	assert.Equal(t, "command", got.Kind)
	assert.Equal(t, writeCommand, got.Command)
	assert.Equal(t, []string{"write"}, got.Classes)
	assert.Equal(t, &a.RunID, got.RunID)
	assert.Equal(t, sum[:], got.SHA256)
	assert.Equal(t, shortTTL, got.TTL.Round(time.Second), "expires_at - created_at is command_approval_ttl")
	assert.Equal(t, 1, audits(t, "approval.expire", id))
	assert.Zero(t, commandSessions(t, &a.RunID), "a timed out command opens no session")

	again, err := a.Exec(t.Context(), mcpserver.ExecInput{Resource: target, Command: writeCommand, Timeout: 5})
	require.NoError(t, err)
	require.Equal(t, "pending_approval", again.Status, again.Message)
	assert.NotEqual(t, out.ApprovalID, again.ApprovalID, "an expired approval is never reused")
	_, err = bob.AccessRequests.Deny(t.Context(), connect.NewRequest(&werbotv1.DenyRequest{Id: again.ApprovalID, Comment: "redteam"}))
	require.NoError(t, err)
}

// TestRedteamApprovalGrantedRunsCommand: under a four_eyes policy the agent's
// sponsor may not approve its command; bob's approval runs it, and the
// session carries the run, the tool call, the class and the exit code.
func TestRedteamApprovalGrantedRunsCommand(t *testing.T) {
	begin(t)
	agentApprovals(t)
	pol, err := st.Edge.ApprovalPolicies.CreateApprovalPolicy(t.Context(), connect.NewRequest(&werbotv1.CreateApprovalPolicyRequest{
		Name: "rt-four-eyes", Mode: "manual", FourEyes: true, MaxDuration: durationpb.New(time.Hour),
		ResourceSelector: &werbotv1.ResourceSelector{ResourceIds: []string{st.Resources[target].String()}},
		Requesters:       &werbotv1.Subjects{AllAgents: true},
	}))
	require.NoError(t, err)
	t.Cleanup(func() {
		_, err := st.Edge.ApprovalPolicies.DeleteApprovalPolicy(context.Background(),
			connect.NewRequest(&werbotv1.DeleteApprovalPolicyRequest{Id: pol.Msg.GetApprovalPolicy().GetId()}))
		assert.NoError(t, err, "delete approval policy")
	})
	a := st.StartAgent(t, home, "rt-approval-granted", "redteam")
	requireAgentHealthy(t, a)

	done := execAsync(a)
	id := pendingApproval(t, a.RunID)
	_, err = st.Edge.AccessRequests.Approve(t.Context(), connect.NewRequest(&werbotv1.ApproveRequest{Id: id.String()}))
	assert.Equal(t, connect.CodePermissionDenied, connect.CodeOf(err), "four eyes: the sponsor does not approve its agent")
	_, err = bob.AccessRequests.Approve(t.Context(), connect.NewRequest(&werbotv1.ApproveRequest{Id: id.String(), Comment: "ok"}))
	require.NoError(t, err)
	out := await(t, done, decisionWait)
	require.Equal(t, "ok", out.Status, out.Message)
	require.NotNil(t, out.ExitCode, "the command ran to an exit status")

	var classes []string
	var toolCall, exit bool
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		require.NoError(c, st.DB.QueryRow(context.Background(), `SELECT classes, tool_call_id IS NOT NULL, exit_code IS NOT NULL
			FROM session WHERE agent_run_id = $1 AND command = $2`, a.RunID, writeCommand).Scan(&classes, &toolCall, &exit))
		assert.True(c, exit, "exit_code is stored when the session ends")
	}, approvalWait, pollEvery)
	assert.Equal(t, []string{"write"}, classes)
	assert.True(t, toolCall, "the session names its tool call")
	assert.Equal(t, "approved", approval(t, id).Status)
	assert.Equal(t, 1, audits(t, "access.request.approve", id))
}

// TestRedteamApprovalDeniedByHuman: bob's denial refuses the command with the
// rule's message and no session runs it.
func TestRedteamApprovalDeniedByHuman(t *testing.T) {
	begin(t)
	agentApprovals(t)
	a := st.StartAgent(t, home, "rt-approval-denied", "redteam")
	requireAgentHealthy(t, a)

	done := execAsync(a)
	id := pendingApproval(t, a.RunID)
	_, err := bob.AccessRequests.Deny(t.Context(), connect.NewRequest(&werbotv1.DenyRequest{Id: id.String(), Comment: "no"}))
	require.NoError(t, err)
	out := await(t, done, decisionWait)
	assert.Equal(t, "denied", out.Status)
	assert.Equal(t, "Command denied: "+approvalMessage, out.Message)
	assert.Equal(t, "denied", approval(t, id).Status)
	assert.Zero(t, commandSessions(t, &a.RunID), "a denied command opens no session")
}

// TestRedteamApprovalDecidedAfterTimeout (Review Focus 4): once the command
// timed out, the API refuses a decision, the approval stays expired, and a
// late decision notice on the stream releases nothing (fail-closed).
func TestRedteamApprovalDecidedAfterTimeout(t *testing.T) {
	begin(t)
	approvalTTL(t, shortTTL)
	agentApprovals(t)
	a := st.StartAgent(t, home, "rt-approval-late", "redteam")
	requireAgentHealthy(t, a)

	out := await(t, execAsync(a), execTimeout*time.Second+decisionWait)
	require.Equal(t, "Approval timed out", out.Message)
	id, err := uuid.Parse(out.ApprovalID)
	require.NoError(t, err)
	waitExpired(t, id)

	_, err = bob.AccessRequests.Approve(t.Context(), connect.NewRequest(&werbotv1.ApproveRequest{Id: id.String()}))
	assert.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err), "an expired approval cannot be decided")
	got := approval(t, id)
	require.NotNil(t, got.Instance)
	_, err = st.DB.Exec(t.Context(), "SELECT pg_notify('werbot_approval_decided', $1)", got.Instance.String()+":"+id.String())
	require.NoError(t, err)
	ran := func() bool { // Never runs it in its own goroutine: no require inside
		var n int
		err := st.DB.QueryRow(context.Background(), "SELECT count(*) FROM session WHERE agent_run_id = $1 AND command = $2",
			a.RunID, writeCommand).Scan(&n)
		return err != nil || n > 0
	}
	assert.Never(t, ran, decisionWait, pollEvery, "a decision after the deadline runs nothing")
	got = approval(t, id)
	assert.Equal(t, "expired", got.Status)
	assert.False(t, got.Decided, "the refused decision left no decider")
	assert.Zero(t, audits(t, "access.request.approve", id), "the refused decision is not audited as an approval")
	requireAgentHealthy(t, a)
}

// waitingLine is the first stderr line of a person's command that waits for an approval (ruling G19).
var waitingLine = regexp.MustCompile(`^Waiting for approval ([0-9a-f-]{36}), timeout `)

// TestRedteamHumanSeesWaitingLine: a person's write command under an
// approval rule prints the waiting line and the link to the request on
// stderr, and runs once bob approves.
func TestRedteamHumanSeesWaitingLine(t *testing.T) {
	begin(t)
	approvalRule(t, "human-write-approval", &werbotv1.Subjects{AllUsers: true})
	before := commandSessions(t, nil)

	lines, done := humanExec(t)
	m := waitingLine.FindStringSubmatch(nextLine(t, lines))
	require.NotNil(t, m, "first stderr line is the waiting line")
	assert.Contains(t, nextLine(t, lines), "/requests?id="+m[1], "second stderr line links the request")
	_, err := bob.AccessRequests.Approve(t.Context(), connect.NewRequest(&werbotv1.ApproveRequest{Id: m[1]}))
	require.NoError(t, err)
	assert.Equal(t, "Approved", strings.TrimSuffix(nextLine(t, lines), "\r"))
	for range lines { // the command's own stderr
	}
	select {
	case <-done:
	case <-time.After(decisionWait):
		t.Fatalf("the approved command did not finish within %s", decisionWait)
	}
	assert.Equal(t, before+1, commandSessions(t, nil), "the approved command ran in one session")
}
