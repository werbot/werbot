package secretbox_test

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/platform/secretbox"
)

func key(b byte) []byte { return bytes.Repeat([]byte{b}, 32) }

func TestSecretboxRotation(t *testing.T) {
	old, err := secretbox.New(key(1), nil)
	require.NoError(t, err)
	c, id, err := old.Seal([]byte("pw"))
	require.NoError(t, err)
	assert.Equal(t, secretbox.KeyID(key(1)), id)
	assert.Equal(t, id, old.KeyID())
	rotated, err := secretbox.New(key(2), [][]byte{key(1)})
	require.NoError(t, err)
	p, err := rotated.Open(c, id)
	require.NoError(t, err)
	assert.Equal(t, []byte("pw"), p)
	_, id2, err := rotated.Seal([]byte("pw"))
	require.NoError(t, err)
	assert.Equal(t, secretbox.KeyID(key(2)), id2)
}

func TestSecretboxOpenUnknownKey(t *testing.T) {
	old, _ := secretbox.New(key(1), nil)
	c, _, err := old.Seal([]byte("pw"))
	require.NoError(t, err)
	b, _ := secretbox.New(key(2), nil)
	_, err = b.Open(c, secretbox.KeyID(key(1)))
	assert.ErrorIs(t, err, secretbox.ErrUnknownKey)
}

func TestSecretboxOpenTampered(t *testing.T) {
	b, _ := secretbox.New(key(1), nil)
	c, id, err := b.Seal([]byte("pw"))
	require.NoError(t, err)
	c[len(c)-1] ^= 1
	_, err = b.Open(c, id)
	assert.Error(t, err)
	_, err = b.Open(c[:5], id)
	assert.Error(t, err)
}

func TestSecretboxRejectsBadKeys(t *testing.T) {
	_, err := secretbox.New(make([]byte, 16), nil)
	assert.Error(t, err)
	_, err = secretbox.New(key(1), [][]byte{{1}})
	assert.Error(t, err)
	_, err = secretbox.New(key(1), [][]byte{key(1)}) // duplicates allowed
	assert.NoError(t, err)
}
