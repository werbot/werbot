package gateway

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	gossh "golang.org/x/crypto/ssh"

	gatewayv1 "github.com/werbot/werbot/gen/gateway/v1"
)

func TestReadyz(t *testing.T) {
	e := newPipeEnv(t, 1)
	newTunnelClient(state{}, nil, nil, e.reg, slog.New(slog.DiscardHandler)) // registers werbot_gateway_tunnel_connected
	h := newHTTPHandler(e.p, e.reg)
	get := func(path string) (int, string) {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		return rec.Code, rec.Body.String()
	}
	pol := e.f.Policy()

	e.f.with(func(f *fakeControl) { f.connected = false })
	code, _ := get("/readyz")
	require.Equal(t, http.StatusServiceUnavailable, code, "not connected")
	e.f.with(func(f *fakeControl) { f.connected, f.policy = true, nil })
	code, _ = get("/readyz")
	require.Equal(t, http.StatusServiceUnavailable, code, "no snapshot")
	e.f.with(func(f *fakeControl) { f.policy = pol })
	code, body := get("/readyz")
	require.Equal(t, http.StatusOK, code)
	require.Equal(t, "ok", body)

	s := e.open(t, "fake", KindExec)
	code, _ = get("/readyz")
	require.Equal(t, http.StatusServiceUnavailable, code, "MaxSessions reached")
	e.p.abort(s)
	code, _ = get("/readyz")
	require.Equal(t, http.StatusOK, code)
	e.p.active.startDrain()
	code, _ = get("/readyz")
	require.Equal(t, http.StatusServiceUnavailable, code, "draining")

	code, body = get("/metrics")
	require.Equal(t, http.StatusOK, code)
	require.Contains(t, body, "werbot_gateway_sessions_active")
	require.Contains(t, body, "werbot_gateway_tunnel_connected")
}

// serveEnv runs serve against a fake enrollment server and a fake tunnel.
type serveEnv struct {
	tun     *fakeTunnel
	sshAddr string
	httpURL string
	key     gossh.Signer
	host    gossh.PublicKey // the pool host key from enrollment
	cancel  context.CancelFunc
	wait    func() error // waits for serve to return and returns its result
	stop    func() error // cancels serve and returns its result
	wantErr error        // what serve returns at cleanup; nil by default
}

// newServeEnv starts serve; opts adjust the fakes before it starts.
func newServeEnv(t *testing.T, drain time.Duration, opts ...func(*fakeEnroll, *fakeTunnel)) *serveEnv {
	t.Helper()
	enr, esrv := newFake(t)
	m := ids{"u1": testUserID, "rblocker": uuid.New()}
	snap := testSnapshot(t, m, `permit (principal == Werbot::User::"u1", action, resource);`, nil, res{"rblocker", "blocker", "blocker"})
	tun, tunnelURL := serveFakeTunnel(t, enr.ca, func(int) attempt { return attempt{snap: snap} })
	for _, o := range opts {
		o(enr, tun)
	}
	host, err := gossh.ParsePrivateKey(enr.hostKey)
	require.NoError(t, err)
	e := &serveEnv{tun: tun, key: newKey(t), host: host.PublicKey()}
	tun.with(func(f *fakeTunnel) { f.fp = fp(e.key) })

	sshLn, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	httpLn, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	e.sshAddr, e.httpURL = sshLn.Addr().String(), "http://"+httpLn.Addr().String()
	cfg := Config{ServerURL: esrv.URL, JoinToken: testToken(enr.ca.Fingerprint()), DataDir: t.TempDir(), Name: "gw",
		TunnelURL: tunnelURL, MaxSessions: 10, DrainTimeout: drain}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- serve(ctx, cfg, sshLn, httpLn, slog.New(slog.DiscardHandler)) }()
	e.cancel = cancel
	e.wait = sync.OnceValue(func() error {
		select {
		case err := <-done:
			return err
		case <-time.After(15 * time.Second):
			return errors.New("serve did not return")
		}
	})
	e.stop = func() error { cancel(); return e.wait() }
	t.Cleanup(func() { require.ErrorIs(t, e.stop(), e.wantErr) }) // errors.Is(nil, nil) holds
	require.Eventually(t, func() bool { return e.readyz() == http.StatusOK }, 5*time.Second, 10*time.Millisecond)
	return e
}

func (e *serveEnv) readyz() int {
	resp, err := http.Get(e.httpURL + "/readyz") //nolint:noctx // test probe
	if err != nil {
		return 0
	}
	_ = resp.Body.Close()
	return resp.StatusCode
}

// blocker opens a shell on the resource "blocker" and waits until the session started.
func (e *serveEnv) blocker(t *testing.T) (*gossh.Session, <-chan error) {
	t.Helper()
	c, err := gossh.Dial("tcp", e.sshAddr, &gossh.ClientConfig{User: "alice+blocker", Auth: []gossh.AuthMethod{gossh.PublicKeys(e.key)},
		HostKeyCallback: gossh.FixedHostKey(e.host), Timeout: 5 * time.Second})
	require.NoError(t, err)
	t.Cleanup(func() { _ = c.Close() })
	s, err := c.NewSession()
	require.NoError(t, err)
	require.NoError(t, s.Shell())
	require.Eventually(t, func() bool {
		var n int
		e.tun.with(func(f *fakeTunnel) { n = f.startCalls })
		return n == 1
	}, 5*time.Second, 10*time.Millisecond)
	done := make(chan error, 1)
	go func() { done <- s.Wait() }()
	return s, done
}

func (e *serveEnv) ends() []*gatewayv1.EndSessionRequest {
	var out []*gatewayv1.EndSessionRequest
	e.tun.with(func(f *fakeTunnel) { out = append(out, f.ends...) })
	return out
}

// requireAlive checks that the session channel still answers and no EndSession was sent.
func (e *serveEnv) requireAlive(t *testing.T, s *gossh.Session, done <-chan error) {
	t.Helper()
	select {
	case err := <-done:
		t.Fatalf("session ended: %v", err)
	default:
	}
	_, err := s.SendRequest("ping@werbot", true, nil)
	require.NoError(t, err, "the channel is alive")
	require.Empty(t, e.ends())
}

func TestStreamCloseKeepsSessions(t *testing.T) {
	e := newServeEnv(t, time.Second)
	s, done := e.blocker(t)

	n := e.tun.attempts()
	e.tun.kill <- connect.NewError(connect.CodeUnavailable, errors.New("rebalance"))
	require.Eventually(t, func() bool { return e.tun.attempts() == n+1 }, 500*time.Millisecond, 5*time.Millisecond)
	e.requireAlive(t, s, done)

	e.tun.kill <- connect.NewError(connect.CodeInternal, errors.New("boom"))
	time.Sleep(100 * time.Millisecond)
	e.requireAlive(t, s, done)
}

func TestServeDrain(t *testing.T) {
	e := newServeEnv(t, 300*time.Millisecond)
	_, done := e.blocker(t)

	start := time.Now()
	e.cancel()
	var readyAtClose int
	require.Eventually(t, func() bool {
		c, err := net.Dial("tcp", e.sshAddr)
		if err == nil {
			_ = c.Close()
			return false
		}
		readyAtClose = e.readyz()
		return true
	}, 100*time.Millisecond, time.Millisecond, "the SSH listener is closed")
	require.Equal(t, http.StatusServiceUnavailable, readyAtClose, "/readyz turns 503 before the listener closes")
	require.Empty(t, e.ends(), "the session lives until DrainTimeout")

	require.NoError(t, e.stop())
	require.GreaterOrEqual(t, time.Since(start), 300*time.Millisecond)
	ends := e.ends()
	require.Len(t, ends, 1)
	require.Equal(t, testSessionID.String(), ends[0].GetSessionId())
	require.Equal(t, "gateway_shutdown", ends[0].GetReason())
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("the SSH session is still open after serve returned")
	}
}

// A certificate that expired while serving ends serve after the drain: the
// process exits (standalone) or enrolls again (embedded) instead of retrying forever.
func TestServeReturnsWhenCertExpires(t *testing.T) {
	const ttl = 2 * time.Second
	e := newServeEnv(t, 50*time.Millisecond, func(enr *fakeEnroll, tun *fakeTunnel) {
		enr.ttl = ttl
		tun.renewErr = connect.NewError(connect.CodeUnavailable, errors.New("down"))
	})
	e.wantErr = ErrCertExpired
	time.Sleep(ttl) // the open stream outlives the certificate; its end reveals the expiry
	e.tun.kill <- connect.NewError(connect.CodeInternal, errors.New("boom"))

	done := make(chan error, 1)
	go func() { done <- e.wait() }()
	select {
	case err := <-done:
		require.ErrorIs(t, err, ErrCertExpired)
	case <-time.After(3 * time.Second):
		t.Fatal("serve did not return after the certificate expired")
	}
	_, err := net.Dial("tcp", e.sshAddr)
	require.Error(t, err, "the SSH listener is closed")
}
