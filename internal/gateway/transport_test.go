package gateway

import (
	"context"
	"errors"
	"io"
	"net"
	"net/netip"
	"testing"
	"testing/synctest"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	gossh "golang.org/x/crypto/ssh"
)

// wsListener hands out connections that report the transport "ws", as the
// WebSocket front's do.
type wsListener struct{ net.Listener }

func (l wsListener) Accept() (net.Conn, error) {
	c, err := l.Listener.Accept()
	if err != nil {
		return nil, err
	}
	return wsConn{c}, nil
}

type wsConn struct{ net.Conn }

func (wsConn) Transport() string { return "ws" }

var errBroken = errors.New("listener broken")

// brokenListener fails its first Accept for good.
type brokenListener struct{ net.Listener }

func (brokenListener) Accept() (net.Conn, error) { return nil, errBroken }

func listenLocal(t *testing.T) net.Listener {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	return ln
}

func TestOpenPassesTransport(t *testing.T) {
	e := newPipeEnv(t, 10)
	r := e.req("fake", KindTerminal)
	r.Transport = "ws"
	s, err := e.p.Open(t.Context(), r)
	require.NoError(t, err)
	assert.Equal(t, "ws", sessionAgent(s.ID).Transport, "StartSession gets the transport of the request")

	s = e.open(t, "fake", KindTerminal)
	assert.Equal(t, "tcp", sessionAgent(s.ID).Transport, "no transport is tcp")
}

func TestStartSessionSendsTransport(t *testing.T) {
	f := &webTunnel{}
	c := newWebTunnel(t, f)
	_, _, err := c.StartSession(t.Context(), testUserID, uuid.New(), netip.MustParseAddr("192.0.2.7"), KindExec, "", "ls", nil, nil,
		startArgs{Front: "ssh", Transport: "ws"})
	require.NoError(t, err)
	assert.Equal(t, "ws", f.starts[0].GetTransport())
}

func TestSSHFrontTransport(t *testing.T) {
	e := newSSHEnv(t, Config{})
	_, code := run(t, e.client(t, "alice+db"), false, "uptime")
	require.Zero(t, code)
	assert.Equal(t, "tcp", sessionAgent(e.lastEnd(t, 1)).Transport, "a plain TCP connection")

	e.addr = e.serve(t, e.front(Config{}, e.host), wsListener{listenLocal(t)})
	_, code = run(t, e.client(t, "alice+db"), false, "uptime")
	require.Zero(t, code)
	assert.Equal(t, "ws", sessionAgent(e.lastEnd(t, 2)).Transport, "a connection that reports its transport")

	addr := e.serve(t, e.front(Config{}, e.host), wrapListener(listenLocal(t), true))
	conn, err := net.Dial("tcp", addr)
	require.NoError(t, err)
	_, err = io.WriteString(conn, "PROXY TCP4 203.0.113.7 127.0.0.1 5555 2222\r\n")
	require.NoError(t, err)
	cc, chans, reqs, err := gossh.NewClientConn(conn, addr, &gossh.ClientConfig{User: "alice+db",
		Auth:            []gossh.AuthMethod{gossh.PublicKeys(e.keys["alice"])},
		HostKeyCallback: gossh.FixedHostKey(e.host.PublicKey()), Timeout: 5 * time.Second})
	require.NoError(t, err)
	c := gossh.NewClient(cc, chans, reqs)
	defer c.Close() //nolint:errcheck // test client
	_, code = run(t, c, false, "uptime")
	require.Zero(t, code)
	assert.Equal(t, "tcp", sessionAgent(e.lastEnd(t, 3)).Transport, "a PROXY protocol connection")
}

func TestSSHFrontTwoListeners(t *testing.T) {
	e := newSSHEnv(t, Config{})
	tcpLn, wsLn := listenLocal(t), wsListener{listenLocal(t)}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- serveFronts(ctx, e.front(Config{}, e.host), e.p, tcpLn, nil, wsLn) }() // nil: a front that is off

	for _, addr := range []string{tcpLn.Addr().String(), wsLn.Addr().String()} {
		e.addr = addr // e.dial pins e.host: both listeners present the same host key
		out, code := run(t, e.client(t, "alice+db"), false, "uptime")
		require.Equal(t, "fake:db user=alice secret=s3cret-value kind=exec\n", out)
		require.Zero(t, code)
	}
	s, err := e.client(t, "alice+blocker").NewSession()
	require.NoError(t, err)
	require.NoError(t, s.Start("wait"))
	waited := make(chan struct{})
	go func() { _ = s.Wait(); close(waited) }()
	require.Eventually(t, func() bool { return len(e.p.active.ids()) == 1 }, 2*time.Second, 5*time.Millisecond)

	cancel()
	for _, addr := range []string{tcpLn.Addr().String(), wsLn.Addr().String()} {
		require.Eventually(t, func() bool {
			c, err := net.Dial("tcp", addr)
			if err == nil {
				_ = c.Close()
			}
			return err != nil
		}, 2*time.Second, 5*time.Millisecond, "every listener closes")
	}
	require.Len(t, e.p.active.ids(), 1, "the session outlives the listeners")
	select {
	case <-done:
		t.Fatal("serveFronts returned before the pipeline stopped")
	default:
	}

	start := time.Now()
	e.p.drain(200 * time.Millisecond)
	require.GreaterOrEqual(t, time.Since(start), 200*time.Millisecond, "the session lives until DrainTimeout")
	require.NoError(t, <-done)
	e.f.with(func(f *fakeControl) { require.Len(t, f.starts, 3) })
	assert.Equal(t, "gateway_shutdown", e.endReason(t, 3))
	select {
	case <-waited:
	case <-time.After(2 * time.Second):
		t.Fatal("the SSH connection is still open after the pipeline stopped")
	}
}

func TestServeFrontsListenerFailure(t *testing.T) {
	e := newSSHEnv(t, Config{})
	good, bad := listenLocal(t), brokenListener{listenLocal(t)}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- serveFronts(ctx, e.front(Config{}, e.host), e.p, good, bad) }()

	require.Eventually(t, e.p.active.isDraining, 2*time.Second, 5*time.Millisecond, "a failed listener starts the drain")
	_, err := net.Dial("tcp", bad.Addr().String())
	require.Error(t, err, "the failed listener is closed")
	e.addr = good.Addr().String()
	c, err := e.dial("alice+db", e.keys["alice"])
	require.NoError(t, err, "the other listener keeps serving until the drain ends its context")
	_ = c.Close()
	select {
	case <-done:
		t.Fatal("serveFronts returned before every listener stopped")
	default:
	}

	cancel()
	e.p.stopNow()
	err = <-done
	require.ErrorIs(t, err, errBroken)
	require.ErrorContains(t, err, "ssh front: ")
}

func TestWebFrontTransportInternal(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		w := newWebEnv(t, 10)
		st := w.open(t, terminalTicket("echo"))
		synctest.Wait()
		assert.Equal(t, startArgs{Front: "web", Transport: "internal"}, sessionAgent(w.session(t)), "the web terminal is internal")
		st.in <- browserClosed
		assert.Equal(t, "closed", st.closedWith(t))
	})
}

// endReason waits for the n-th session end and returns its reason.
func (e *sshEnv) endReason(t *testing.T, n int) string {
	t.Helper()
	id := e.lastEnd(t, n)
	var reason string
	e.f.with(func(f *fakeControl) {
		for _, end := range f.ends {
			if end.id == id {
				reason = end.reason
			}
		}
	})
	return reason
}
