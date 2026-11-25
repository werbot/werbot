package gateway

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	gossh "golang.org/x/crypto/ssh"

	"github.com/werbot/werbot/internal/gateway/wsfront"
)

// testXFF is the client address every test WebSocket client sends in X-Forwarded-For.
const testXFF = "203.0.113.7"

var loopback = []netip.Prefix{netip.MustParsePrefix("127.0.0.0/8")}

// serveWS serves f on a WebSocket input of e.p that trusts loopback proxies and returns its address.
func (e *sshEnv) serveWS(t *testing.T, f *sshFront) string {
	t.Helper()
	ln, err := wsfront.Listener("127.0.0.1:0", "", "", loopback)
	require.NoError(t, err)
	return e.serve(t, f, ln)
}

// conf logs in as user with auth, alice's key by default.
func (e *sshEnv) conf(user string, auth ...gossh.AuthMethod) *gossh.ClientConfig {
	if len(auth) == 0 {
		auth = []gossh.AuthMethod{gossh.PublicKeys(e.keys["alice"])}
	}
	return &gossh.ClientConfig{User: user, Auth: auth, HostKeyCallback: gossh.FixedHostKey(e.host.PublicKey()), Timeout: 5 * time.Second}
}

// wsDial opens a WebSocket connection to the input at addr through a proxy that sends testXFF.
func wsDial(t *testing.T, addr string) (net.Conn, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	ws, _, err := websocket.Dial(ctx, "ws://"+addr+wsfront.Path, //nolint:bodyclose // the 101 body belongs to the conn
		&websocket.DialOptions{HTTPHeader: http.Header{"X-Forwarded-For": {testXFF}}})
	if err != nil {
		return nil, err
	}
	t.Cleanup(func() { _ = ws.CloseNow() })
	return websocket.NetConn(context.Background(), ws, websocket.MessageBinary), nil
}

// dialWS logs in with conf over the WebSocket input at addr.
func dialWS(t *testing.T, addr string, conf *gossh.ClientConfig) (*gossh.Client, error) {
	t.Helper()
	nc, err := wsDial(t, addr)
	if err != nil {
		return nil, err
	}
	cc, chans, reqs, err := gossh.NewClientConn(nc, addr, conf)
	if err != nil {
		return nil, err
	}
	c := gossh.NewClient(cc, chans, reqs)
	t.Cleanup(func() { _ = c.Close() })
	return c, nil
}

func TestWSFrontExec(t *testing.T) {
	e := newSSHEnv(t, Config{})
	c, err := dialWS(t, e.serveWS(t, e.front(Config{}, e.host)), e.conf("alice+db"))
	require.NoError(t, err)
	out, code := run(t, c, false, "uptime")
	require.Zero(t, code)
	require.Equal(t, "fake:db user=alice secret=s3cret-value kind=exec\n", out)
	assert.Equal(t, "ws", sessionAgent(e.lastEnd(t, 1)).Transport)
	e.f.with(func(f *fakeControl) {
		require.Len(t, f.starts, 1)
		assert.True(t, strings.HasPrefix(f.starts[0], fmt.Sprintf("%s %s %s", e.m["u1"], e.m["rdb"], testXFF)),
			"StartSession gets the client address of the trusted proxy's X-Forwarded-For: %s", f.starts[0])
	})
}

// The PDP, denials and the menu read the client IP from RemoteAddr, which
// only a *net.TCPAddr carries.
func TestWSRemoteAddrIsTCPAddr(t *testing.T) {
	e := newSSHEnv(t, Config{})
	addr := e.serveWS(t, e.front(Config{}, e.host))
	_, err := dialWS(t, addr, e.conf("alice+db", gossh.PublicKeys(newKey(t))))
	require.ErrorContains(t, err, "unable to authenticate")
	got := e.denialsAfter(t, 1)
	require.Len(t, got, 1)
	assert.Equal(t, "auth_failed", got[0].GetReason())
	assert.Equal(t, testXFF, got[0].GetClientIp())
}

func TestWSHandshakeTimeout(t *testing.T) {
	e := newSSHEnv(t, Config{})
	f := e.front(Config{}, e.host)
	f.handshakeTimeout = 200 * time.Millisecond
	nc, err := wsDial(t, e.serveWS(t, f))
	require.NoError(t, err)
	start := time.Now()
	_, _ = io.Copy(io.Discard, nc) // the server's version line, then the close
	elapsed := time.Since(start)
	assert.GreaterOrEqual(t, elapsed, 150*time.Millisecond, "not closed before the handshake timeout")
	assert.Less(t, elapsed, 2*time.Second, "an upgraded connection that never authenticates is closed")
}

func TestWSFrontMFA(t *testing.T) {
	e, fm := mfaSSH(t)
	fm.verify = func(code string) (bool, error) { return code == "123456", nil }
	ki := &kiClient{answers: []string{"123456"}}
	c, err := dialWS(t, e.serveWS(t, e.front(Config{}, e.host)),
		e.conf("alice+db", gossh.PublicKeys(e.keys["alice"]), gossh.KeyboardInteractive(ki.challenge)))
	require.NoError(t, err)
	out, code := run(t, c, false, "uptime")
	require.Zero(t, code)
	require.Contains(t, out, "fake:db user=alice")
	assert.Equal(t, [][]string{{"", "", testPromptAny}}, ki.rounds, "one keyboard-interactive round after the key")
	creates, codes, _ := fm.calls()
	require.Len(t, creates, 1)
	assert.Equal(t, testXFF, creates[0].ClientIP.String())
	assert.Equal(t, []string{"123456"}, codes)
}

// SSH packets above the WebSocket default read limit of 32 KiB pass both ways.
func TestWSLargePackets(t *testing.T) {
	e := newSSHEnv(t, Config{})
	e.withPortResources(t) // "pg" echoes its stream
	c, err := dialWS(t, e.serveWS(t, e.front(Config{}, e.host)), e.conf("alice+pg"))
	require.NoError(t, err)
	ch, err := c.Dial("tcp", "10.0.0.5:5432")
	require.NoError(t, err)
	defer ch.Close() //nolint:errcheck // test channel

	data := make([]byte, 1<<20)
	_, _ = rand.Read(data)
	go func() { _, _ = ch.Write(data) }()
	got := make([]byte, len(data))
	_, err = io.ReadFull(ch, got)
	require.NoError(t, err)
	require.True(t, bytes.Equal(data, got), "1 MiB comes back unchanged")
}

// wsOn turns on the WebSocket input of newServeEnv, trusting loopback proxies.
func wsOn(_ *fakeEnroll, _ *fakeTunnel, cfg *Config) {
	cfg.WSAddr, cfg.TrustedProxies = "127.0.0.1:0", loopback
}

// wsBlocker is blocker over the WebSocket input; the channel gets the end of the shell.
func (e *serveEnv) wsBlocker(t *testing.T) <-chan error {
	t.Helper()
	c, err := dialWS(t, e.ws.Addr().String(), &gossh.ClientConfig{User: "alice+ready", Auth: []gossh.AuthMethod{gossh.PublicKeys(e.key)},
		HostKeyCallback: gossh.FixedHostKey(e.host), Timeout: 5 * time.Second})
	require.NoError(t, err)
	s, err := c.NewSession()
	require.NoError(t, err)
	out, err := s.StdoutPipe()
	require.NoError(t, err)
	require.NoError(t, s.Shell())
	ready := make(chan string, 1)
	go func() {
		line, _ := bufio.NewReader(out).ReadString('\n')
		ready <- line
	}()
	select {
	case line := <-ready:
		require.Equal(t, "ready\n", line)
	case <-time.After(5 * time.Second):
		t.Fatal("the blocker session did not start")
	}
	done := make(chan error, 1)
	go func() { done <- s.Wait() }()
	return done
}

func TestWSIgnoresProxyProtocol(t *testing.T) {
	e := newServeEnv(t, 50*time.Millisecond, func(enr *fakeEnroll, tun *fakeTunnel, cfg *Config) {
		wsOn(enr, tun, cfg)
		cfg.ProxyProtocol = true
	})
	_, err := gossh.Dial("tcp", e.sshAddr, &gossh.ClientConfig{User: "alice+ready", Auth: []gossh.AuthMethod{gossh.PublicKeys(e.key)},
		HostKeyCallback: gossh.FixedHostKey(e.host), Timeout: 5 * time.Second})
	require.Error(t, err, "the SSH port still requires the PROXY header")
	e.wsBlocker(t) // the WebSocket input takes none
}

func TestWSDrain(t *testing.T) {
	e := newServeEnv(t, 300*time.Millisecond, wsOn)
	done := e.wsBlocker(t)
	addr := e.ws.Addr().String()

	start := time.Now()
	e.cancel()
	require.Eventually(t, func() bool {
		_, err := wsDial(t, addr)
		return err != nil
	}, time.Second, 5*time.Millisecond, "no new upgrade once the drain starts")
	require.Empty(t, e.ends(), "the WebSocket session lives until DrainTimeout")

	require.NoError(t, e.stop())
	require.GreaterOrEqual(t, time.Since(start), 300*time.Millisecond)
	ends := e.ends()
	require.Len(t, ends, 1)
	require.Equal(t, "gateway_shutdown", ends[0].GetReason())
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("the WebSocket session is still open after serve returned")
	}
}

// withWSListener counts the WebSocket listeners Run opens.
func withWSListener(t *testing.T) *int {
	t.Helper()
	var n int
	newWSListener = func(addr, cert, key string, trusted []netip.Prefix) (net.Listener, error) {
		n++
		return wsfront.Listener(addr, cert, key, trusted)
	}
	t.Cleanup(func() { newWSListener = wsfront.Listener })
	return &n
}

func TestWSDisabled(t *testing.T) {
	opened := withWSListener(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // serve stops at enrollment: only the listeners matter
	cfg := Config{SSHAddr: "127.0.0.1:0", DataDir: t.TempDir()}
	require.Error(t, Run(ctx, cfg, slog.New(slog.DiscardHandler)))
	assert.Zero(t, *opened, "no WebSocket listener without WSAddr")

	cfg.WSAddr = "127.0.0.1:0"
	require.Error(t, Run(ctx, cfg, slog.New(slog.DiscardHandler)))
	assert.Equal(t, 1, *opened, "WSAddr opens one")
}

func TestRunWSListenError(t *testing.T) {
	busy := listenLocal(t)
	defer busy.Close() //nolint:errcheck // test listener
	err := Run(t.Context(), Config{SSHAddr: "127.0.0.1:0", WSAddr: busy.Addr().String(), DataDir: t.TempDir()}, slog.New(slog.DiscardHandler))
	require.ErrorContains(t, err, "gateway ws listen: ")
	var op *net.OpError
	require.ErrorAs(t, err, &op, "the embedded gateway stops retrying on a listen error")
	assert.Equal(t, "listen", op.Op)

	err = Run(t.Context(), Config{SSHAddr: "127.0.0.1:0", WSAddr: "127.0.0.1:0", WSTLSCert: "missing.pem", WSTLSKey: "missing.pem",
		DataDir: t.TempDir()}, slog.New(slog.DiscardHandler))
	require.ErrorContains(t, err, "gateway ws listen: ", "an unreadable certificate fails at start")
}
