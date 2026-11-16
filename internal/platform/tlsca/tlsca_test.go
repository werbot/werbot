package tlsca

import (
	"context"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"io"
	"net"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/werbot/werbot/internal/platform/apperr"
	"github.com/werbot/werbot/internal/platform/db"
	"github.com/werbot/werbot/internal/platform/db/dbtest"
	"github.com/werbot/werbot/internal/platform/secretbox"
)

func testBox(t *testing.T, fill byte) *secretbox.Box {
	t.Helper()
	key := make([]byte, 32)
	for i := range key {
		key[i] = fill
	}
	box, err := secretbox.New(key, nil)
	if err != nil {
		t.Fatal(err)
	}
	return box
}

type memStore struct {
	mu sync.Mutex
	s  *StoredCA
}

func (m *memStore) Load(context.Context) (StoredCA, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.s == nil {
		return StoredCA{}, false, nil
	}
	return *m.s, true, nil
}

func (m *memStore) Insert(_ context.Context, s StoredCA) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.s == nil {
		m.s = &s
	}
	return nil
}

func newTestCA(t *testing.T) *CA {
	t.Helper()
	ca, err := LoadOrCreate(context.Background(), &memStore{}, testBox(t, 1))
	if err != nil {
		t.Fatal(err)
	}
	return ca
}

func ed25519CSR(t *testing.T, cn string) []byte {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	der, err := x509.CreateCertificateRequest(rand.Reader, &x509.CertificateRequest{Subject: pkix.Name{CommonName: cn}}, priv)
	if err != nil {
		t.Fatal(err)
	}
	return der
}

func parseCertPEM(t *testing.T, p []byte) *x509.Certificate {
	t.Helper()
	blk, _ := pem.Decode(p)
	if blk == nil {
		t.Fatal("no PEM block")
	}
	c, err := x509.ParseCertificate(blk.Bytes)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestLoadOrCreateIsStable(t *testing.T) {
	pool, _ := dbtest.New(t)
	ctx := context.Background()
	box := testBox(t, 1)
	store := PostgresStore(pool)
	a, err := LoadOrCreate(ctx, store, box)
	if err != nil {
		t.Fatal(err)
	}
	b, err := LoadOrCreate(ctx, store, box)
	if err != nil {
		t.Fatal(err)
	}
	if a.Fingerprint() != b.Fingerprint() {
		t.Fatalf("fingerprints differ: %s vs %s", a.Fingerprint(), b.Fingerprint())
	}
	if !regexp.MustCompile(`^[0-9a-f]{64}$`).MatchString(a.Fingerprint()) {
		t.Fatalf("bad fingerprint %q", a.Fingerprint())
	}
	fp, err := FingerprintPEM(a.RootPEM())
	if err != nil || fp != a.Fingerprint() {
		t.Fatalf("FingerprintPEM = %q, %v", fp, err)
	}
	if _, err := LoadOrCreate(ctx, store, testBox(t, 2)); err == nil {
		t.Fatal("want error with a different box key")
	}
}

func TestLoadOrCreateUnderBootstrapLock(t *testing.T) {
	pool, _ := dbtest.New(t)
	ctx := context.Background()
	box := testBox(t, 1)
	cas := make([]*CA, 2)
	errs := make([]error, 2)
	var wg sync.WaitGroup
	for i := range cas {
		wg.Go(func() {
			errs[i] = db.WithLock(ctx, pool, db.BootstrapLock, func(ctx context.Context) error {
				var err error
				cas[i], err = LoadOrCreate(ctx, PostgresStore(pool), box)
				return err
			})
		})
	}
	wg.Wait()
	for _, err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	if cas[0].Fingerprint() != cas[1].Fingerprint() {
		t.Fatal("fingerprints differ")
	}
	var n int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM ca_key").Scan(&n); err != nil || n != 1 {
		t.Fatalf("ca_key rows = %d, %v", n, err)
	}
}

func TestIssueClient(t *testing.T) {
	ca := newTestCA(t)
	cn := GatewayCN(uuid.New(), uuid.New())
	certPEM, err := ca.IssueClient(ed25519CSR(t, "evil"), cn, 24*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	cert := parseCertPEM(t, certPEM)
	if cert.Subject.CommonName != cn {
		t.Fatalf("CN = %q", cert.Subject.CommonName)
	}
	if len(cert.ExtKeyUsage) != 1 || cert.ExtKeyUsage[0] != x509.ExtKeyUsageClientAuth {
		t.Fatalf("ExtKeyUsage = %v", cert.ExtKeyUsage)
	}
	if d := time.Until(cert.NotAfter) - 24*time.Hour; d < -time.Minute || d > time.Minute {
		t.Fatalf("NotAfter off by %v", d)
	}
	roots := x509.NewCertPool()
	roots.AppendCertsFromPEM(ca.RootPEM())
	if _, err := cert.Verify(x509.VerifyOptions{Roots: roots, KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}}); err != nil {
		t.Fatal(err)
	}

	ecKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	ecCSR, err := x509.CreateCertificateRequest(rand.Reader, &x509.CertificateRequest{}, ecKey)
	if err != nil {
		t.Fatal(err)
	}
	for name, csr := range map[string][]byte{"ecdsa": ecCSR, "garbage": []byte("garbage")} {
		if _, err := ca.IssueClient(csr, cn, time.Hour); !errors.Is(err, apperr.ErrInvalidArgument) {
			t.Fatalf("%s: err = %v, want ErrInvalidArgument", name, err)
		}
	}
}

func TestServerTLSRequiresValidClientCert(t *testing.T) {
	ca := newTestCA(t)
	cfg, err := ca.ServerTLS([]string{"127.0.0.1"})
	if err != nil {
		t.Fatal(err)
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	srv := &http.Server{TLSConfig: cfg, ReadHeaderTimeout: time.Minute, Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(r.Proto)) //nolint:gosec // test echoes the protocol, no HTML context
	})}
	go func() { _ = srv.ServeTLS(ln, "", "") }()
	t.Cleanup(func() { _ = srv.Close() })

	get := func(cert *tls.Certificate) (string, error) {
		tc := &tls.Config{RootCAs: x509.NewCertPool(), MinVersion: tls.VersionTLS13}
		tc.RootCAs.AppendCertsFromPEM(ca.RootPEM())
		if cert != nil {
			tc.Certificates = []tls.Certificate{*cert}
		}
		cl := &http.Client{Transport: &http.Transport{TLSClientConfig: tc, ForceAttemptHTTP2: true}}
		defer cl.CloseIdleConnections()
		resp, err := cl.Get("https://" + ln.Addr().String())
		if err != nil {
			return "", err
		}
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		return string(b), nil
	}
	clientCert := func(issue func(csr []byte, cn string) ([]byte, error)) *tls.Certificate {
		_, priv, _ := ed25519.GenerateKey(rand.Reader)
		csr, err := x509.CreateCertificateRequest(rand.Reader, &x509.CertificateRequest{}, priv)
		if err != nil {
			t.Fatal(err)
		}
		certPEM, err := issue(csr, GatewayCN(uuid.New(), uuid.New()))
		if err != nil {
			t.Fatal(err)
		}
		blk, _ := pem.Decode(certPEM)
		return &tls.Certificate{Certificate: [][]byte{blk.Bytes}, PrivateKey: priv}
	}

	good := clientCert(func(csr []byte, cn string) ([]byte, error) { return ca.IssueClient(csr, cn, time.Hour) })
	if body, err := get(good); err != nil || body != "HTTP/2.0" {
		t.Fatalf("valid cert: body %q, err %v", body, err)
	}
	if _, err := get(nil); err == nil {
		t.Fatal("no client cert must fail")
	}
	other := newTestCA(t)
	foreign := clientCert(func(csr []byte, cn string) ([]byte, error) { return other.IssueClient(csr, cn, time.Hour) })
	if _, err := get(foreign); err == nil {
		t.Fatal("foreign CA cert must fail")
	}
	now := time.Now()
	expired := clientCert(func(csr []byte, cn string) ([]byte, error) {
		return ca.issue(csr, cn, now.Add(-2*time.Hour), now.Add(-time.Hour))
	})
	if _, err := get(expired); err == nil {
		t.Fatal("expired cert must fail")
	}
}

func TestParseGatewayCN(t *testing.T) {
	gw, inst := uuid.New(), uuid.New()
	g, i, err := ParseGatewayCN(GatewayCN(gw, inst))
	if err != nil || g != gw || i != inst {
		t.Fatalf("round trip: %v %v %v", g, i, err)
	}
	for _, cn := range []string{
		"gateway:" + gw.String(),
		"user:" + gw.String() + ":" + inst.String(),
		"gateway:xyz:" + inst.String(),
		"gateway:{" + gw.String() + "}:" + inst.String(),
		"gateway:" + strings.ToUpper(gw.String()) + ":" + inst.String(),
		"gateway:" + strings.ReplaceAll(gw.String(), "-", "") + ":" + inst.String(),
		"gateway:" + gw.String() + ":" + inst.String() + ":" + gw.String(),
	} {
		if _, _, err := ParseGatewayCN(cn); err == nil {
			t.Errorf("ParseGatewayCN(%q) must fail", cn)
		}
	}
}
