package jwt_test

import (
	"context"
	"os"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	profilemessage "github.com/werbot/werbot/internal/core/profile/proto/message"
	"github.com/werbot/werbot/internal/web/jwt"
	"github.com/werbot/werbot/pkg/storage/redis"
)

func TestMain(m *testing.M) {
	os.Setenv("JWT_PUBLIC_KEY", "../../../fixtures/keys/jwt/jwt_public.key")
	os.Setenv("JWT_PRIVATE_KEY", "../../../fixtures/keys/jwt/jwt_private.key")
	os.Exit(m.Run())
}

func newTokenConfig(t *testing.T) *jwt.Config {
	t.Helper()

	config, err := jwt.New(&profilemessage.ProfileParameters{
		Name:      "name",
		ProfileId: "66666666-6666-6666-6666-666666666666",
		SessionId: "3e2e1d9a-2b3a-4c39-8c4f-7b7b6c1e2f3a",
	})
	require.NoError(t, err)

	return config
}

func TestPairTokensHaveTokenType(t *testing.T) {
	pair, err := newTokenConfig(t).PairTokens()
	require.NoError(t, err)

	accessClaims, err := jwt.Parse(pair.GetAccess())
	require.NoError(t, err)
	assert.Equal(t, jwt.TokenTypeAccess, jwt.TokenType(accessClaims))

	refreshClaims, err := jwt.Parse(pair.GetRefresh())
	require.NoError(t, err)
	assert.Equal(t, jwt.TokenTypeRefresh, jwt.TokenType(refreshClaims))
}

func TestTokenTypeMissingClaim(t *testing.T) {
	assert.Empty(t, jwt.TokenType(nil))
	assert.Empty(t, jwt.TokenType(map[string]any{}))
	assert.Empty(t, jwt.TokenType(map[string]any{"typ": 42}))
}

func newTestRedis(t *testing.T) (*redis.Connect, *miniredis.Miniredis) {
	t.Helper()

	server := miniredis.RunT(t)
	return redis.New(context.Background(), &redis.Config{Addr: server.Addr()}), server
}

func TestCacheLifecycle(t *testing.T) {
	cache, _ := newTestRedis(t)
	const sessionID = "session-id"
	want := jwt.SessionInfo{ProfileID: "profile-id"}

	require.NoError(t, jwt.CacheAdd(cache, "refresh_token", sessionID, want))

	got, err := jwt.CacheGet(cache, "refresh_token", sessionID)
	require.NoError(t, err)
	assert.Equal(t, want, *got)

	assert.True(t, jwt.CacheDelete(cache, "refresh_token", sessionID))
	assert.False(t, jwt.CacheDelete(cache, "refresh_token", sessionID))

	_, err = jwt.CacheGet(cache, "refresh_token", sessionID)
	assert.Error(t, err)
}

func TestCacheOperationsReturnErrors(t *testing.T) {
	cache, server := newTestRedis(t)
	server.SetError("cache is unavailable")

	assert.Error(t, jwt.CacheAdd(cache, "access_token", "session-id", jwt.SessionInfo{ProfileID: "profile-id"}))

	_, err := jwt.CacheGet(cache, "access_token", "session-id")
	assert.Error(t, err)

	assert.False(t, jwt.CacheDelete(cache, "access_token", "session-id"))
}
