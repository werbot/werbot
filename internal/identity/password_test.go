package identity

import (
	"context"
	"encoding/base64"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/platform/apperr"
)

func TestHashPasswordParams(t *testing.T) {
	h, err := hashPassword(context.Background(), "correct horse battery")
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

	ok, err := verifyPassword(context.Background(), h, "correct horse battery")
	require.NoError(t, err)
	assert.True(t, ok)
	ok, err = verifyPassword(context.Background(), h, "wrong horse battery!")
	require.NoError(t, err)
	assert.False(t, ok)

	h2, err := hashPassword(context.Background(), "correct horse battery")
	require.NoError(t, err)
	assert.NotEqual(t, h, h2, "salt must be random")
}

func TestArgonConcurrencyIsCapped(t *testing.T) {
	hash := testPasswordHash()  // before the slots are taken: it hashes too
	for range cap(argonSlots) { // take every slot, as GOMAXPROCS running hashes would
		argonSlots <- struct{}{}
	}
	t.Cleanup(func() {
		for len(argonSlots) > 0 {
			<-argonSlots
		}
	})
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	_, err := hashPassword(ctx, testPassword)
	require.ErrorIs(t, err, context.DeadlineExceeded, "no free slot: the call waits, then gives up with ctx")
	_, err = verifyPassword(ctx, hash, testPassword)
	require.ErrorIs(t, err, context.DeadlineExceeded)

	<-argonSlots // one hash finishes
	ok, err := verifyPassword(context.Background(), hash, testPassword)
	require.NoError(t, err)
	assert.True(t, ok)
}

func TestVerifyPasswordRejectsMalformed(t *testing.T) {
	for _, enc := range []string{"", "plain", "$argon2i$v=19$m=65536,t=3,p=2$c2FsdA$a2V5", "$argon2id$v=19$m=x,t=3,p=2$c2FsdA$a2V5"} {
		_, err := verifyPassword(context.Background(), enc, "pw")
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
