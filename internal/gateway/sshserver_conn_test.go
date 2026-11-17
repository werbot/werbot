package gateway

import (
	"bytes"
	"io"
	"net"
	"runtime"
	"slices"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"
	gossh "golang.org/x/crypto/ssh"
	"google.golang.org/protobuf/types/known/durationpb"
)

func TestRefusedKeyLogged(t *testing.T) {
	e := newSSHEnv(t, Config{})
	k := newKey(t)
	_, err := e.dial("alice+db", k)
	require.Error(t, err)
	require.Contains(t, e.log.String(), `msg="ssh: key refused" login=alice client_ip=127.0.0.1`)
	require.NotContains(t, e.log.String(), fp(k), "no key material in the log")
}

func TestForwardRequiresResource(t *testing.T) {
	e := newSSHEnv(t, Config{})
	_, err := e.client(t, "alice").Dial("tcp", "10.0.0.1:80")
	require.ErrorContains(t, err, "prohibited")
}

func TestForwardDenied(t *testing.T) {
	e := newSSHEnv(t, Config{})
	_, err := e.client(t, "alice+denied").Dial("tcp", "10.0.0.1:80")
	require.ErrorContains(t, err, "prohibited")
	require.Zero(t, e.startCalls())
}

func TestEndSessionOnceOnClientDrop(t *testing.T) {
	e := newSSHEnv(t, Config{})
	c := e.client(t, "alice+blocker")
	s, err := c.NewSession()
	require.NoError(t, err)
	require.NoError(t, s.RequestPty("xterm", 40, 120, gossh.TerminalModes{}))
	require.NoError(t, s.Shell())
	require.Eventually(t, func() bool { return e.startCalls() == 1 }, 2*time.Second, 10*time.Millisecond)

	require.NoError(t, c.Close())
	ends := func() int {
		var n int
		e.f.with(func(f *fakeControl) { n = len(f.ends) })
		return n
	}
	require.Eventually(t, func() bool { return ends() == 1 }, 2*time.Second, 10*time.Millisecond)
	time.Sleep(100 * time.Millisecond)
	require.Equal(t, 1, ends())
}

// A client multiplexing sessions closes one channel and keeps the connection.
func TestChannelCloseEndsPTYSession(t *testing.T) {
	e := newSSHEnv(t, Config{})
	s, err := e.client(t, "alice+blocker").NewSession()
	require.NoError(t, err)
	require.NoError(t, s.RequestPty("xterm", 40, 120, gossh.TerminalModes{}))
	require.NoError(t, s.Shell())
	require.Eventually(t, func() bool { return e.startCalls() == 1 }, 2*time.Second, 10*time.Millisecond)

	require.NoError(t, s.Close())
	require.Eventually(t, func() bool {
		var n int
		e.f.with(func(f *fakeControl) { n = len(f.ends) })
		return n == 1
	}, 2*time.Second, 10*time.Millisecond)
}

// The connection idle limit reaps only connections that run no session: a running
// session is ended by the sweeper with its resource's idle_timeout (30m here).
func TestIdleTimeout(t *testing.T) {
	const idle = 200 * time.Millisecond
	e := newSSHEnv(t, Config{IdleTimeout: idle})
	snap := testSnapshot(t, e.m, sshPolicies, nil, res{"rblocker", "blocker", "blocker"})
	snap.Resources[0].IdleTimeout = durationpb.New(30 * time.Minute)
	pol, err := CompilePolicy(snap)
	require.NoError(t, err)
	e.f.setPolicy(pol)
	startSweeper(t, e.p)
	closed := func(c *gossh.Client) <-chan error {
		done := make(chan error, 1)
		go func() { done <- c.Wait() }()
		return done
	}

	busy := e.client(t, "alice+blocker")
	s, err := busy.NewSession()
	require.NoError(t, err)
	require.NoError(t, s.Shell())
	busyDone := closed(busy)
	bare := closed(e.client(t, "alice+blocker")) // no channel
	select {
	case <-bare:
	case <-time.After(5 * idle):
		t.Fatal("a connection without a session is still open")
	}
	select {
	case <-busyDone:
		t.Fatal("a connection with a running session was closed")
	case <-time.After(5 * idle):
	}

	require.NoError(t, s.Close()) // the session ends; the connection idles out after it
	select {
	case <-busyDone:
	case <-time.After(5 * idle):
		t.Fatal("the connection outlived its last session by more than the idle timeout")
	}
}

func TestProxyProtocol(t *testing.T) {
	e := newSSHEnv(t, Config{})
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := e.serve(t, e.front(Config{}, e.host), wrapListener(ln, true))
	cfg := &gossh.ClientConfig{User: "alice+db", Auth: []gossh.AuthMethod{gossh.PublicKeys(e.keys["alice"])},
		HostKeyCallback: gossh.FixedHostKey(e.host.PublicKey()), Timeout: 5 * time.Second}

	conn, err := net.Dial("tcp", addr)
	require.NoError(t, err)
	_, err = io.WriteString(conn, "PROXY TCP4 203.0.113.7 127.0.0.1 5555 2222\r\n")
	require.NoError(t, err)
	cc, chans, reqs, err := gossh.NewClientConn(conn, addr, cfg)
	require.NoError(t, err)
	c := gossh.NewClient(cc, chans, reqs)
	defer c.Close() //nolint:errcheck // test client
	_, code := run(t, c, false, "uptime")
	require.Equal(t, 0, code)
	e.f.with(func(f *fakeControl) {
		require.Len(t, f.starts, 1)
		require.Contains(t, f.starts[0], " 203.0.113.7")
	})

	conn, err = net.Dial("tcp", addr)
	require.NoError(t, err)
	_, _, _, err = gossh.NewClientConn(conn, addr, cfg)
	require.Error(t, err, "a connection without the header is refused")
}

func TestHandshakeTimeout(t *testing.T) {
	e := newSSHEnv(t, Config{})
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	f := e.front(Config{}, e.host)
	f.handshakeTimeout = 200 * time.Millisecond
	e.addr = e.serve(t, f, ln)

	silent, err := net.Dial("tcp", e.addr)
	require.NoError(t, err)
	defer silent.Close() //nolint:errcheck // test client
	require.NoError(t, silent.SetReadDeadline(time.Now().Add(2*time.Second)))
	start := time.Now()
	_, err = io.ReadAll(silent) // the server banner, then EOF
	require.NoError(t, err, "closed by the server, not by the read deadline")
	require.Less(t, time.Since(start), time.Second)

	s, err := e.client(t, "alice+blocker").NewSession()
	require.NoError(t, err)
	require.NoError(t, s.Shell())
	time.Sleep(400 * time.Millisecond)
	_, err = s.SendRequest("ping@werbot", true, nil)
	require.NoError(t, err, "an authenticated connection outlives the handshake limit")
}

// Window changes before shell or exec must not hang the channel's request loop:
// gliderlabs would block on them forever, even after the connection closes.
func TestEarlyWindowChangesDoNotLeak(t *testing.T) {
	e := newSSHEnv(t, Config{})
	e.f.with(func(f *fakeControl) { f.authErr = connect.NewError(connect.CodeUnavailable, nil) })
	before := runtime.NumGoroutine()
	for range 5 {
		c := e.client(t, "alice+db")
		s, err := c.NewSession()
		require.NoError(t, err)
		require.NoError(t, s.RequestPty("xterm", 40, 120, gossh.TerminalModes{}))
		require.NoError(t, s.WindowChange(30, 80))
		require.NoError(t, s.WindowChange(31, 81))
		var out bytes.Buffer
		s.Stdout = &out
		require.NoError(t, s.Shell())
		_ = s.WindowChange(40, 100) // races the server closing the channel
		require.Equal(t, 1, exitCode(t, s.Wait()))
		require.Equal(t, "Gateway is not ready, try again later\r\n", out.String())
		require.NoError(t, c.Close())
	}
	require.Eventually(t, func() bool { return runtime.NumGoroutine() <= before+2 }, 2*time.Second, 20*time.Millisecond,
		"goroutines left behind by closed connections")
}

func TestHandshakeConnCloseStopsTimer(t *testing.T) {
	a, b := net.Pipe()
	defer b.Close() //nolint:errcheck // test pipe
	c := &handshakeConn{Conn: a, timer: time.AfterFunc(time.Hour, func() {})}
	require.NoError(t, c.Close())
	require.False(t, c.timer.Stop(), "Close already stopped the timer")
}

// A not-ready connection gets only the not ready text, so the handshake limit
// still closes it.
func TestNotReadyConnClosedAtHandshakeLimit(t *testing.T) {
	e := newSSHEnv(t, Config{})
	e.f.with(func(f *fakeControl) { f.authErr = connect.NewError(connect.CodeUnavailable, nil) })
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	f := e.front(Config{}, e.host)
	f.handshakeTimeout = 200 * time.Millisecond
	e.addr = e.serve(t, f, ln)

	c := e.client(t, "alice+db")
	done := make(chan error, 1)
	go func() { done <- c.Wait() }()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("the not-ready connection outlived the handshake limit")
	}
}

// A client multiplexing sessions closes an exec channel without a PTY and keeps the connection.
func TestChannelCloseEndsExecSession(t *testing.T) {
	e := newSSHEnv(t, Config{})
	s, err := e.client(t, "alice+blocker").NewSession()
	require.NoError(t, err)
	require.NoError(t, s.Start("sleep"))
	require.Eventually(t, func() bool { return e.startCalls() == 1 }, 2*time.Second, 10*time.Millisecond)

	require.NoError(t, s.Close())
	require.Eventually(t, func() bool {
		var reasons []string
		e.f.with(func(f *fakeControl) {
			for _, end := range f.ends {
				reasons = append(reasons, end.reason)
			}
		})
		return slices.Equal(reasons, []string{"closed"})
	}, 2*time.Second, 10*time.Millisecond)
}
