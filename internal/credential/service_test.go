package credential_test

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"

	"github.com/werbot/werbot/internal/connector"
	"github.com/werbot/werbot/internal/credential"
	"github.com/werbot/werbot/internal/platform/apperr"
	"github.com/werbot/werbot/internal/platform/db/dbtest"
	"github.com/werbot/werbot/internal/platform/principal"
	"github.com/werbot/werbot/internal/platform/secretbox"
)

func key(b byte) []byte { return bytes.Repeat([]byte{b}, 32) }

func box(t *testing.T, cur []byte, prev ...[]byte) *secretbox.Box {
	t.Helper()
	b, err := secretbox.New(cur, prev)
	require.NoError(t, err)
	return b
}

type env struct {
	pool *pgxpool.Pool
	ctx  context.Context
	org  uuid.UUID
	res  uuid.UUID
}

func setup(t *testing.T) env {
	t.Helper()
	pool, _ := dbtest.New(t)
	org := dbtest.Org(t, pool)
	owner := dbtest.User(t, pool, org, "owner")
	ctx := principal.With(context.Background(), principal.Principal{UserID: owner, OrgID: org, Role: "owner"})
	return env{pool, ctx, org, dbtest.Resource(t, pool, org, "web-1", nil)}
}

func (e env) keyID(t *testing.T) (id string, ciphertext []byte) {
	t.Helper()
	require.NoError(t, e.pool.QueryRow(context.Background(),
		"SELECT key_id, ciphertext FROM credential WHERE resource_id = $1", e.res).Scan(&id, &ciphertext))
	return id, ciphertext
}

func TestSetReveal(t *testing.T) {
	e := setup(t)
	b := box(t, key(1))
	svc := credential.New(e.pool, b)
	require.NoError(t, svc.Set(e.ctx, e.org, e.res, connector.CredentialPassword, []byte("s3cr3t")))

	got, err := svc.Reveal(e.ctx, e.org, e.res)
	require.NoError(t, err)
	assert.Equal(t, credential.Secret{Kind: connector.CredentialPassword, Value: []byte("s3cr3t")}, got)

	id, ct := e.keyID(t)
	assert.Equal(t, b.KeyID(), id)
	assert.NotContains(t, string(ct), "s3cr3t")

	kinds, err := svc.Kinds(e.ctx, e.org, []uuid.UUID{e.res, uuid.New()})
	require.NoError(t, err)
	assert.Equal(t, map[uuid.UUID]connector.CredentialKind{e.res: connector.CredentialPassword}, kinds)

	require.NoError(t, svc.Clear(e.ctx, e.org, e.res))
	require.NoError(t, svc.Clear(e.ctx, e.org, e.res), "clear without record is nil")
	_, err = svc.Reveal(e.ctx, e.org, e.res)
	assert.ErrorIs(t, err, apperr.ErrNotFound)
}

func TestRotationReseal(t *testing.T) {
	e := setup(t)
	require.NoError(t, credential.New(e.pool, box(t, key(1))).Set(e.ctx, e.org, e.res, connector.CredentialPassword, []byte("pw")))
	svc := credential.New(e.pool, box(t, key(2), key(1)))
	got, err := svc.Reveal(e.ctx, e.org, e.res)
	require.NoError(t, err)
	assert.Equal(t, "pw", string(got.Value))

	require.NoError(t, svc.Set(e.ctx, e.org, e.res, connector.CredentialPassword, []byte("pw2")))
	id, _ := e.keyID(t)
	assert.Equal(t, secretbox.KeyID(key(2)), id)
	got, err = svc.Reveal(e.ctx, e.org, e.res)
	require.NoError(t, err)
	assert.Equal(t, "pw2", string(got.Value))
}

func TestRevealWithRemovedKeyFails(t *testing.T) {
	e := setup(t)
	require.NoError(t, credential.New(e.pool, box(t, key(1))).Set(e.ctx, e.org, e.res, connector.CredentialPassword, []byte("pw")))
	_, err := credential.New(e.pool, box(t, key(2))).Reveal(e.ctx, e.org, e.res)
	assert.ErrorIs(t, err, secretbox.ErrUnknownKey)
}

func TestSetRejects(t *testing.T) {
	e := setup(t)
	svc := credential.New(e.pool, box(t, key(1)))
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	blk, err := ssh.MarshalPrivateKeyWithPassphrase(priv, "", []byte("pass"))
	require.NoError(t, err)
	for name, c := range map[string]struct {
		kind connector.CredentialKind
		val  []byte
	}{
		"empty":            {connector.CredentialPassword, nil},
		"unknown kind":     {"token", []byte("x")},
		"long password":    {connector.CredentialPassword, bytes.Repeat([]byte("a"), 1025)},
		"bad private key":  {connector.CredentialPrivateKey, []byte("not a key")},
		"encrypted key":    {connector.CredentialPrivateKey, pem.EncodeToMemory(blk)},
		"long private key": {connector.CredentialPrivateKey, bytes.Repeat([]byte("a"), 16385)},
	} {
		err := svc.Set(e.ctx, e.org, e.res, c.kind, c.val)
		assert.ErrorIs(t, err, apperr.ErrInvalidArgument, name)
		assert.NotContains(t, err.Error(), "invalid argument: invalid argument", name)
	}
}

func TestSetPrivateKey(t *testing.T) {
	e := setup(t)
	svc := credential.New(e.pool, box(t, key(1)))
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	blk, err := ssh.MarshalPrivateKey(priv, "")
	require.NoError(t, err)
	pemKey := pem.EncodeToMemory(blk)
	require.NoError(t, svc.Set(e.ctx, e.org, e.res, connector.CredentialPrivateKey, pemKey))
	got, err := svc.Reveal(e.ctx, e.org, e.res)
	require.NoError(t, err)
	assert.Equal(t, pemKey, got.Value)
}

func TestCrossOrg(t *testing.T) {
	e := setup(t)
	svc := credential.New(e.pool, box(t, key(1)))
	require.NoError(t, svc.Set(e.ctx, e.org, e.res, connector.CredentialPassword, []byte("pw")))
	orgB := dbtest.Org(t, e.pool)

	assert.ErrorIs(t, svc.Set(e.ctx, orgB, e.res, connector.CredentialPassword, []byte("evil")), apperr.ErrNotFound)
	_, err := svc.Reveal(e.ctx, orgB, e.res)
	assert.ErrorIs(t, err, apperr.ErrNotFound)
	kinds, err := svc.Kinds(e.ctx, orgB, []uuid.UUID{e.res})
	require.NoError(t, err)
	assert.Empty(t, kinds)
	require.NoError(t, svc.Clear(e.ctx, orgB, e.res))

	got, err := svc.Reveal(e.ctx, e.org, e.res)
	require.NoError(t, err)
	assert.Equal(t, "pw", string(got.Value), "org A secret untouched")
}

func TestAuditHasNoSecret(t *testing.T) {
	e := setup(t)
	svc := credential.New(e.pool, box(t, key(1)))
	require.NoError(t, svc.Set(e.ctx, e.org, e.res, connector.CredentialPassword, []byte("s3cr3t")))
	require.NoError(t, svc.Clear(e.ctx, e.org, e.res))
	var n int
	var data string
	require.NoError(t, e.pool.QueryRow(context.Background(),
		"SELECT count(*), coalesce(string_agg(data::text, ' '), '') FROM audit_event WHERE action IN ('credential.set', 'credential.clear') AND data->>'resource_id' = $1 AND data->>'kind' = 'password'",
		e.res.String()).Scan(&n, &data))
	assert.Equal(t, 2, n)
	assert.NotContains(t, data, "s3cr3t")
}
