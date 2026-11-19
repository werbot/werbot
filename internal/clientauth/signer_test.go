package clientauth

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"

	"github.com/werbot/werbot/internal/platform/sshca"
)

func signerCert(t *testing.T, s ssh.Signer) *ssh.Certificate {
	t.Helper()
	cert, ok := s.PublicKey().(*ssh.Certificate)
	require.True(t, ok)
	return cert
}

func readFile(t *testing.T, path string) []byte {
	t.Helper()
	raw, err := os.ReadFile(path) //nolint:gosec // test temp directory
	require.NoError(t, err)
	return raw
}

// plantCert writes a certificate with the given remaining validity for the profile key.
func plantCert(t *testing.T, as *fakeAS, dir string, remaining time.Duration) *ssh.Certificate {
	t.Helper()
	key, err := ssh.ParsePrivateKey(readFile(t, filepath.Join(dir, "id_ed25519")))
	require.NoError(t, err)
	cert, err := as.sign(key.PublicKey(), time.Now().Add(remaining-sshca.UserCertTTL), sshca.UserCertTTL)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "id_ed25519-cert.pub"), ssh.MarshalAuthorizedKey(cert), 0o600))
	return cert
}

func TestSignerIssuesAndRenews(t *testing.T) {
	as := newFakeAS(t)
	home, p := login(t, as)
	dir := Dir(home, p.Server)
	const issue = "/werbot.v1.CertService/IssueUserCert"

	s, err := p.Signer(context.Background(), home)
	require.NoError(t, err)
	assert.Equal(t, 1, as.count(issue))
	cert := signerCert(t, s)
	assert.Equal(t, []string{"alice"}, cert.ValidPrincipals)
	assertMode(t, filepath.Join(dir, "id_ed25519"), 0o600)
	assertMode(t, filepath.Join(dir, "id_ed25519-cert.pub"), 0o600)

	planted := plantCert(t, as, dir, 30*time.Minute)
	s, err = p.Signer(context.Background(), home)
	require.NoError(t, err)
	assert.Equal(t, 1, as.count(issue))
	assert.Equal(t, planted.Serial, signerCert(t, s).Serial)

	planted = plantCert(t, as, dir, 9*time.Minute)
	s, err = p.Signer(context.Background(), home)
	require.NoError(t, err)
	assert.Equal(t, 2, as.count(issue))
	assert.NotEqual(t, planted.Serial, signerCert(t, s).Serial)
	assert.Equal(t, ssh.MarshalAuthorizedKey(signerCert(t, s)), readFile(t, filepath.Join(dir, "id_ed25519-cert.pub")))
}

func TestSignerKeepsValidCertWhenRenewFails(t *testing.T) {
	as := newFakeAS(t)
	home, p := login(t, as)
	dir := Dir(home, p.Server)
	_, err := p.Signer(context.Background(), home)
	require.NoError(t, err)
	as.certFail = true

	planted := plantCert(t, as, dir, 5*time.Minute)
	s, err := p.Signer(context.Background(), home)
	require.NoError(t, err)
	assert.Equal(t, planted.Serial, signerCert(t, s).Serial)

	plantCert(t, as, dir, -time.Minute)
	_, err = p.Signer(context.Background(), home)
	require.Error(t, err)
}

func TestSignerClockSkew(t *testing.T) {
	as := newFakeAS(t)
	home, p := login(t, as)
	dir := Dir(home, p.Server)
	_, err := p.Signer(context.Background(), home)
	require.NoError(t, err)
	plantCert(t, as, dir, 5*time.Minute)
	before := readFile(t, filepath.Join(dir, "id_ed25519-cert.pub"))

	as.certSkew = 2 * time.Hour // valid_before = now + 3h
	_, err = p.Signer(context.Background(), home)
	require.ErrorContains(t, err, "clock skew 2h0m0s between this computer and "+p.Server+"; sync the system clock")
	assert.Equal(t, before, readFile(t, filepath.Join(dir, "id_ed25519-cert.pub")))
}
