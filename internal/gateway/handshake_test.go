package gateway

import (
	"sync"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"
)

type testClock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *testClock) now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *testClock) add(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = c.t.Add(d)
}

func newClockedSSHEnv(t *testing.T) (*sshEnv, *testClock) {
	t.Helper()
	clk := &testClock{t: time.Unix(1_700_000_000, 0)}
	return newSSHEnvWith(t, Config{}, func(p *Pipeline) { p.now = clk.now }), clk
}

func (e *sshEnv) authCount() int {
	var n int
	e.f.with(func(f *fakeControl) { n = len(f.authCalls) })
	return n
}

func TestChannelReusesHandshakeUser(t *testing.T) {
	e, clk := newClockedSSHEnv(t)
	c := e.client(t, "alice+db")
	require.Equal(t, 1, e.authCount(), "the handshake")

	_, code := run(t, c, false, "uptime")
	require.Equal(t, 0, code)
	clk.add(handshakeReuse)
	_, code = run(t, c, false, "uptime")
	require.Equal(t, 0, code)
	require.Equal(t, 1, e.authCount(), "channels within handshakeReuse use the handshake user")

	clk.add(time.Nanosecond)
	out, code := run(t, c, false, "uptime")
	require.Equal(t, 0, code, out)
	require.Equal(t, 2, e.authCount(), "a later channel authenticates again")
	e.f.with(func(f *fakeControl) {
		require.Len(t, f.starts, 3)
		for _, s := range f.starts {
			require.Contains(t, s, e.m["u1"].String())
		}
	})
}

// A key revoked after the window is refused on the next channel, as today.
func TestChannelAfterWindowSeesRevokedKey(t *testing.T) {
	e, clk := newClockedSSHEnv(t)
	c := e.client(t, "alice+db")
	e.f.with(func(f *fakeControl) { delete(f.byKey, fp(e.keys["alice"])) })
	clk.add(handshakeReuse + time.Nanosecond)
	out, code := run(t, c, false, "uptime")
	require.Equal(t, "Access denied\r\n", out)
	require.Equal(t, 1, code)
	require.Zero(t, e.startCalls())
}

// Without a handshake user there is nothing to reuse: every channel says not ready.
func TestNoHandshakeUserNotReused(t *testing.T) {
	e, _ := newClockedSSHEnv(t)
	e.f.with(func(f *fakeControl) { f.authErr = connect.NewError(connect.CodeUnavailable, nil) })
	c := e.client(t, "alice+db")
	e.f.with(func(f *fakeControl) { f.authErr = nil })
	out, code := run(t, c, false, "uptime")
	require.Equal(t, "Gateway is not ready, try again later\r\n", out)
	require.Equal(t, 1, code)
	require.Equal(t, 1, e.authCount())
	require.Zero(t, e.startCalls())
}

// The snapshot checks run on a reused handshake user.
func TestHandshakeUserStillChecked(t *testing.T) {
	e := newPipeEnv(t, 10)
	clk := &testClock{t: time.Unix(1_700_000_000, 0)}
	e.p.now = clk.now
	r := e.req("denied", KindTerminal)
	r.Handshake = &Handshake{User: User{ID: e.m["u1"], Login: "alice"}, At: clk.now()}
	_, err := e.p.Open(t.Context(), r)
	require.ErrorIs(t, err, ErrDenied)
	r.Resource = "fake"
	s, err := e.p.Open(t.Context(), r)
	require.NoError(t, err)
	require.Equal(t, e.m["u1"], s.User.ID)
	e.f.with(func(f *fakeControl) { require.Empty(t, f.authCalls) })
}
