package app

import (
	"context"
	"io"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/connectcli"
)

// startTCPEcho echoes each connection until its end of input, then half-closes.
func startTCPEcho(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = ln.Close() })
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go func() {
				defer c.Close() //nolint:errcheck // test echo
				_, _ = io.Copy(c, c)
				_ = c.(*net.TCPConn).CloseWrite()
			}()
		}
	}()
	return ln.Addr().String()
}

// startUDPEcho answers each datagram with "re:" and the datagram.
func startUDPEcho(t *testing.T) string {
	t.Helper()
	pc, err := net.ListenPacket("udp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = pc.Close() })
	go func() {
		buf := make([]byte, 2048)
		for {
			n, a, err := pc.ReadFrom(buf)
			if err != nil {
				return
			}
			_, _ = pc.WriteTo(append([]byte("re:"), buf[:n]...), a)
		}
	}()
	return pc.LocalAddr().String()
}

// cutProxy relays TCP to a gateway; cut closes every relayed connection, as a
// killed instance behind a load balancer does.
type cutProxy struct {
	addr  string
	mu    sync.Mutex
	conns []net.Conn
}

func startCutProxy(t *testing.T, to string) *cutProxy {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	p := &cutProxy{addr: ln.Addr().String()}
	t.Cleanup(func() { _ = ln.Close(); p.cut() })
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			up, err := net.Dial("tcp", to)
			if err != nil {
				_ = c.Close()
				continue
			}
			p.mu.Lock()
			p.conns = append(p.conns, c, up)
			p.mu.Unlock()
			go func() { _, _ = io.Copy(up, c); _ = up.Close() }()
			go func() { _, _ = io.Copy(c, up); _ = c.Close() }()
		}
	}()
	return p
}

func (p *cutProxy) cut() {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, c := range p.conns {
		_ = c.Close()
	}
	p.conns = nil
}

// connectRun is connectcli.Run in the background.
type connectRun struct {
	stdout, stderr syncBuffer
	cancel         context.CancelFunc
	done           chan error
}

func runConnect(t *testing.T, cfg connectcli.Config) *connectRun {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	r := &connectRun{cancel: cancel, done: make(chan error, 1)}
	go func() { r.done <- connectcli.Run(ctx, cfg, &r.stdout, &r.stderr) }()
	t.Cleanup(func() { _ = r.stop(t) })
	return r
}

func (r *connectRun) ready(t *testing.T, resource, port, proto string) string {
	t.Helper()
	line := resource + " → 127.0.0.1:" + port + " (" + proto + ")\n"
	require.Eventually(t, func() bool { return strings.Contains(r.stdout.String(), line) },
		10*time.Second, 10*time.Millisecond, "stderr %q", r.stderr.String())
	return "127.0.0.1:" + port
}

func (r *connectRun) stop(t *testing.T) error {
	t.Helper()
	r.cancel()
	return r.wait(t)
}

func (r *connectRun) wait(t *testing.T) error {
	t.Helper()
	select {
	case err := <-r.done:
		r.done <- err
		return err
	case <-time.After(10 * time.Second):
		t.Fatal("connect did not return within 10s")
		return nil
	}
}
