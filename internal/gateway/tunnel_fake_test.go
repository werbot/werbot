package gateway

import (
	"bytes"
	"context"
	"crypto/x509"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	gatewayv1 "github.com/werbot/werbot/gen/gateway/v1"
	"github.com/werbot/werbot/gen/gateway/v1/gatewayv1connect"
	"github.com/werbot/werbot/internal/platform/metrics"
	"github.com/werbot/werbot/internal/platform/tlsca"
)

// attempt is what the fake does with Connect attempt n: reject fails it before any
// snapshot; otherwise it sends snap and ends with closeWith (nil: stays open).
type attempt struct {
	reject, closeWith error
	snap              *gatewayv1.Snapshot
}

// fakeTunnel is a TunnelService behind real mTLS HTTP/2 that records the calls.
type fakeTunnel struct {
	gatewayv1connect.UnimplementedTunnelServiceHandler
	ca   *tlsca.CA
	cn   string
	plan func(n int) attempt
	push chan *gatewayv1.Snapshot // sent on the open stream

	mu      sync.Mutex
	starts  []time.Time         // Connect attempts
	closes  []time.Time         // stream ends caused by the plan
	peers   []*x509.Certificate // client certificates of Connect attempts
	hbs     []*gatewayv1.Heartbeat
	pins    []*gatewayv1.PinHostKeyRequest
	pinErr  error
	ends    []*gatewayv1.EndSessionRequest
	csrs    [][]byte
	unaryIP []string
}

func (f *fakeTunnel) Connect(ctx context.Context, st *connect.BidiStream[gatewayv1.ConnectRequest, gatewayv1.ConnectResponse]) error {
	f.mu.Lock()
	n := len(f.starts)
	f.starts = append(f.starts, time.Now())
	f.mu.Unlock()
	a := f.plan(n)
	if a.reject != nil {
		f.closed()
		return a.reject
	}
	go func() {
		for {
			m, err := st.Receive()
			if err != nil {
				return
			}
			f.mu.Lock()
			f.hbs = append(f.hbs, m.GetHeartbeat())
			f.mu.Unlock()
		}
	}()
	send := func(s *gatewayv1.Snapshot) error {
		return st.Send(&gatewayv1.ConnectResponse{Msg: &gatewayv1.ConnectResponse_Snapshot{Snapshot: s}})
	}
	if err := send(a.snap); err != nil {
		return err
	}
	if a.closeWith != nil {
		f.closed()
		return a.closeWith
	}
	for {
		select {
		case s := <-f.push:
			if err := send(s); err != nil {
				return err
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (f *fakeTunnel) closed() {
	f.mu.Lock()
	f.closes = append(f.closes, time.Now())
	f.mu.Unlock()
}

func (f *fakeTunnel) AuthenticateUser(_ context.Context, r *connect.Request[gatewayv1.AuthenticateUserRequest]) (*connect.Response[gatewayv1.AuthenticateUserResponse], error) {
	f.mu.Lock()
	f.unaryIP = append(f.unaryIP, r.Msg.GetClientIp())
	f.mu.Unlock()
	if r.Msg.GetFingerprintSha256() != "SHA256:known" {
		return nil, connect.NewError(connect.CodeUnauthenticated, nil)
	}
	return connect.NewResponse(&gatewayv1.AuthenticateUserResponse{UserId: testUserID.String(), Login: "alice",
		TeamIds: []string{testTeamID.String()}, Country: "NL"}), nil
}

func (f *fakeTunnel) StartSession(ctx context.Context, r *connect.Request[gatewayv1.StartSessionRequest]) (*connect.Response[gatewayv1.StartSessionResponse], error) {
	if r.Msg.GetUserId() == uuid.Nil.String() { // hangs
		<-ctx.Done()
		return nil, ctx.Err()
	}
	return connect.NewResponse(&gatewayv1.StartSessionResponse{SessionId: testSessionID.String(),
		Credential: &gatewayv1.Credential{Kind: "password", Value: []byte("pw")}}), nil
}

func (f *fakeTunnel) EndSession(_ context.Context, r *connect.Request[gatewayv1.EndSessionRequest]) (*connect.Response[gatewayv1.EndSessionResponse], error) {
	f.mu.Lock()
	f.ends = append(f.ends, r.Msg)
	f.mu.Unlock()
	return connect.NewResponse(&gatewayv1.EndSessionResponse{}), nil
}

func (f *fakeTunnel) PinHostKey(_ context.Context, r *connect.Request[gatewayv1.PinHostKeyRequest]) (*connect.Response[gatewayv1.PinHostKeyResponse], error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.pins = append(f.pins, r.Msg)
	if f.pinErr != nil {
		return nil, f.pinErr
	}
	return connect.NewResponse(&gatewayv1.PinHostKeyResponse{}), nil
}

func (f *fakeTunnel) RenewCertificate(_ context.Context, r *connect.Request[gatewayv1.RenewCertificateRequest]) (*connect.Response[gatewayv1.RenewCertificateResponse], error) {
	f.mu.Lock()
	f.csrs = append(f.csrs, r.Msg.GetCsrDer())
	f.mu.Unlock()
	cert, err := f.ca.IssueClient(r.Msg.GetCsrDer(), f.cn, time.Hour)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	return connect.NewResponse(&gatewayv1.RenewCertificateResponse{CertPem: cert}), nil
}

var (
	testUserID    = uuid.MustParse("0198a000-0000-7000-8000-0000000000a1")
	testTeamID    = uuid.MustParse("0198a000-0000-7000-8000-0000000000a2")
	testSessionID = uuid.MustParse("0198a000-0000-7000-8000-0000000000a3")
)

// testSnap is a valid empty snapshot with version v.
func testSnap(v string) *gatewayv1.Snapshot { return &gatewayv1.Snapshot{Version: v} }

// syncBuffer is a log sink safe for concurrent use.
type syncBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *syncBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *syncBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}

// tunnelEnv is a fake tunnel server and an enrolled client with fast test timings.
type tunnelEnv struct {
	f   *fakeTunnel
	c   *tunnelClient
	reg *metrics.Registry
	log *syncBuffer
	dir string
}

// newTunnelEnv serves f over mTLS and builds a client whose certificate lives for certTTL.
func newTunnelEnv(t *testing.T, plan func(n int) attempt, certTTL time.Duration) *tunnelEnv {
	t.Helper()
	ca := newCA(t)
	f := &fakeTunnel{ca: ca, cn: tlsca.GatewayCN(uuid.New(), uuid.New()), plan: plan, push: make(chan *gatewayv1.Snapshot)}
	if f.plan == nil {
		f.plan = func(int) attempt { return attempt{snap: testSnap("v1")} }
	}
	cfg, err := ca.ServerTLS([]string{"127.0.0.1"})
	require.NoError(t, err)
	mux := http.NewServeMux()
	mux.Handle(gatewayv1connect.NewTunnelServiceHandler(f))
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	var protos http.Protocols
	protos.SetHTTP2(true)
	srv := &http.Server{TLSConfig: cfg, Protocols: &protos, ReadHeaderTimeout: 10 * time.Second,
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasSuffix(r.URL.Path, "/Connect") {
				f.mu.Lock()
				f.peers = append(f.peers, r.TLS.PeerCertificates[0])
				f.mu.Unlock()
			}
			mux.ServeHTTP(w, r)
		})}
	go func() { _ = srv.ServeTLS(ln, "", "") }()
	t.Cleanup(func() { _ = srv.Close() })

	dir := t.TempDir()
	keyPEM, csr, err := newCSR()
	require.NoError(t, err)
	certPEM, err := ca.IssueClient(csr, f.cn, certTTL)
	require.NoError(t, err)
	cs := &certStore{dir: dir}
	require.NoError(t, cs.Replace(certPEM, keyPEM))
	roots, _, err := singleCA(ca.RootPEM())
	require.NoError(t, err)

	e := &tunnelEnv{f: f, reg: metrics.NewRegistry(), log: &syncBuffer{}, dir: dir}
	e.c = newTunnelClient(state{TunnelURL: "https://" + ln.Addr().String()}, roots, cs, e.reg, slog.New(slog.NewTextHandler(e.log, nil)))
	e.c.heartbeatEvery, e.c.backoffMin, e.c.callTimeout = 50*time.Millisecond, 10*time.Millisecond, 100*time.Millisecond
	return e
}

// run starts Run until the test ends and checks that it returns nil.
func (e *tunnelEnv) run(t *testing.T, heartbeat func() *gatewayv1.Heartbeat) {
	t.Helper()
	if heartbeat == nil {
		heartbeat = func() *gatewayv1.Heartbeat { return &gatewayv1.Heartbeat{Version: "test"} }
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- e.c.Run(ctx, heartbeat) }()
	t.Cleanup(func() {
		cancel()
		require.NoError(t, <-done)
	})
}

func (e *tunnelEnv) metrics(t *testing.T) string {
	t.Helper()
	rec := httptest.NewRecorder()
	e.reg.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	return rec.Body.String()
}

// with runs fn under the fake's lock.
func (f *fakeTunnel) with(fn func(*fakeTunnel)) {
	f.mu.Lock()
	defer f.mu.Unlock()
	fn(f)
}

// attempts is the number of Connect attempts so far.
func (f *fakeTunnel) attempts() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.starts)
}
