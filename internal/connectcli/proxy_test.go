package connectcli

import (
	"context"
	"io"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

// proxyRun is Proxy in the background with its output.
func proxyRun(ctx context.Context, ep Endpoint, in io.Reader) (*syncBuffer, chan error) {
	out, done := &syncBuffer{}, make(chan error, 1)
	go func() { done <- Proxy(ctx, ep, in, out) }()
	return out, done
}

func proxyDone(t *testing.T, done chan error) error {
	t.Helper()
	select {
	case err := <-done:
		return err
	case <-time.After(5 * time.Second):
		t.Fatal("Proxy did not return within 5s")
		return nil
	}
}

// Over TCP the end of input half-closes the connection; the echo's end ends Proxy.
func TestProxyCopies(t *testing.T) {
	out, done := proxyRun(t.Context(), Endpoint{SSHAddress: startTCPEcho(t)}, strings.NewReader("ping"))
	require.NoError(t, proxyDone(t, done))
	assert.Equal(t, "ping", out.String())
}

// Over WebSocket (no half-close) Proxy waits proxyDrain for the gateway's end
// after the end of input, then closes and returns nil.
func TestProxyWebSocketDrain(t *testing.T) {
	old := proxyDrain
	proxyDrain = 300 * time.Millisecond
	t.Cleanup(func() { proxyDrain = old })
	ws := startWSGateway(t, false)
	out, done := proxyRun(t.Context(), Endpoint{WSURL: ws.url("")}, strings.NewReader("ping"))
	require.NoError(t, proxyDone(t, done))
	assert.Equal(t, "SSH-2.0-ws\r\nping", out.String())
}

// The end of ctx ends Proxy with nil; a dial error is returned.
func TestProxyEnds(t *testing.T) {
	ws := startWSGateway(t, false)
	in, w := io.Pipe()
	t.Cleanup(func() { _ = w.Close() })
	ctx, cancel := context.WithCancel(t.Context())
	out, done := proxyRun(ctx, Endpoint{WSURL: ws.url("")}, in)
	require.Eventually(t, func() bool { return out.String() == "SSH-2.0-ws\r\n" }, 5*time.Second, 10*time.Millisecond)
	cancel()
	require.NoError(t, proxyDone(t, done))

	_, done = proxyRun(t.Context(), Endpoint{}, strings.NewReader(""))
	assert.EqualError(t, proxyDone(t, done), "gateway address is not set")
}
