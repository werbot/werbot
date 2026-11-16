package gateway

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// writeExpiredCert replaces the client certificate in dir with one that expired an hour ago.
func writeExpiredCert(t *testing.T, dir string) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	tmpl := &x509.Certificate{SerialNumber: big.NewInt(1), NotBefore: time.Now().Add(-48 * time.Hour), NotAfter: time.Now().Add(-time.Hour)}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, pub, priv)
	require.NoError(t, err)
	key, err := x509.MarshalPKCS8PrivateKey(priv)
	require.NoError(t, err)
	require.NoError(t, (&certStore{dir: dir}).Replace(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}),
		pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: key})))
}

func TestEnsureEnrolledReenrollsStaleIdentity(t *testing.T) {
	f, srv := newFake(t)
	dir := t.TempDir()
	cfg := Config{ServerURL: srv.URL, DataDir: dir, Name: "gw", JoinToken: testToken(f.ca.Fingerprint())}
	_, _, _, err := ensureEnrolled(t.Context(), cfg, srv.Client(), nil)
	require.NoError(t, err)
	writeExpiredCert(t, dir)
	require.False(t, Enrolled(dir), "expired certificate")

	f.spent = true // a standalone gateway restarted with its old token
	_, _, _, err = ensureEnrolled(t.Context(), cfg, srv.Client(), nil)
	require.ErrorContains(t, err, "expired")
	require.ErrorContains(t, err, "new join token")
	require.ErrorContains(t, err, "invalid join token")

	f.spent = false
	_, cs, _, err := ensureEnrolled(t.Context(), cfg, srv.Client(), nil)
	require.NoError(t, err)
	require.Len(t, f.reqs, 3, "Enroll is called for the stale identity")
	require.True(t, cs.Leaf().NotAfter.After(time.Now()))
	require.True(t, Enrolled(dir))

	require.NoError(t, os.WriteFile(filepath.Join(dir, certFile), []byte("garbage"), 0o600))
	require.False(t, Enrolled(dir), "unloadable certificate")
}

func TestStaleIdentityErrorNamesTokenOnlyWhenRejected(t *testing.T) {
	f, srv := newFake(t)
	dir := t.TempDir()
	cfg := Config{ServerURL: srv.URL, DataDir: dir, Name: "gw", JoinToken: testToken(f.ca.Fingerprint())}
	_, _, _, err := ensureEnrolled(t.Context(), cfg, srv.Client(), nil)
	require.NoError(t, err)
	writeExpiredCert(t, dir)

	down := cfg
	down.ServerURL = "http://127.0.0.1:1" // nothing listens: an outage, not a refusal
	_, _, _, err = ensureEnrolled(t.Context(), down, srv.Client(), nil)
	require.ErrorContains(t, err, "saved identity is unusable")
	require.ErrorContains(t, err, "expired")
	require.NotContains(t, err.Error(), "new join token")

	f.spent = true
	_, _, _, err = ensureEnrolled(t.Context(), cfg, srv.Client(), nil)
	require.ErrorContains(t, err, "new join token")
}

// A gateway.json that does not parse or lacks the ids is a stale identity: the
// instance enrolls again instead of failing every start.
func TestCorruptStateReenrolls(t *testing.T) {
	f, srv := newFake(t)
	dir := t.TempDir()
	cfg := Config{ServerURL: srv.URL, DataDir: dir, Name: "gw", JoinToken: testToken(f.ca.Fingerprint())}
	_, _, _, err := ensureEnrolled(t.Context(), cfg, srv.Client(), nil)
	require.NoError(t, err)
	for _, bad := range []string{"{", "{}"} {
		require.NoError(t, os.WriteFile(filepath.Join(dir, stateFile), []byte(bad), 0o600))
		require.False(t, Enrolled(dir), bad)
	}

	st, cs, _, err := ensureEnrolled(t.Context(), cfg, srv.Client(), nil)
	require.NoError(t, err)
	require.Len(t, f.reqs, 2, "Enroll is called once for the corrupt identity")
	require.Equal(t, "0198a000-0000-7000-8000-000000000002", st.InstanceID.String())
	require.NotNil(t, cs.Leaf())
	require.True(t, Enrolled(dir))
}
