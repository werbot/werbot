package app

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/platform/config"
	"github.com/werbot/werbot/internal/platform/db/dbtest"
)

func TestBootstrapCreatesSigningKeyOnlyForServer(t *testing.T) {
	pool, url := dbtest.New(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	countKeys := func() int {
		var n int
		require.NoError(t, pool.QueryRow(ctx, "SELECT count(*) FROM oauth_signing_key").Scan(&n))
		return n
	}
	getenv := env(map[string]string{"WERBOT_DATABASE_URL": url, "WERBOT_PUBLIC_URL": testPublicURL, "WERBOT_SECRET_KEY": testSecretKey})
	require.NoError(t, Run(ctx, []string{"migrate"}, getenv, io.Discard))
	assert.Equal(t, 0, countKeys(), "werbot migrate must not create the OAuth signing key")

	httpAddr := freeAddr(t)
	r := startRun(t, []string{"server"}, env(map[string]string{
		"WERBOT_DATABASE_URL": url, "WERBOT_HTTP_ADDR": httpAddr, "WERBOT_PUBLIC_URL": "http://" + httpAddr,
		"WERBOT_SECRET_KEY": testSecretKey, "WERBOT_GATEWAY_ADDR": freeAddr(t), "WERBOT_DATA_DIR": t.TempDir(),
	})).healthy(t)
	assert.Equal(t, 1, countKeys())
	r.stop()
}

// TestServeMountsOAuth: the authorization server is mounted even without the
// embedded UI, and its issuer is WERBOT_PUBLIC_URL.
func TestServeMountsOAuth(t *testing.T) {
	pool, _ := dbtest.New(t)
	cfg := config.Config{HTTPAddr: freeAddr(t), WebEmbedded: false}
	startServe(t, cfg, pool, nil, nil, slog.New(slog.DiscardHandler))
	base := "http://" + cfg.HTTPAddr
	for _, p := range []string{"/.well-known/openid-configuration", "/.well-known/oauth-authorization-server"} {
		res, err := http.Get(base + p)
		require.NoError(t, err)
		var doc map[string]any
		require.NoError(t, json.NewDecoder(res.Body).Decode(&doc), p)
		_ = res.Body.Close()
		assert.Equal(t, http.StatusOK, res.StatusCode, p)
		assert.Contains(t, res.Header.Get("Content-Type"), "application/json", p)
		assert.Equal(t, testPublicURL, doc["issuer"], p)
	}
	code, _ := get(t, base+"/oauth/keys")
	assert.Equal(t, http.StatusOK, code)
}
