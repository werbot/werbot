package gateway

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	gatewayv1 "github.com/werbot/werbot/gen/gateway/v1"
	"github.com/werbot/werbot/internal/gateway/recorder"
)

// runConsoleConn runs s on the consoley connector with a terminal writing to
// out; the returned func cancels it and waits for Run.
func (e *pipeEnv) runConsoleConn(t *testing.T, s *Session, out *syncBuffer) (stop func()) {
	t.Helper()
	c, _ := LookupConnector("consoley")
	s.Terminal = &Terminal{ReadWriter: struct {
		*bytes.Reader
		*syncBuffer
	}{bytes.NewReader(nil), out}}
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan struct{})
	go func() { defer close(done); _ = e.p.Run(ctx, s, c) }()
	stop = func() { cancel(); <-done }
	t.Cleanup(stop)
	return stop
}

func TestShellAllowedRaw(t *testing.T) {
	e := pathsPipeEnv(t, ids{}, pathsPolicies)
	s := e.open(t, "c", KindTerminal)
	require.False(t, s.Console)
	require.Nil(t, s.CheckCommand)
	require.Equal(t, "consoley.shell", e.trackedReq(s.ID).Action, "the sweeper re-decides the shell")
	var out syncBuffer
	stop := e.runConsoleConn(t, s, &out)
	stop()
	require.NotContains(t, out.String(), "console", "a raw shell runs Serve")
}

func TestShellConsoleObligation(t *testing.T) {
	e := pathsPipeEnv(t, ids{}, pathsPolicies+consoleOn("rc"))
	s := e.open(t, "c", KindTerminal)
	require.True(t, s.Console)
	require.NotNil(t, s.CheckCommand)
	require.True(t, e.trackedReq(s.ID).Console)
	var out syncBuffer
	e.runConsoleConn(t, s, &out)
	require.Eventually(t, func() bool { return out.String() == "console\n" }, time.Second, 5*time.Millisecond, "ServeConsole runs, Serve does not")
}

func TestShellConsoleWithoutConsoleServer(t *testing.T) {
	e := pathsPipeEnv(t, ids{}, pathsPolicies+consoleOn("rx"))
	_, err := e.p.Open(t.Context(), e.req("x", KindTerminal))
	require.ErrorIs(t, err, ErrShellDenied)
	require.Zero(t, e.startCalls())
	ds := e.p.denials.take()
	require.Len(t, ds, 1)
	assert.Equal(t, "terminal", ds[0].GetKind())
	assert.Equal(t, "denied", ds[0].GetReason())
}

func TestShellUnknownObligationDenied(t *testing.T) {
	e := pathsPipeEnv(t, ids{}, pathsPolicies+`
@id("custom:c:0")
@obligation("approval")
forbid (principal, action == Werbot::Action::"consoley.shell", resource) unless { context.approval_granted };`)
	_, err := e.p.Open(t.Context(), e.req("c", KindTerminal))
	require.ErrorIs(t, err, ErrShellDenied)
	require.Zero(t, e.startCalls())
}

// Ruling O1: a terminal with a command also takes the shell decision and only
// a plain allow passes; an exec without a PTY takes only the exec checks.
func TestTerminalWithCommandUnderConsoleRuleDenied(t *testing.T) {
	deny := `
@id("terminal:d")
forbid (principal, action in [Werbot::Action::"consoley.shell"], resource);`
	for name, policies := range map[string]string{"console": consoleOn("rc"), "deny": deny} {
		e := pathsPipeEnv(t, ids{}, pathsPolicies+policies)
		_, err := e.p.Open(t.Context(), e.cmdReq("c", KindTerminal, "ls"))
		require.ErrorIs(t, err, ErrShellDenied, name)
		require.Zero(t, e.startCalls(), name)
		s, err := e.p.Open(t.Context(), e.cmdReq("c", KindExec, "ls"))
		require.NoError(t, err, name)
		require.Equal(t, "consoley.exec", e.trackedReq(s.ID).Action, name)
	}
	e := pathsPipeEnv(t, ids{}, pathsPolicies)
	s, err := e.p.Open(t.Context(), e.cmdReq("c", KindTerminal, "ls"))
	require.NoError(t, err)
	require.False(t, s.Console)
	require.Equal(t, "consoley.exec", e.trackedReq(s.ID).Action, "a terminal with a command is re-decided as its exec")
}

// A console line is checked on the session's exec action from its cwd and
// home; a refusal goes to the heartbeat with the session id and the decision.
func TestCheckCommandDenialAudit(t *testing.T) {
	m := ids{}
	rule, text := pathRule(m, "p1", "deny", "No /etc", "/etc/**")
	e := pathsPipeEnv(t, m, pathsPolicies+consoleOn("rc")+text, rule)
	s := e.open(t, "c", KindTerminal)

	chk, err := s.CheckCommand(t.Context(), "cat app/a", "/home/u", "/home/u", nil)
	require.NoError(t, err)
	require.Equal(t, CommandCheck{Classes: []string{"read"}, Paths: []string{"/home/u/app/a"}}, chk)
	require.Empty(t, e.p.denials.take())

	_, err = s.CheckCommand(t.Context(), "cat /etc/hosts", "/home/u", "/home/u", nil)
	require.EqualError(t, err, "Command denied: No /etc")
	ds := e.p.denials.take()
	require.Len(t, ds, 1)
	d := ds[0]
	assert.Equal(t, s.ID.String(), d.GetSessionId())
	assert.Equal(t, "terminal", d.GetKind())
	assert.Equal(t, "cat /etc/hosts", d.GetTarget())
	assert.Equal(t, []string{"read"}, d.GetClasses())
	assert.Equal(t, []string{"/etc/hosts"}, d.GetPaths())
	assert.Equal(t, []string{"path:" + m["p1"].String()}, d.GetReasons())
	assert.Equal(t, "denied", d.GetReason())
	assert.Equal(t, e.m["u1"].String(), d.GetUserId())
	requireValid(t, &gatewayv1.Heartbeat{Denials: ds})
}

// A console denial is cut to the heartbeat bounds: target 1024 bytes, 16
// paths of 1024 bytes, 16 reasons of 128 bytes.
func TestDenialBoundsStage11b(t *testing.T) {
	long := strings.Repeat("é", 1000)
	paths, reasons := make([]string, 20), make([]string, 20)
	for i := range paths {
		paths[i], reasons[i] = "/"+long, "path:"+long
	}
	req := AccessRequest{User: User{ID: uuid.New()}, Command: long,
		Classes: []string{"destructive", "egress", "opaque", "privilege", "read", "secrets", "write"}, Paths: paths}
	d := newCommandDenial("denied", uuid.New(), consoleOpen(), req, reasons)
	require.LessOrEqual(t, len(d.GetTarget()), 1024)
	require.Len(t, d.GetPaths(), 16)
	require.Len(t, d.GetReasons(), 16)
	for i := range 16 {
		require.LessOrEqual(t, len(d.GetPaths()[i]), 1024)
		require.LessOrEqual(t, len(d.GetReasons()[i]), 128)
	}
	requireValid(t, &gatewayv1.Heartbeat{Denials: []*gatewayv1.Denial{d}})
}

// consoleOpen is an OpenRequest of a terminal on "c".
func consoleOpen() OpenRequest {
	return OpenRequest{Front: "ssh", Kind: KindTerminal, Identity: Identity{Method: "ssh-key", Value: "SHA256:key"}, Resource: "c"}
}

// Ruling G23: a console line waiting for an approval does not end the session
// by the idle limit; the idle time counts from the end of the check.
func TestConsoleApprovalWaitNotIdle(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		e := pathsPipeEnv(t, ids{}, pathsPolicies+consoleOn("rc")+`
@id("command:w")
@obligation("approval")
forbid (principal, action == Werbot::Action::"consoley.exec", resource) when { context.classes.contains("write") }
unless { context.approval_granted };`)
		e.p.cfg.IdleTimeout = time.Minute
		a := &fakeApprovals{ttl: time.Hour, statuses: []string{"pending"}}
		e.f.with(func(f *fakeControl) { f.approvals = a })
		startSweeper(t, e.p)
		s := e.open(t, "c", KindTerminal)
		var out syncBuffer
		e.runConsoleConn(t, s, &out)
		time.AfterFunc(10*time.Minute, func() { a.decide("approved") })
		_, err := s.CheckCommand(t.Context(), "mkdir x", "/home/u", "/home/u", nil)
		require.NoError(t, err)
		require.Empty(t, e.endedWith(s.ID), "the wait outlasted the idle limit")
		require.Equal(t, "cd -- '/home/u' || exit 1\nmkdir x", a.requests[0].Command, "approvers see where the line runs")
		time.Sleep(30 * time.Second)
		require.Empty(t, e.endedWith(s.ID), "the end of the check is activity")
		time.Sleep(time.Minute)
		require.Equal(t, []string{"idle_timeout"}, e.endedWith(s.ID))
	})
}

func TestSweeperRevokesExecOnNewPathRule(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		e := pathsPipeEnv(t, ids{}, pathsPolicies)
		startSweeper(t, e.p)
		r := e.cmdReq("c", KindExec, "tail -f ~/app/log")
		tail, err := e.p.Open(t.Context(), r)
		require.NoError(t, err)
		var out syncBuffer
		e.runConsoleConn(t, tail, &out)

		approval, approvalText := pathRule(e.m, "p1", "approval", "", "~/app/**")
		e.f.setPolicy(pathsPolicy(t, e.m, "v2", pathsPolicies+approvalText, approval))
		require.Empty(t, e.endedWith(tail.ID), "an approval rule leaves the running command")

		deny, denyText := pathRule(e.m, "p2", "deny", "", "~/app/**")
		e.f.setPolicy(pathsPolicy(t, e.m, "v3", pathsPolicies+denyText, deny))
		require.Equal(t, []string{"access_revoked"}, e.endedWith(tail.ID))
	})
}

// Ruling G22: a raw shell ends when its shell decision is no longer a plain
// allow; a console session under the same rule stays.
func TestSweeperRevokesRawShellOnConsoleRule(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		e := pathsPipeEnv(t, ids{}, pathsPolicies+consoleOn("rc"))
		startSweeper(t, e.p)
		raw, console := e.open(t, "ask", KindTerminal), e.open(t, "c", KindTerminal)
		require.False(t, raw.Console)
		require.True(t, console.Console)
		e.runConsoleConn(t, raw, &syncBuffer{})
		e.runConsoleConn(t, console, &syncBuffer{})
		e.f.setPolicy(pathsPolicy(t, e.m, "v2", pathsPolicies+consoleOn("rc", "rask")))
		require.Equal(t, []string{"access_revoked"}, e.endedWith(raw.ID))
		require.Empty(t, e.endedWith(console.ID), "the console session stays")
	})
}

// The end of a console session carries its result; any other session none.
func TestFinishSpoolsConsoleResult(t *testing.T) {
	e := pathsPipeEnv(t, ids{}, pathsPolicies+consoleOn("rc"))
	s := e.open(t, "c", KindTerminal)
	s.Command, s.Classes, s.Paths = "ls", []string{"read"}, []string{"/srv"} // as the console sets them
	stop := e.runConsoleConn(t, s, &syncBuffer{})
	stop()
	require.Eventually(t, func() bool { return len(e.f.endsOf(s.ID)) == 1 }, 2*time.Second, 5*time.Millisecond)
	require.Equal(t, &recorder.SessionResult{Command: "ls", Classes: []string{"read"}, Paths: []string{"/srv"}}, endResult(s.ID))

	x, err := e.p.Open(t.Context(), e.cmdReq("c", KindExec, "ls"))
	require.NoError(t, err)
	stop = e.runConsoleConn(t, x, &syncBuffer{})
	stop()
	require.Eventually(t, func() bool { return len(e.f.endsOf(x.ID)) == 1 }, 2*time.Second, 5*time.Millisecond)
	require.Nil(t, endResult(x.ID))
}

// Ruling O1: a running terminal with a command ends when a new terminal rule
// takes its shell decision away; an exec without a PTY stays.
func TestSweeperRevokesPTYExecOnTerminalRule(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		e := pathsPipeEnv(t, ids{}, pathsPolicies)
		startSweeper(t, e.p)
		pty, err := e.p.Open(t.Context(), e.cmdReq("c", KindTerminal, "tail -f log"))
		require.NoError(t, err)
		exec, err := e.p.Open(t.Context(), e.cmdReq("c", KindExec, "tail -f log"))
		require.NoError(t, err)
		e.runConsoleConn(t, pty, &syncBuffer{})
		e.runConsoleConn(t, exec, &syncBuffer{})
		e.f.setPolicy(pathsPolicy(t, e.m, "v2", pathsPolicies+consoleOn("rc")))
		require.Equal(t, []string{"access_revoked"}, e.endedWith(pty.ID))
		require.Empty(t, e.endedWith(exec.ID), "an exec without a PTY takes only the exec checks")
	})
}
