package session_test

import (
	"bytes"
	"context"
	"crypto/rand"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/platform/db/dbtest"
	"github.com/werbot/werbot/internal/platform/hashchain"
	"github.com/werbot/werbot/internal/platform/secretbox"
	"github.com/werbot/werbot/internal/session"
)

func key(t *testing.T) []byte {
	t.Helper()
	k := make([]byte, 32)
	_, err := rand.Read(k)
	require.NoError(t, err)
	return k
}

func TestManifestCanonical(t *testing.T) {
	id := uuid.MustParse("0b6f3c2e-8f1d-4c3a-9a51-2f7d1e0c4b55")
	got := session.NewManifest(id, 2, 10, hashchain.Genesis()).Canonical()
	assert.Equal(t, `{"bytes":10,"chunks":2,"final_hash":"`+strings.Repeat("0", 64)+`","session_id":"0b6f3c2e-8f1d-4c3a-9a51-2f7d1e0c4b55"}`, string(got))
}

func TestLoadSignerPersists(t *testing.T) {
	ctx := context.Background()
	pool, _ := dbtest.New(t)
	box, err := secretbox.New(key(t), nil)
	require.NoError(t, err)

	s1, err := session.LoadSigner(ctx, pool, box)
	require.NoError(t, err)
	s2, err := session.LoadSigner(ctx, pool, box)
	require.NoError(t, err)
	assert.Equal(t, s1.PublicKey(), s2.PublicKey())

	var ct []byte
	require.NoError(t, pool.QueryRow(ctx, `SELECT ciphertext FROM signing_key`).Scan(&ct))
	var keyID string
	require.NoError(t, pool.QueryRow(ctx, `SELECT key_id FROM signing_key`).Scan(&keyID))
	seed, err := box.Open(ct, keyID)
	require.NoError(t, err)
	assert.Len(t, seed, 32)
	assert.False(t, bytes.Contains(ct, seed), "seed must be sealed, not stored raw")

	m := []byte("manifest")
	sig := s1.Sign(m)
	assert.True(t, s2.Verify(m, sig))
	assert.False(t, s2.Verify(append(m, 'x'), sig))
}

func TestLoadSignerAfterKeyRotation(t *testing.T) {
	ctx := context.Background()
	pool, _ := dbtest.New(t)
	k1, k2 := key(t), key(t)
	b1, err := secretbox.New(k1, nil)
	require.NoError(t, err)
	b2, err := secretbox.New(k2, [][]byte{k1})
	require.NoError(t, err)

	s1, err := session.LoadSigner(ctx, pool, b1)
	require.NoError(t, err)
	s2, err := session.LoadSigner(ctx, pool, b2)
	require.NoError(t, err)
	assert.Equal(t, s1.PublicKey(), s2.PublicKey())
}

func TestLoadSignerConcurrent(t *testing.T) {
	ctx := context.Background()
	pool, _ := dbtest.New(t)
	box, err := secretbox.New(key(t), nil)
	require.NoError(t, err)

	const n = 4
	keys := make([][]byte, n)
	errs := make([]error, n)
	var wg sync.WaitGroup
	for i := range n {
		wg.Go(func() {
			s, err := session.LoadSigner(ctx, pool, box)
			if err == nil {
				keys[i] = s.PublicKey()
			}
			errs[i] = err
		})
	}
	wg.Wait()
	for i := range n {
		require.NoError(t, errs[i])
		assert.Equal(t, keys[0], keys[i])
	}
}

func TestLoadSignerRejectsForeignPublicKey(t *testing.T) {
	ctx := context.Background()
	pool, _ := dbtest.New(t)
	box, err := secretbox.New(key(t), nil)
	require.NoError(t, err)
	_, err = session.LoadSigner(ctx, pool, box)
	require.NoError(t, err)

	_, err = pool.Exec(ctx, `UPDATE signing_key SET public_key = $1`, key(t))
	require.NoError(t, err)
	_, err = session.LoadSigner(ctx, pool, box)
	require.ErrorContains(t, err, "does not match")
}
