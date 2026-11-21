package identity

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/rsa"
	"encoding/pem"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"

	"github.com/werbot/werbot/internal/platform/apperr"
	"github.com/werbot/werbot/internal/platform/db/dbtest"
	"github.com/werbot/werbot/internal/platform/principal"
)

// newEd25519 returns a fresh key, its authorized_keys line with comment, and the public key.
func newEd25519(t *testing.T, comment string) (line string, pub ssh.PublicKey) {
	t.Helper()
	k, _, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	pub, err = ssh.NewPublicKey(k)
	require.NoError(t, err)
	line = string(ssh.MarshalAuthorizedKey(pub))
	if comment != "" {
		line = line[:len(line)-1] + " " + comment + "\n"
	}
	return line, pub
}

func TestAddSSHKey(t *testing.T) {
	svc, pool, _ := newTestService(t)
	org := seedOrg(t, pool, "Acme")
	seedUser(t, pool, org, "alice", principal.RoleOwner)
	seedUser(t, pool, org, "bob", principal.RoleMember)
	actx, _ := loginCtx(t, svc, "alice")
	bctx, _ := loginCtx(t, svc, "bob")
	line, pub := newEd25519(t, "alice@laptop")

	k, err := svc.AddSSHKey(actx, "", line)
	require.NoError(t, err)
	assert.Equal(t, "alice@laptop", k.Name)
	assert.Equal(t, ssh.FingerprintSHA256(pub), k.FingerprintSHA256)
	assert.NotContains(t, k.PublicKey, "alice@laptop")
	assert.Equal(t, strings.TrimSpace(string(ssh.MarshalAuthorizedKey(pub))), k.PublicKey)

	long, _ := newEd25519(t, strings.Repeat("c", 200))
	k3, err := svc.AddSSHKey(actx, "", long)
	require.NoError(t, err)
	assert.Equal(t, "key", k3.Name)
	_, err = svc.UpdateSSHKey(actx, k3.ID, "  \t ")
	assert.ErrorIs(t, err, apperr.ErrInvalidArgument)
	_, err = svc.AddSSHKey(actx, strings.Repeat("n", 129), line)
	assert.ErrorIs(t, err, apperr.ErrInvalidArgument)

	_, err = svc.AddSSHKey(bctx, "dup", line)
	assert.ErrorIs(t, err, apperr.ErrAlreadyExists)
	assert.Contains(t, dbtest.AuditActions(t, pool), "ssh_key.add")

	keys, err := svc.ListSSHKeys(actx)
	require.NoError(t, err)
	assert.Contains(t, keys, k)

	k2, err := svc.UpdateSSHKey(actx, k.ID, "work")
	require.NoError(t, err)
	assert.Equal(t, "work", k2.Name)
	require.NoError(t, svc.DeleteSSHKey(actx, k.ID))
	assert.Subset(t, dbtest.AuditActions(t, pool), []string{"ssh_key.update", "ssh_key.delete"})
}

func TestAddSSHKeyRejectsPrivateKey(t *testing.T) {
	svc, pool, _ := newTestService(t)
	seedUser(t, pool, seedOrg(t, pool, "Acme"), "alice", principal.RoleOwner)
	ctx, _ := loginCtx(t, svc, "alice")

	_, priv, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	block, err := ssh.MarshalPrivateKey(priv, "")
	require.NoError(t, err)
	rsaKey, err := rsa.GenerateKey(rand.Reader, 1024) //nolint:gosec // weak on purpose: must be rejected
	require.NoError(t, err)
	rsaPub, err := ssh.NewPublicKey(&rsaKey.PublicKey)
	require.NoError(t, err)
	signer, err := ssh.NewSignerFromKey(rsaKey)
	require.NoError(t, err)
	cert := &ssh.Certificate{Key: rsaPub, CertType: ssh.UserCert, ValidBefore: ssh.CertTimeInfinity}
	require.NoError(t, cert.SignCert(rand.Reader, signer))
	one, _ := newEd25519(t, "")
	two, _ := newEd25519(t, "")

	for name, in := range map[string]string{
		"private":  string(pem.EncodeToMemory(block)),
		"garbage":  "garbage",
		"two keys": one + "\n" + two,
		"rsa 1024": string(ssh.MarshalAuthorizedKey(rsaPub)),
		"cert":     string(ssh.MarshalAuthorizedKey(cert)),
	} {
		_, err := svc.AddSSHKey(ctx, "k", in)
		require.ErrorIs(t, err, apperr.ErrInvalidArgument, name)
		assert.NotContains(t, err.Error(), "BEGIN", name)
		assert.NotContains(t, err.Error(), in, name)
	}
	var n int
	require.NoError(t, pool.QueryRow(context.Background(), "SELECT count(*) FROM user_ssh_key").Scan(&n))
	assert.Zero(t, n)
}

func TestSSHKeysAreUserScoped(t *testing.T) {
	svc, pool, _ := newTestService(t)
	org := seedOrg(t, pool, "Acme")
	seedUser(t, pool, org, "alice", principal.RoleOwner)
	seedUser(t, pool, org, "bob", principal.RoleMember)
	actx, _ := loginCtx(t, svc, "alice")
	bctx, _ := loginCtx(t, svc, "bob")
	line, _ := newEd25519(t, "")
	k, err := svc.AddSSHKey(actx, "mine", line)
	require.NoError(t, err)

	keys, err := svc.ListSSHKeys(bctx)
	require.NoError(t, err)
	assert.Empty(t, keys)
	_, err = svc.UpdateSSHKey(bctx, k.ID, "stolen")
	assert.ErrorIs(t, err, apperr.ErrNotFound)
	assert.ErrorIs(t, svc.DeleteSSHKey(bctx, k.ID), apperr.ErrNotFound)
	assert.ErrorIs(t, svc.DeleteSSHKey(actx, uuid.New()), apperr.ErrNotFound)
}
