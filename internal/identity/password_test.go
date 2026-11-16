package identity

import (
	"encoding/base64"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/platform/apperr"
)

func TestHashPasswordParams(t *testing.T) {
	h, err := hashPassword("correct horse battery")
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(h, "$argon2id$v=19$m=65536,t=3,p=2$"))
	parts := strings.Split(h, "$")
	require.Len(t, parts, 6)
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	require.NoError(t, err)
	key, err := base64.RawStdEncoding.DecodeString(parts[5])
	require.NoError(t, err)
	assert.Len(t, salt, 16)
	assert.Len(t, key, 32)

	ok, err := verifyPassword(h, "correct horse battery")
	require.NoError(t, err)
	assert.True(t, ok)
	ok, err = verifyPassword(h, "wrong horse battery!")
	require.NoError(t, err)
	assert.False(t, ok)

	h2, err := hashPassword("correct horse battery")
	require.NoError(t, err)
	assert.NotEqual(t, h, h2, "salt must be random")
}

func TestVerifyPasswordRejectsMalformed(t *testing.T) {
	for _, enc := range []string{"", "plain", "$argon2i$v=19$m=65536,t=3,p=2$c2FsdA$a2V5", "$argon2id$v=19$m=x,t=3,p=2$c2FsdA$a2V5"} {
		_, err := verifyPassword(enc, "pw")
		assert.Error(t, err, enc)
	}
}

func TestValidatePassword(t *testing.T) {
	assert.ErrorIs(t, validatePassword(strings.Repeat("a", 11)), apperr.ErrInvalidArgument)
	assert.NoError(t, validatePassword(strings.Repeat("a", 12)))
	assert.NoError(t, validatePassword(strings.Repeat("ж", 12)), "length counts runes, not bytes")
	assert.NoError(t, validatePassword(strings.Repeat("a", 256)))
	assert.ErrorIs(t, validatePassword(strings.Repeat("a", 257)), apperr.ErrInvalidArgument)
}

func TestNewSecret(t *testing.T) {
	raw, hash := newSecret()
	assert.Len(t, raw, 43)
	assert.Equal(t, hashSecret(raw), hash)
	assert.Len(t, hash, 32)
	raw2, _ := newSecret()
	assert.NotEqual(t, raw, raw2)
}

func TestNormalizeEmail(t *testing.T) {
	assert.Equal(t, "alice@example.com", normalizeEmail("  Alice@Example.COM "))
}
