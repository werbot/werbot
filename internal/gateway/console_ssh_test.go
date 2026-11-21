package gateway

import (
	"bytes"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	gossh "golang.org/x/crypto/ssh"
)

// A refused shell reads why and exits 1; an exec of the same login still runs.
func TestShellDenied(t *testing.T) {
	e := newSSHEnv(t, Config{})
	p := pathsPolicy(t, e.m, "v1", pathsPolicies+`
@id("terminal:d")
forbid (principal, action in [Werbot::Action::"execy.shell"], resource);`)
	e.f.with(func(f *fakeControl) { f.policy = p })
	c := e.client(t, "alice+x")
	out, code := run(t, c, true, "")
	require.Equal(t, "Interactive shell is not allowed on x\r\n", out)
	require.Equal(t, 1, code)
	ds := e.p.denials.take()
	require.Len(t, ds, 1)
	require.Equal(t, "terminal", ds[0].GetKind())
	require.Equal(t, "denied", ds[0].GetReason())
	out, code = run(t, c, false, "ls")
	require.Equal(t, "spy:x kind=exec cmd=\"ls\"\n", out)
	require.Equal(t, 7, code)
}

// Ruling C14: a console line that waits for an approval inside Run keeps the
// connection alive with keepalives, as an exec waiting in Open does.
func TestApprovalKeepaliveInsideRun(t *testing.T) {
	const idle = 200 * time.Millisecond
	cfg := Config{IdleTimeout: idle, MaxSessions: 10}
	e := newSSHEnv(t, cfg)
	e.m["rcx"] = uuid.New()
	p := compileTest(t, e.m, pathsPolicies+`
@id("terminal:t")
@obligation("console")
forbid (principal, action in [Werbot::Action::"consoley.shell"], resource) unless { context.console };
@id("command:w")
@obligation("approval")
forbid (principal, action == Werbot::Action::"consoley.exec", resource) when { context.classes.contains("write") }
unless { context.approval_granted };`, nil, res{"rcx", "x", "consoley"})
	a := &fakeApprovals{ttl: time.Minute, statuses: []string{"pending"}}
	e.f.with(func(f *fakeControl) { f.policy, f.approvals = p, a })
	f := e.front(cfg, e.host)
	f.keepalive = 50 * time.Millisecond
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	e.addr = e.serve(t, f, ln)

	var keepalives atomic.Int32
	s, err := e.keepaliveClient(t, &keepalives).NewSession() // alice+x: the console of "x" checks "mkdir x"
	require.NoError(t, err)
	require.NoError(t, s.RequestPty("xterm", 40, 120, gossh.TerminalModes{}))
	var out bytes.Buffer
	s.Stdout = &out
	require.NoError(t, s.Shell())
	require.Eventually(t, func() bool { return a.waiting() == 1 }, 5*time.Second, 5*time.Millisecond)
	before := keepalives.Load()
	time.Sleep(5 * idle) // the wait outlasts the idle limit
	a.decide("approved")
	require.Equal(t, 0, exitCode(t, s.Wait()))
	require.Contains(t, out.String(), "Waiting for approval a1")
	require.Contains(t, out.String(), "checked {[write] [/home/u/x]} <nil>")
	require.GreaterOrEqual(t, keepalives.Load()-before, int32(4), "keepalives flow while the line waits")
}
