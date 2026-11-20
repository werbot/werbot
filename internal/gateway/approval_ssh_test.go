package gateway

import (
	"bytes"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	gossh "golang.org/x/crypto/ssh"
)

// keepaliveClient logs in as alice+x and counts the keepalive@openssh.com
// requests of the gateway; the client answers them as OpenSSH does.
func (e *sshEnv) keepaliveClient(t *testing.T, keepalives *atomic.Int32) *gossh.Client {
	t.Helper()
	nc, err := net.DialTimeout("tcp", e.addr, 5*time.Second)
	require.NoError(t, err)
	cc, chans, reqs, err := gossh.NewClientConn(nc, e.addr, &gossh.ClientConfig{User: "alice+x",
		Auth: []gossh.AuthMethod{gossh.PublicKeys(e.keys["alice"])}, HostKeyCallback: gossh.FixedHostKey(e.host.PublicKey())})
	require.NoError(t, err)
	seen := make(chan *gossh.Request)
	go func() {
		defer close(seen)
		for r := range reqs {
			if r.Type == "keepalive@openssh.com" {
				keepalives.Add(1)
			}
			seen <- r
		}
	}()
	c := gossh.NewClient(cc, chans, seen)
	t.Cleanup(func() { _ = c.Close() })
	return c
}

// A connection that waits for an approval runs a session: the gateway idle
// limit does not close it, and keepalives flow while Open waits.
func TestApprovalWaitKeepsConnection(t *testing.T) {
	const idle = 200 * time.Millisecond
	cfg := Config{IdleTimeout: idle, MaxSessions: 10}
	e := newSSHEnv(t, cfg)
	e.f.with(func(f *fakeControl) { f.policy = execPolicy(t, e.m, "v1", approvalPolicies("")) })
	a := &fakeApprovals{ttl: time.Minute, statuses: []string{"pending"}}
	e.f.with(func(f *fakeControl) { f.approvals = a })
	f := e.front(cfg, e.host)
	f.keepalive = 50 * time.Millisecond
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	e.addr = e.serve(t, f, ln)

	var keepalives atomic.Int32
	s, err := e.keepaliveClient(t, &keepalives).NewSession()
	require.NoError(t, err)
	var out, errOut bytes.Buffer
	s.Stdout, s.Stderr = &out, &errOut
	require.NoError(t, s.Start("mkdir x"))
	require.Eventually(t, func() bool { return a.waiting() == 1 }, 5*time.Second, 5*time.Millisecond)
	time.Sleep(5 * idle) // the wait outlasts the idle limit
	a.decide("approved")
	require.Equal(t, 7, exitCode(t, s.Wait()), "the approved command ran")
	require.Equal(t, "spy:x kind=exec cmd=\"mkdir x\"\n", out.String())
	require.Equal(t, "Waiting for approval a1, timeout 1m\r\nhttps://werbot.test/requests/a1\r\n", errOut.String())
	require.GreaterOrEqual(t, keepalives.Load(), int32(5))
}

// Closing the waiting session channel ends the wait at once, while the
// connection stays open for other channels: a later approval runs nothing.
func TestApprovalWaitEndsWithChannel(t *testing.T) {
	e := newSSHEnv(t, Config{})
	e.f.with(func(f *fakeControl) { f.policy = execPolicy(t, e.m, "v1", approvalPolicies("")) })
	a := &fakeApprovals{ttl: time.Minute, statuses: []string{"pending"}}
	e.f.with(func(f *fakeControl) { f.approvals = a })
	c := e.client(t, "alice+x")

	waiting, err := c.NewSession()
	require.NoError(t, err)
	require.NoError(t, waiting.Start("mkdir x"))
	require.Eventually(t, func() bool { return a.waiting() == 1 }, 5*time.Second, 5*time.Millisecond)
	require.NoError(t, waiting.Close())
	require.Eventually(t, func() bool { return a.waiting() == 0 }, 5*time.Second, 5*time.Millisecond, "the wait ended")
	a.decide("approved")

	other, err := c.NewSession()
	require.NoError(t, err, "the connection is still open")
	out, err := other.Output("ls")
	require.Equal(t, 7, exitCode(t, err))
	require.Equal(t, "spy:x kind=exec cmd=\"ls\"\n", string(out))
	require.Equal(t, 1, e.startCalls(), "only the other channel started a session")
}
