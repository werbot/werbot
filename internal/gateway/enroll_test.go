package gateway

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/require"
	gossh "golang.org/x/crypto/ssh"

	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
	"github.com/werbot/werbot/gen/werbot/v1/werbotv1connect"
	"github.com/werbot/werbot/internal/platform/secretbox"
	"github.com/werbot/werbot/internal/platform/tlsca"
	"github.com/werbot/werbot/internal/platform/version"
)

type memCAStore struct {
	mu sync.Mutex
	s  *tlsca.StoredCA
}

func (m *memCAStore) Load(context.Context) (tlsca.StoredCA, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.s == nil {
		return tlsca.StoredCA{}, false, nil
	}
	return *m.s, true, nil
}

func (m *memCAStore) Insert(_ context.Context, s tlsca.StoredCA) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.s == nil {
		m.s = &s
	}
	return nil
}

func newCA(t *testing.T) *tlsca.CA {
	t.Helper()
	box, err := secretbox.New(make([]byte, 32), nil)
	require.NoError(t, err)
	ca, err := tlsca.LoadOrCreate(t.Context(), &memCAStore{}, box)
	require.NoError(t, err)
	return ca
}

type fakeEnroll struct {
	werbotv1connect.UnimplementedGatewayEnrollServiceHandler
	ca      *tlsca.CA
	extraCA []byte // appended to ca_pem
	hostKey []byte
	mu      sync.Mutex
	reqs    []*werbotv1.EnrollRequest
}

func (f *fakeEnroll) Enroll(_ context.Context, r *connect.Request[werbotv1.EnrollRequest]) (*connect.Response[werbotv1.EnrollResponse], error) {
	f.mu.Lock()
	f.reqs = append(f.reqs, r.Msg)
	f.mu.Unlock()
	cert, err := f.ca.IssueClient(r.Msg.GetCsrDer(), "gateway:x:y", time.Hour)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	return connect.NewResponse(&werbotv1.EnrollResponse{
		GatewayId: "0198a000-0000-7000-8000-000000000001", InstanceId: "0198a000-0000-7000-8000-000000000002",
		CertPem: string(cert), CaPem: string(f.ca.RootPEM()) + string(f.extraCA), TunnelUrl: "https://tunnel.test", HostKeyPem: string(f.hostKey),
	}), nil
}

func newFake(t *testing.T) (*fakeEnroll, *httptest.Server) {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	blk, err := gossh.MarshalPrivateKey(priv, "")
	require.NoError(t, err)
	f := &fakeEnroll{ca: newCA(t), hostKey: pem.EncodeToMemory(blk)}
	mux := http.NewServeMux()
	mux.Handle(werbotv1connect.NewGatewayEnrollServiceHandler(f))
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return f, srv
}

func TestEnrollVerifiesCAFingerprint(t *testing.T) {
	f, srv := newFake(t)
	dir := filepath.Join(t.TempDir(), "data")
	cfg := Config{ServerURL: srv.URL, DataDir: dir, Name: "gw-1"}

	cfg.JoinToken = testToken(newCA(t).Fingerprint()) // CA "B"
	_, _, _, err := ensureEnrolled(t.Context(), cfg, srv.Client(), []string{"ssh"})
	require.ErrorContains(t, err, "CA fingerprint mismatch")
	require.NoDirExists(t, dir)

	f.extraCA = newCA(t).RootPEM() // CA A + CA B, token pins A: the appended root must not slip in
	cfg.JoinToken = testToken(f.ca.Fingerprint())
	_, _, _, err = ensureEnrolled(t.Context(), cfg, srv.Client(), nil)
	require.ErrorContains(t, err, "exactly one certificate")
	require.NoDirExists(t, dir)
	f.extraCA = nil
	f.reqs = nil

	cfg.JoinToken = "  " + testToken(f.ca.Fingerprint()) + "\n" // pasted with whitespace
	st, cs, pool, err := ensureEnrolled(t.Context(), cfg, srv.Client(), []string{"ssh", "rdp"})
	require.NoError(t, err)
	require.Equal(t, "https://tunnel.test", st.TunnelURL)
	require.NotNil(t, pool)
	require.NotNil(t, cs.Leaf())

	for _, n := range []string{keyFile, hostKeyFile} {
		fi, err := os.Stat(filepath.Join(dir, n))
		require.NoError(t, err)
		require.Equal(t, os.FileMode(0o600), fi.Mode().Perm(), n)
	}
	fi, err := os.Stat(dir)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o700), fi.Mode().Perm())
	hk, err := os.ReadFile(filepath.Join(dir, hostKeyFile)) //nolint:gosec // path is DataDir + fixed name
	require.NoError(t, err)
	require.Equal(t, f.hostKey, hk)
	require.True(t, Enrolled(dir))

	r := f.reqs[len(f.reqs)-1]
	require.Equal(t, "gw-1", r.GetHostname())
	require.Equal(t, []string{"ssh", "rdp"}, r.GetConnectors())
	csr, err := x509.ParseCertificateRequest(r.GetCsrDer())
	require.NoError(t, err)
	require.Equal(t, x509.Ed25519, csr.PublicKeyAlgorithm)
	require.Len(t, f.reqs, 1)
	require.Equal(t, version.Version, r.GetVersion())
	require.Equal(t, testToken(f.ca.Fingerprint()), r.GetJoinToken()) // trimmed
	var saved state
	raw, err := os.ReadFile(filepath.Join(dir, stateFile)) //nolint:gosec // test path
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(raw, &saved))
	require.Equal(t, st, saved)
	require.Equal(t, "0198a000-0000-7000-8000-000000000001", saved.GatewayID.String())
	require.Equal(t, "0198a000-0000-7000-8000-000000000002", saved.InstanceID.String())
	ca, err := os.ReadFile(filepath.Join(dir, caFile)) //nolint:gosec // test path
	require.NoError(t, err)
	require.Equal(t, f.ca.RootPEM(), ca)
}

func TestEnsureEnrolledSkipsWhenEnrolled(t *testing.T) {
	f, srv := newFake(t)
	dir := t.TempDir()
	cfg := Config{ServerURL: srv.URL, DataDir: dir, Name: "gw", JoinToken: testToken(f.ca.Fingerprint()), TunnelURL: "https://embedded"}
	_, _, _, err := ensureEnrolled(t.Context(), cfg, srv.Client(), nil)
	require.NoError(t, err)
	p := filepath.Join(dir, hostKeyFile)
	before, err := os.Stat(p)
	require.NoError(t, err)
	body, _ := os.ReadFile(p) //nolint:gosec // test path

	// restart: token exhausted/removed, identity wins
	st, cs, _, err := ensureEnrolled(t.Context(), Config{DataDir: dir, TunnelURL: "https://embedded"}, srv.Client(), nil)
	require.NoError(t, err)
	require.Len(t, f.reqs, 1)
	require.Equal(t, "https://embedded", st.TunnelURL)
	require.NotNil(t, cs.Leaf())
	st, _, _, err = ensureEnrolled(t.Context(), Config{DataDir: dir}, srv.Client(), nil)
	require.NoError(t, err)
	require.Equal(t, "https://tunnel.test", st.TunnelURL)
	after, _ := os.Stat(p)
	body2, _ := os.ReadFile(p) //nolint:gosec // test path
	require.Equal(t, body, body2)
	require.Equal(t, before.ModTime(), after.ModTime())
}

func TestEnsureEnrolledRequiresServerAndToken(t *testing.T) {
	_, _, _, err := ensureEnrolled(t.Context(), Config{DataDir: t.TempDir()}, http.DefaultClient, nil)
	require.ErrorContains(t, err, "WERBOT_GATEWAY_SERVER_URL")
}

func TestLoadHostKey(t *testing.T) {
	f, srv := newFake(t)
	dir := t.TempDir()
	_, _, _, err := ensureEnrolled(t.Context(), Config{ServerURL: srv.URL, DataDir: dir, Name: "gw", JoinToken: testToken(f.ca.Fingerprint())}, srv.Client(), nil)
	require.NoError(t, err)
	s, err := loadHostKey(dir)
	require.NoError(t, err)
	require.Equal(t, "ssh-ed25519", s.PublicKey().Type())

	_, err = loadHostKey(t.TempDir())
	require.ErrorContains(t, err, "ssh_host_ed25519_key")
}

func TestRenewAt(t *testing.T) {
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	require.Equal(t, t0.Add(16*time.Hour), renewAt(&x509.Certificate{NotBefore: t0, NotAfter: t0.Add(24 * time.Hour)}))
}

func TestReplaceRejectsMismatchedKey(t *testing.T) {
	f, srv := newFake(t)
	dir := t.TempDir()
	_, cs, _, err := ensureEnrolled(t.Context(), Config{ServerURL: srv.URL, DataDir: dir, Name: "gw", JoinToken: testToken(f.ca.Fingerprint())}, srv.Client(), nil)
	require.NoError(t, err)
	oldCert, _ := os.ReadFile(filepath.Join(dir, certFile)) //nolint:gosec // path is DataDir + fixed name
	otherKey, _, err := newCSR()
	require.NoError(t, err)

	require.Error(t, cs.Replace(oldCert, otherKey))
	got, _ := os.ReadFile(filepath.Join(dir, certFile)) //nolint:gosec // path is DataDir + fixed name
	require.Equal(t, oldCert, got)
}
