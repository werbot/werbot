package connectcli

import (
	"bufio"
	"context"
	"crypto/tls"
	"crypto/x509"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var _ Dialer = Dial

// wsGateway is a WebSocket server standing in for the gateway's ws input: on
// any path it writes "SSH-2.0-ws\r\n", then echoes binary messages.
type wsGateway struct {
	srv   *httptest.Server
	hits  atomic.Int64
	mu    sync.Mutex
	paths []string
}

func startWSGateway(t *testing.T, tlsOn bool) *wsGateway {
	t.Helper()
	g := &wsGateway{}
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		g.hits.Add(1)
		g.mu.Lock()
		g.paths = append(g.paths, r.URL.Path)
		g.mu.Unlock()
		c, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		nc := websocket.NetConn(context.Background(), c, websocket.MessageBinary)
		defer nc.Close() //nolint:errcheck // test server
		if _, err := io.WriteString(nc, "SSH-2.0-ws\r\n"); err != nil {
			return
		}
		_, _ = io.Copy(nc, nc)
	})
	if tlsOn {
		g.srv = httptest.NewTLSServer(h)
	} else {
		g.srv = httptest.NewServer(h)
	}
	t.Cleanup(g.srv.Close)
	return g
}

// url is the server's ws:// or wss:// URL with path.
func (g *wsGateway) url(path string) string {
	return "ws" + strings.TrimPrefix(g.srv.URL, "http") + path
}

func (g *wsGateway) lastPath() string {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.paths[len(g.paths)-1]
}

// startSSHBanner accepts TCP connections and writes "SSH-2.0-tcp\r\n" to each.
func startSSHBanner(t *testing.T) string {
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
			_, _ = io.WriteString(c, "SSH-2.0-tcp\r\n")
			_ = c.Close()
		}
	}()
	return ln.Addr().String()
}

// closedAddr is a loopback address nobody listens on.
func closedAddr(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := ln.Addr().String()
	require.NoError(t, ln.Close())
	return addr
}

// banner reads the first line of c.
func banner(t *testing.T, c net.Conn) string {
	t.Helper()
	require.NoError(t, c.SetReadDeadline(time.Now().Add(5*time.Second)))
	line, err := bufio.NewReader(c).ReadString('\n')
	require.NoError(t, err)
	require.NoError(t, c.SetReadDeadline(time.Time{}))
	return strings.TrimRight(line, "\r\n")
}

func TestDial(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer ln.Close() //nolint:errcheck // test listener

	c, err := Dial(t.Context(), Endpoint{SSHAddress: ln.Addr().String()})
	require.NoError(t, err)
	require.NoError(t, c.Close())
}

// Both addresses with the SSH port open: SSH, the WebSocket URL is not tried.
func TestDialPrefersSSH(t *testing.T) {
	ws := startWSGateway(t, false)
	c, err := Dial(t.Context(), Endpoint{SSHAddress: startSSHBanner(t), WSURL: ws.url("")})
	require.NoError(t, err)
	defer c.Close() //nolint:errcheck // test conn
	assert.Equal(t, "SSH-2.0-tcp", banner(t, c))
	assert.Zero(t, ws.hits.Load())
}

// A closed SSH port or no SSH address: the WebSocket URL; the connection's
// remote address is the URL's host:port, so known_hosts can check it.
func TestDialFallsBackToWebSocket(t *testing.T) {
	ws := startWSGateway(t, false)
	host := strings.TrimPrefix(ws.srv.URL, "http://")
	for _, ssh := range []string{closedAddr(t), ""} {
		c, err := Dial(t.Context(), Endpoint{SSHAddress: ssh, WSURL: ws.url("")})
		require.NoError(t, err, ssh)
		assert.Equal(t, "SSH-2.0-ws", banner(t, c), ssh)
		assert.Equal(t, host, c.RemoteAddr().String())
		require.NoError(t, c.Close())
	}
	assert.EqualValues(t, 2, ws.hits.Load())
}

// An SSH address that does not answer gives way to the WebSocket URL after sshDialTimeout.
func TestDialFallsBackAfterTimeout(t *testing.T) {
	old := sshDialTimeout
	sshDialTimeout = 200 * time.Millisecond
	t.Cleanup(func() { sshDialTimeout = old })
	ws := startWSGateway(t, false)
	start := time.Now()
	c, err := Dial(t.Context(), Endpoint{SSHAddress: "192.0.2.1:2222", WSURL: ws.url("")}) // TEST-NET-1
	require.NoError(t, err)
	defer c.Close() //nolint:errcheck // test conn
	assert.Less(t, time.Since(start), time.Second)
	assert.Equal(t, "SSH-2.0-ws", banner(t, c))
}

func TestDialErrors(t *testing.T) {
	_, err := Dial(t.Context(), Endpoint{})
	assert.EqualError(t, err, "gateway address is not set")

	ssh, ws := closedAddr(t), "ws://"+closedAddr(t)+"/v1/ssh"
	_, err = Dial(t.Context(), Endpoint{SSHAddress: ssh, WSURL: ws})
	require.Error(t, err)
	assert.True(t, strings.HasPrefix(err.Error(), "gateway unreachable: ssh "+ssh+": "), err)
	assert.Contains(t, err.Error(), "; websocket "+ws+": ")
}

// A wss:// URL goes through the proxy of proxyFunc with CONNECT to the gateway's host:port.
func TestDialHonoursProxy(t *testing.T) {
	ws := startWSGateway(t, true)
	roots := x509.NewCertPool()
	roots.AddCert(ws.srv.Certificate())
	var seen atomic.Value
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodConnect {
			http.Error(w, "CONNECT only", http.StatusMethodNotAllowed)
			return
		}
		seen.Store(r.Host)
		up, err := net.Dial("tcp", r.Host) //nolint:gosec // test proxy on loopback
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		w.WriteHeader(http.StatusOK)
		down, _, err := http.NewResponseController(w).Hijack()
		if err != nil {
			_ = up.Close()
			return
		}
		go func() { _, _ = io.Copy(up, down); _ = up.Close() }()
		_, _ = io.Copy(down, up)
		_ = down.Close()
	}))
	t.Cleanup(proxy.Close)
	pu, err := url.Parse(proxy.URL)
	require.NoError(t, err)
	oldProxy, oldTLS := proxyFunc, wsTLS
	proxyFunc, wsTLS = http.ProxyURL(pu), &tls.Config{RootCAs: roots, MinVersion: tls.VersionTLS12}
	t.Cleanup(func() { proxyFunc, wsTLS = oldProxy, oldTLS })

	c, err := Dial(t.Context(), Endpoint{WSURL: ws.url("/v1/ssh")})
	require.NoError(t, err)
	defer c.Close() //nolint:errcheck // test conn
	assert.Equal(t, "SSH-2.0-ws", banner(t, c))
	assert.Equal(t, strings.TrimPrefix(ws.srv.URL, "https://"), seen.Load())
}

// An empty path is /v1/ssh; any other path is kept.
func TestDialWSPath(t *testing.T) {
	ws := startWSGateway(t, false)
	for path, want := range map[string]string{"": "/v1/ssh", "/": "/", "/edge/ssh": "/edge/ssh"} {
		c, err := Dial(t.Context(), Endpoint{WSURL: ws.url(path)})
		require.NoError(t, err, path)
		require.NoError(t, c.Close())
		assert.Equal(t, want, ws.lastPath(), path)
	}
}

// The dial ctx bounds the dial only: the connection works after it ends.
func TestDialWSOutlivesDialContext(t *testing.T) {
	ws := startWSGateway(t, false)
	ctx, cancel := context.WithCancel(t.Context())
	c, err := Dial(ctx, Endpoint{WSURL: ws.url("")})
	require.NoError(t, err)
	defer c.Close() //nolint:errcheck // test conn
	cancel()
	assert.Equal(t, "SSH-2.0-ws", banner(t, c))
	require.NoError(t, c.SetDeadline(time.Now().Add(5*time.Second)))
	_, err = io.WriteString(c, "ping")
	require.NoError(t, err)
	got := make([]byte, 4)
	_, err = io.ReadFull(c, got)
	require.NoError(t, err)
	assert.Equal(t, "ping", string(got))
}

func TestHostKeyAddr(t *testing.T) {
	for ep, want := range map[Endpoint]string{
		{SSHAddress: "gw"}: "gw:2222",
		{SSHAddress: "gw:22", WSURL: "wss://other/v1/ssh"}: "gw:22",
		{SSHAddress: "::1"}:                    "[::1]:2222",
		{WSURL: "wss://gw.example.com/v1/ssh"}: "gw.example.com:443",
		{WSURL: "ws://h:8088"}:                 "h:8088",
		{WSURL: "ws://h"}:                      "h:80",
		{WSURL: "wss://[::1]:8443/x"}:          "[::1]:8443",
	} {
		assert.Equal(t, want, HostKeyAddr(ep), ep)
	}
}
