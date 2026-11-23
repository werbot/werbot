package gateway

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"testing"
	"testing/synctest"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOpenAgentExec(t *testing.T) {
	e, a := newAgentEnv(t)
	tool := uuid.New()
	r := e.cmdReq("x", KindExec, "ls")
	r.ToolCall = tool
	s, err := e.p.Open(t.Context(), r)
	require.NoError(t, err)
	assert.Equal(t, agentStart{RunID: e.m["run1"], ToolCall: tool}, sessionAgent(s.ID), "StartSession carries the run and the tool call")
	require.NotNil(t, s.User.Agent)

	_, err = e.p.Open(t.Context(), e.req("x", KindTerminal))
	require.ErrorIs(t, err, ErrDenied, "an agent never gets a terminal")
	_, err = e.p.Open(t.Context(), e.cmdReq("x", KindExec, "rm -rf /"))
	require.ErrorIs(t, err, ErrDenied)
	got := a.recorded()
	require.Len(t, got, 2, "each policy denial of an agent is recorded at once")
	assert.Equal(t, e.m["run1"], got[0].run)
	assert.Equal(t, "x", got[0].resource)
	assert.Contains(t, got[0].reasons, "agent:baseline:terminal")
	assert.Equal(t, recordedDenial{run: e.m["run1"], resource: "x", command: "rm -rf /", classes: []string{"destructive"},
		reasons: []string{"agent:baseline:dangerous", "command:a"}}, got[1])
	assert.Empty(t, e.p.heartbeat().GetDenials(), "a recorded denial is not reported twice")

	for text, want := range map[string]error{"agent limit exceeded": ErrAgentLimit, "agent run is not active": ErrAgentRunNotActive} {
		e.f.with(func(f *fakeControl) { f.startErr = connect.NewError(connect.CodeFailedPrecondition, errors.New(text)) })
		_, err = e.p.Open(t.Context(), e.cmdReq("x", KindExec, "ls"))
		require.ErrorIs(t, err, want)
		assert.NotErrorIs(t, err, ErrDenied, "the run refused it, not the policy")
	}
	assert.Len(t, a.recorded(), 2, "a refusal of the run is no policy denial")
	e.f.with(func(f *fakeControl) {
		f.startErr = connect.NewError(connect.CodeFailedPrecondition, errors.New("other"))
	})
	_, err = e.p.Open(t.Context(), e.cmdReq("x", KindExec, "ls"))
	require.ErrorIs(t, err, ErrDenied, "any other failed precondition")
}

func TestOpenHumanIgnoresToolCall(t *testing.T) {
	e, a := newAgentEnv(t)
	e.f.with(func(f *fakeControl) { f.user.Agent = nil })
	r := e.cmdReq("x", KindExec, "ls")
	r.ToolCall = uuid.New()
	s, err := e.p.Open(t.Context(), r)
	require.NoError(t, err)
	assert.Equal(t, agentStart{}, sessionAgent(s.ID), "a person's session has no run and no tool call")
	_, err = e.p.Open(t.Context(), e.cmdReq("x", KindExec, "rm -rf /"))
	require.ErrorIs(t, err, ErrDenied)
	assert.Empty(t, a.recorded(), "a person's denial is never recorded as an agent's")
	ds := e.p.heartbeat().GetDenials()
	require.Len(t, ds, 1)
	assert.Empty(t, ds[0].GetAgentRunId())
}

func TestRecordDenialFallback(t *testing.T) {
	e, a := newAgentEnv(t)
	a.denyErr = connect.NewError(connect.CodeUnavailable, errors.New("down"))
	_, err := e.p.Open(t.Context(), e.cmdReq("x", KindExec, "rm -rf /"))
	require.ErrorIs(t, err, ErrDenied)
	ds := e.p.heartbeat().GetDenials()
	require.Len(t, ds, 1, "an unrecorded denial goes to the next heartbeat")
	d := ds[0]
	assert.Equal(t, e.m["run1"].String(), d.GetAgentRunId())
	assert.Equal(t, "denied", d.GetReason())
	assert.Equal(t, "rm -rf /", d.GetTarget())
	assert.Equal(t, []string{"destructive"}, d.GetClasses())
	assert.Contains(t, d.GetReasons(), "agent:baseline:dangerous")
}

// agentApprovalPolicies ask an approval of write commands, for agents too.
const agentApprovalPolicies = `
@id("binding:b")
permit (principal == Werbot::User::"u1", action, resource);
@id("command:w")
@obligation("approval")
forbid (principal, action == Werbot::Action::"execy.exec", resource) when { context.classes.contains("write") }
unless { context.approval_granted };
@id("command:w:agent")
permit (principal is Werbot::Agent, action in [Werbot::Action::"execy.exec"], resource) when { ["write"].containsAll(context.classes) };`

func TestAgentApprovalAlreadyApproved(t *testing.T) {
	e, _ := newAgentEnv(t)
	e.f.setPolicy(agentPolicy(t, e.m, agentApprovalPolicies, nil, testAgent{"a1", "u1", "active"}))
	appr := &fakeApprovals{ttl: 5 * time.Minute, statuses: []string{"pending"}, status: "approved"}
	e.f.with(func(f *fakeControl) { f.approvals = appr })
	var notice bytes.Buffer
	_, took, err := e.openWrite(t.Context(), &notice)
	require.NoError(t, err)
	assert.Less(t, took, 100*time.Millisecond)
	assert.Empty(t, appr.gets, "an approval granted before is not waited for")
	assert.Empty(t, notice.String())
	require.Len(t, appr.requests, 1)
	assert.Equal(t, e.m["run1"], appr.requests[0].AgentRunID)
}

// runOuty opens an exec of command on "x" and runs it on outy; it returns
// what the client read on stdout.
func (e *pipeEnv) runOuty(t *testing.T, command string) (*Session, string) {
	t.Helper()
	s, err := e.p.Open(t.Context(), e.cmdReq("x", KindExec, command))
	require.NoError(t, err)
	var out bytes.Buffer
	s.Exec = &Exec{Command: s.Command, Stdin: bytes.NewReader(nil), Stdout: &out, Stderr: &bytes.Buffer{}, SetExitStatus: func(int) {}}
	require.NoError(t, e.p.Run(t.Context(), s, funcConn{typ: "execy", kinds: []Kind{KindExec}, serve: outy}))
	return s, out.String()
}

func TestAgentMaskedByDefault(t *testing.T) {
	e, _ := newAgentEnv(t)
	_, out := e.runOuty(t, "ls")
	assert.Equal(t, "ls secret=[masked]\n", out, "an agent's output is masked under the default mode")
	e.f.with(func(f *fakeControl) { f.user.Agent = nil })
	_, out = e.runOuty(t, "ls")
	assert.Equal(t, "ls secret=s3cret-value\n", out, "a person's is not")
}

func TestExecResultInEndSession(t *testing.T) {
	e, _ := newAgentEnv(t)
	s, out := e.runOuty(t, "ls")
	sum := sha256.Sum256([]byte("ls secret=s3cret-value\n")) // the recording keeps the original
	require.Eventually(t, func() bool { return len(e.f.endsOf(s.ID)) == 1 }, 2*time.Second, 5*time.Millisecond)
	r := endResult(s.ID)
	require.NotNil(t, r, out)
	require.NotNil(t, r.ExitCode)
	assert.Equal(t, int32(3), *r.ExitCode)
	assert.Equal(t, sum[:], r.OutputSHA256)

	t.Run("terminal", func(t *testing.T) {
		ts, err := e.p.Open(t.Context(), e.req("x", KindTerminal))
		require.ErrorIs(t, err, ErrDenied) // agents get none; a person's terminal has no exec result
		require.Nil(t, ts)
		e.f.with(func(f *fakeControl) { f.user.Agent = nil })
		ts = e.open(t, "x", KindTerminal)
		e.p.abort(ts)
		require.Eventually(t, func() bool { return len(e.f.endsOf(ts.ID)) == 1 }, 2*time.Second, 5*time.Millisecond)
		assert.Nil(t, endResult(ts.ID))
	})
}

func TestRevokeAgentRunOnSnapshot(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		e, _ := newAgentEnv(t)
		startSweeper(t, e.p)
		first, err := e.p.Open(t.Context(), e.cmdReq("x", KindExec, "sleep 30"))
		require.NoError(t, err)
		e.m["run2"] = uuid.New()
		e.f.with(func(f *fakeControl) { f.user.Agent = &Agent{ID: e.m["a1"], RunID: e.m["run2"], AuthAt: time.Now()} })
		second, err := e.p.Open(t.Context(), e.cmdReq("x", KindExec, "sleep 30"))
		require.NoError(t, err)
		e.runBlocking(t, first)
		e.runBlocking(t, second)

		pol := agentPolicy(t, e.m, execTestPolicies, []string{e.m["run1"].String()}, testAgent{"a1", "u1", "active"})
		pol.version = "v2"
		e.f.setPolicy(pol)
		time.Sleep(time.Second)
		assert.Equal(t, []string{"access_revoked"}, e.endedWith(first.ID), "the blocked run ends within a second")
		assert.Empty(t, e.endedWith(second.ID), "another run of the agent keeps its session")
	})
}

// runBlocking runs s on a connector that blocks until the session ends.
func (e *pipeEnv) runBlocking(t *testing.T, s *Session) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = e.p.Run(ctx, s, funcConn{typ: "execy", kinds: []Kind{KindExec}, serve: block})
	}()
	t.Cleanup(func() { cancel(); <-done })
}
