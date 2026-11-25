package oauth_test

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/x509"
	"encoding/json"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/identity/oauth"
	"github.com/werbot/werbot/internal/platform/db/dbtest"
)

func TestLoadSigningKeyOnce(t *testing.T) {
	ctx := context.Background()
	pool, _ := dbtest.New(t)
	box := testBox(t)

	const n = 4
	ids := make([]string, n)
	errs := make([]error, n)
	var wg sync.WaitGroup
	for i := range n {
		wg.Go(func() {
			k, err := oauth.LoadSigningKey(ctx, pool, box)
			if err == nil {
				ids[i] = k.ID()
			}
			errs[i] = err
		})
	}
	wg.Wait()
	for i := range n {
		require.NoError(t, errs[i])
		assert.Equal(t, ids[0], ids[i])
	}
	again, err := oauth.LoadSigningKey(ctx, pool, box)
	require.NoError(t, err)
	assert.Equal(t, ids[0], again.ID())
	assert.NotEmpty(t, ids[0])

	var rows int
	require.NoError(t, pool.QueryRow(ctx, "SELECT count(*) FROM oauth_signing_key").Scan(&rows))
	assert.Equal(t, 1, rows)
}

func TestSigningKeySealed(t *testing.T) {
	ctx := context.Background()
	pool, _ := dbtest.New(t)
	box := testBox(t)
	key, err := oauth.LoadSigningKey(ctx, pool, box)
	require.NoError(t, err)
	priv, ok := key.Key().(*ecdsa.PrivateKey)
	require.True(t, ok)
	der, err := x509.MarshalPKCS8PrivateKey(priv)
	require.NoError(t, err)

	var (
		ct      []byte
		keyID   string
		jwkJSON []byte
	)
	require.NoError(t, pool.QueryRow(ctx, "SELECT ciphertext, key_id, public_jwk FROM oauth_signing_key").Scan(&ct, &keyID, &jwkJSON))
	assert.Equal(t, box.KeyID(), keyID)
	assert.False(t, bytes.Contains(ct, der), "private key must be sealed, not stored raw")
	scalar, err := priv.Bytes()
	require.NoError(t, err)
	assert.False(t, bytes.Contains(ct, scalar), "private scalar must not appear in the row")
	plain, err := box.Open(ct, keyID)
	require.NoError(t, err)
	assert.Equal(t, der, plain)

	var jwk map[string]any
	require.NoError(t, json.Unmarshal(jwkJSON, &jwk))
	assert.NotContains(t, jwk, "d")
	assert.Equal(t, "EC", jwk["kty"])
	assert.Equal(t, "P-256", jwk["crv"])
	assert.Equal(t, key.ID(), jwk["kid"])
}
