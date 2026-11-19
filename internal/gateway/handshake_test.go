package gateway

import (
	"net"
	"sync"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/gliderlabs/ssh"
	"github.com/google/uuid"
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

// The window counts from before the handshake lookup: a slow AuthenticateUser
// uses up the window instead of extending it.
func TestReuseWindowStartsBeforeAuth(t *testing.T) {
	e, clk := newClockedSSHEnv(t)
	e.f.with(func(f *fakeControl) { f.onAuth = func() { clk.add(handshakeReuse * 3 / 4) } })
	c := e.client(t, "alice+db")
	require.Equal(t, 1, e.authCount())
	clk.add(handshakeReuse / 2) // 1.25 windows after the lookup started, 0.5 after it returned
	_, code := run(t, c, false, "uptime")
	require.Equal(t, 0, code)
	require.Equal(t, 2, e.authCount(), "the channel authenticates again")
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

// A reused handshake user takes the teams of the snapshot the channel is
// decided on: a team removed (or added) after the handshake counts at once,
// not at the next full re-evaluation.
func TestHandshakeUserTakesSnapshotTeams(t *testing.T) {
	e := newPipeEnv(t, 10)
	clk := &testClock{t: time.Unix(1_700_000_000, 0)}
	e.p.now = clk.now
	e.m["t1"], e.m["u2"] = uuid.New(), uuid.New()
	r := e.req("fake", KindExec)
	r.Handshake = &Handshake{User: User{ID: e.m["u1"], Login: "alice", TeamIDs: []uuid.UUID{e.m["t1"]}}, At: clk.now()}
	e.f.setPolicy(e.memberSnapshot(t, "v2", e.member("u1"), e.member("u2", "t1")))
	clk.add(handshakeReuse)
	_, err := e.p.Open(t.Context(), r)
	require.ErrorIs(t, err, ErrDenied, "the snapshot dropped t1 after the handshake")

	r.Handshake.User.TeamIDs = nil
	e.f.setPolicy(e.memberSnapshot(t, "v3", e.member("u1", "t1")))
	s, err := e.p.Open(t.Context(), r)
	require.NoError(t, err, "the snapshot added t1 after the handshake")
	require.Equal(t, []uuid.UUID{e.m["t1"]}, s.User.TeamIDs)

	r.Resource = "blocker"
	e.f.setPolicy(e.memberSnapshot(t, "v4", e.member("u2", "t1")))
	_, err = e.p.Open(t.Context(), r)
	require.ErrorIs(t, err, ErrDenied, "u1 left the organization")
	e.f.with(func(f *fakeControl) { require.Empty(t, f.authCalls) })
}

// remoteCtx adds the client address to fakeConnContext.
type remoteCtx struct{ *fakeConnContext }

func (remoteCtx) RemoteAddr() net.Addr { return &net.TCPAddr{IP: net.IPv4(192, 0, 2, 7), Port: 2222} }

// The handshake user is attached only when it was found by the signing key.
func TestRequestHandshakeNeedsSigningKey(t *testing.T) {
	signing, other := newKey(t), newKey(t)
	ctx := remoteCtx{&fakeConnContext{ctx: t.Context()}}
	ctx.SetValue(ssh.ContextKeyPublicKey, signing.PublicKey())
	h := &sshHandler{}
	alice := User{Login: "alice"}

	ctx.SetValue(userKey{}, &authed{u: alice, id: Identity{Method: "ssh-key", Value: fp(other)}, at: time.Unix(1, 0)})
	require.Nil(t, h.request(ctx, "db", KindTerminal, Window{}, "").Handshake, "another key's user")

	ctx.SetValue(userKey{}, &authed{u: alice, id: Identity{Method: "ssh-key", Value: fp(signing)}, at: time.Unix(1, 0)})
	r := h.request(ctx, "db", KindTerminal, Window{}, "")
	require.NotNil(t, r.Handshake)
	require.Equal(t, "alice", r.Handshake.User.Login)
	require.Equal(t, time.Unix(1, 0), r.Handshake.At)
}
