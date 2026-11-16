package tunnel

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
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

	gatewayv1 "github.com/werbot/werbot/gen/gateway/v1"
	"github.com/werbot/werbot/gen/gateway/v1/gatewayv1connect"
	"github.com/werbot/werbot/internal/platform/metrics"
	"github.com/werbot/werbot/internal/platform/secretbox"
	"github.com/werbot/werbot/internal/platform/tlsca"
)

type memCA struct{ s *tlsca.StoredCA }

func (m *memCA) Load(context.Context) (tlsca.StoredCA, bool, error) {
	if m.s == nil {
		return tlsca.StoredCA{}, false, nil
	}
	return *m.s, true, nil
}

func (m *memCA) Insert(_ context.Context, s tlsca.StoredCA) error {
	m.s = &s
	return nil
}

// bench is a real mTLS HTTP/2 tunnel server backed by the fake.
type bench struct {
	ca  *tlsca.CA
	f   *fake
	hub *Hub
	reg *metrics.Registry
	url string
	org uuid.UUID
}

func newBench(t *testing.T, tune func(h *Hub)) *bench {
	t.Helper()
	box, err := secretbox.New(make([]byte, 32), nil)
	if err != nil {
		t.Fatal(err)
	}
	ca, err := tlsca.LoadOrCreate(context.Background(), &memCA{}, box)
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := ca.ServerTLS([]string{"127.0.0.1"})
	if err != nil {
		t.Fatal(err)
	}
	b := &bench{ca: ca, f: newFake(), hub: NewHub(), reg: metrics.NewRegistry(), org: uuid.New()}
	if tune != nil {
		tune(b.hub)
	}
	d := b.f.deps()
	d.Metrics, d.Logger = b.reg, slog.New(slog.DiscardHandler)
	mux := http.NewServeMux()
	New(d, b.hub).Mount(mux)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	var protos http.Protocols
	protos.SetHTTP2(true)
	srv := &http.Server{Handler: mux, TLSConfig: cfg, Protocols: &protos, ReadHeaderTimeout: 10 * time.Second}
	go func() { _ = srv.ServeTLS(ln, "", "") }()
	t.Cleanup(func() {
		b.hub.Shutdown()
		_ = srv.Close()
	})
	b.url = "https://" + ln.Addr().String()
	return b
}

// gw is one enrolled gateway instance with its client certificate.
type gw struct {
	Gateway
	serial string
	client gatewayv1connect.TunnelServiceClient
}

// instance enrolls a new instance of pool in org and returns a client with its certificate.
func (b *bench) instance(t *testing.T, orgID, pool uuid.UUID) *gw {
	t.Helper()
	return b.instanceWith(t, orgID, pool, nil)
}

func (b *bench) instanceWith(t *testing.T, orgID, pool uuid.UUID, wrap func(http.RoundTripper) http.RoundTripper) *gw {
	t.Helper()
	g := Gateway{ID: pool, InstanceID: uuid.New(), OrgID: orgID}
	b.f.with(func(f *fake) { f.instances[g.InstanceID] = g })
	cert, serial := b.clientCert(t, tlsca.GatewayCN(g.ID, g.InstanceID))
	roots := x509.NewCertPool()
	roots.AppendCertsFromPEM(b.ca.RootPEM())
	var protos http.Protocols
	protos.SetHTTP2(true)
	tr := &http.Transport{Protocols: &protos, TLSClientConfig: &tls.Config{RootCAs: roots, Certificates: []tls.Certificate{cert}, MinVersion: tls.VersionTLS13}}
	t.Cleanup(tr.CloseIdleConnections)
	var rt http.RoundTripper = tr
	if wrap != nil {
		rt = wrap(tr)
	}
	return &gw{Gateway: g, serial: serial, client: gatewayv1connect.NewTunnelServiceClient(&http.Client{Transport: rt}, b.url)}
}

func (b *bench) clientCert(t *testing.T, cn string) (tls.Certificate, string) {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	csr, err := x509.CreateCertificateRequest(rand.Reader, &x509.CertificateRequest{}, priv)
	if err != nil {
		t.Fatal(err)
	}
	certPEM, err := b.ca.IssueClient(csr, cn, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	blk, _ := pem.Decode(certPEM)
	leaf, err := x509.ParseCertificate(blk.Bytes)
	if err != nil {
		t.Fatal(err)
	}
	return tls.Certificate{Certificate: [][]byte{blk.Bytes}, PrivateKey: priv}, leaf.SerialNumber.Text(16)
}

func (b *bench) metric(t *testing.T) string {
	t.Helper()
	rec := httptest.NewRecorder()
	b.reg.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	return rec.Body.String()
}

// conn is an open Connect stream read by a background goroutine.
type conn struct {
	st    *connect.BidiStreamForClient[gatewayv1.ConnectRequest, gatewayv1.ConnectResponse]
	snaps chan *gatewayv1.Snapshot
	err   chan error
	once  sync.Once
}

func (g *gw) connect(t *testing.T, hb *gatewayv1.Heartbeat) *conn {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	c := &conn{st: g.client.Connect(ctx), snaps: make(chan *gatewayv1.Snapshot, 64), err: make(chan error, 1)}
	if hb == nil {
		hb = &gatewayv1.Heartbeat{Version: "1.0"}
	}
	if err := c.send(hb); err != nil {
		t.Fatal(err)
	}
	go func() {
		for {
			m, err := c.st.Receive()
			if err != nil {
				c.err <- err
				return
			}
			c.snaps <- m.GetSnapshot()
		}
	}()
	return c
}

func (c *conn) send(hb *gatewayv1.Heartbeat) error {
	return c.st.Send(&gatewayv1.ConnectRequest{Msg: &gatewayv1.ConnectRequest_Heartbeat{Heartbeat: hb}})
}

func (c *conn) close() {
	c.once.Do(func() {
		_ = c.st.CloseRequest()
		_ = c.st.CloseResponse()
	})
}

// next waits for the next snapshot.
func (c *conn) next(t *testing.T, within time.Duration) *gatewayv1.Snapshot {
	t.Helper()
	select {
	case s := <-c.snaps:
		return s
	case err := <-c.err:
		t.Fatalf("stream closed: %v", err)
	case <-time.After(within):
		t.Fatalf("no snapshot within %v", within)
	}
	return nil
}

// quiet asserts that no snapshot arrives for d.
func (c *conn) quiet(t *testing.T, d time.Duration) {
	t.Helper()
	select {
	case s := <-c.snaps:
		t.Fatalf("unexpected snapshot %s", s.GetVersion())
	case err := <-c.err:
		t.Fatalf("stream closed: %v", err)
	case <-time.After(d):
	}
}

// closed waits for the stream to end, skipping snapshots, and returns its Connect error.
func (c *conn) closed(t *testing.T, within time.Duration) *connect.Error {
	t.Helper()
	deadline := time.After(within)
	for {
		select {
		case <-c.snaps:
		case err := <-c.err:
			var ce *connect.Error
			if !errors.As(err, &ce) {
				t.Fatalf("stream ended with %v, want a Connect error", err)
			}
			return ce
		case <-deadline:
			t.Fatalf("stream still open after %v", within)
			return nil
		}
	}
}

func wantCode(t *testing.T, err error, code connect.Code, msg string) {
	t.Helper()
	if connect.CodeOf(err) != code {
		t.Fatalf("err = %v, want code %v", err, code)
	}
	var ce *connect.Error
	if msg != "" && (!errors.As(err, &ce) || ce.Message() != msg) {
		t.Fatalf("err = %v, want message %q", err, msg)
	}
}

func eventually(t *testing.T, within time.Duration, cond func() bool, what string) {
	t.Helper()
	deadline := time.Now().Add(within)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func hasLine(body, line string) bool {
	for l := range strings.Lines(body) {
		if strings.TrimSpace(l) == line {
			return true
		}
	}
	return false
}
