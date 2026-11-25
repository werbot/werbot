package sshconnector

import (
	"context"
	"errors"
	"io"
	"net"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/gateway"
	"github.com/werbot/werbot/internal/gateway/sshtest"
)

// freePort is a TCP port free on 127.0.0.1 a moment ago.
func freePort(t *testing.T) uint32 {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer ln.Close()
	return uint32(ln.Addr().(*net.TCPAddr).Port) //nolint:gosec // a TCP port
}

// echoForward is a remote forward on port whose client side echoes every
// connection; the first failOpens opens fail. It records the origins.
type echoForward struct {
	gateway.RemoteForward
	ready     chan struct{}
	mu        sync.Mutex
	origins   []string
	failOpens int
}

func newEchoForward(host string, port uint32, failOpens int) *echoForward {
	f := &echoForward{ready: make(chan struct{}), failOpens: failOpens}
	f.BindHost, f.BindPort = host, port
	f.Ready = func() { close(f.ready) }
	f.Open = func(addr string, _ uint32) (io.ReadWriteCloser, error) {
		f.mu.Lock()
		defer f.mu.Unlock()
		f.origins = append(f.origins, addr)
		if f.failOpens > 0 {
			f.failOpens--
			return nil, errors.New("client refused the channel")
		}
		user, conn := net.Pipe()
		go func() { defer user.Close(); _, _ = io.Copy(user, user) }()
		return conn, nil
	}
	return f
}

// waitReady waits for Ready; Serve must not end first.
func (f *echoForward) waitReady(t *testing.T, served <-chan error) {
	t.Helper()
	select {
	case <-f.ready:
	case err := <-served:
		t.Fatalf("Serve ended before Ready: %v", err)
	case <-time.After(waitFor):
		t.Fatal("not ready")
	}
}

// dialEcho connects to addr at the target and checks the echo of "ping".
func dialEcho(t *testing.T, addr string) {
	t.Helper()
	c, err := net.DialTimeout("tcp", addr, waitFor)
	require.NoError(t, err)
	defer c.Close()
	require.NoError(t, c.SetDeadline(time.Now().Add(waitFor)))
	_, err = io.WriteString(c, "ping")
	require.NoError(t, err)
	buf := make([]byte, 4)
	_, err = io.ReadFull(c, buf)
	require.NoError(t, err)
	require.Equal(t, "ping", string(buf))
}

// The connector listens on the bind address at the target (an empty host is
// localhost), is ready once it does, and relays every connection to a
// forwarded-tcpip channel; a refused channel drops only its connection. A
// cancel stops the listener at the target and is not an error.
func TestRemoteForwardRoundTrip(t *testing.T) {
	tg := sshtest.StartTarget(t, "pw", nil)
	for _, host := range []string{"", "127.0.0.1"} {
		port := freePort(t)
		addr := net.JoinHostPort("127.0.0.1", strconv.FormatUint(uint64(port), 10))
		f := newEchoForward(host, port, 1)
		s := newSession(t, tg, gateway.KindStream)
		s.Target, s.RemoteForward = net.JoinHostPort("localhost", strconv.FormatUint(uint64(port), 10)), &f.RemoteForward
		ctx, cancel := context.WithCancel(t.Context())
		served := serve(ctx, s)
		f.waitReady(t, served)

		c, err := net.DialTimeout("tcp", addr, waitFor)
		require.NoError(t, err)
		require.NoError(t, c.SetDeadline(time.Now().Add(waitFor)))
		_, err = c.Read(make([]byte, 1))
		require.Error(t, err, "a refused channel closes its connection")
		_ = c.Close()
		dialEcho(t, addr)
		dialEcho(t, addr)
		f.mu.Lock()
		require.Equal(t, []string{"127.0.0.1", "127.0.0.1", "127.0.0.1"}, f.origins, host)
		f.mu.Unlock()

		cancel()
		require.NoError(t, wait(t, served), host)
		require.Eventually(t, func() bool {
			c, err := net.DialTimeout("tcp", addr, time.Second)
			if err == nil {
				_ = c.Close()
			}
			return err != nil
		}, waitFor, 10*time.Millisecond, "the target stops listening")
	}
}

// A target that refuses tcpip-forward fails Serve without Ready.
func TestRemoteForwardTargetRefuses(t *testing.T) {
	tg := sshtest.StartTarget(t, "pw", nil, sshtest.NoRemoteForward)
	f := newEchoForward("localhost", freePort(t), 0)
	s := newSession(t, tg, gateway.KindStream)
	s.Target, s.RemoteForward = "localhost:1", &f.RemoteForward
	require.Error(t, wait(t, serve(t.Context(), s)))
	select {
	case <-f.ready:
		t.Fatal("Ready after a refused tcpip-forward")
	default:
	}
}

// Connections open at the cancel are closed: Serve does not wait for them.
func TestRemoteForwardCancelClosesConnections(t *testing.T) {
	tg := sshtest.StartTarget(t, "pw", nil)
	port := freePort(t)
	f := newEchoForward("localhost", port, 0)
	s := newSession(t, tg, gateway.KindStream)
	s.RemoteForward = &f.RemoteForward
	ctx, cancel := context.WithCancel(t.Context())
	served := serve(ctx, s)
	f.waitReady(t, served)
	c, err := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", strconv.FormatUint(uint64(port), 10)), waitFor)
	require.NoError(t, err)
	defer c.Close()
	_, err = io.WriteString(c, "ping")
	require.NoError(t, err)
	_, err = io.ReadFull(c, make([]byte, 4))
	require.NoError(t, err)

	cancel()
	require.NoError(t, wait(t, served))
	require.NoError(t, c.SetDeadline(time.Now().Add(waitFor)))
	_, err = c.Read(make([]byte, 1))
	require.Error(t, err, "the connection is closed")
}
