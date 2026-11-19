package app

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
	"github.com/werbot/werbot/gen/werbot/v1/werbotv1connect"
	"github.com/werbot/werbot/internal/identity/oauth"
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

// TestServeMountsConsent: OAuthConsentService is served and needs a subject.
func TestServeMountsConsent(t *testing.T) {
	pool, _ := dbtest.New(t)
	cfg := config.Config{HTTPAddr: freeAddr(t), WebEmbedded: false}
	startServe(t, cfg, pool, nil, nil, slog.New(slog.DiscardHandler))
	cl := werbotv1connect.NewOAuthConsentServiceClient(http.DefaultClient, "http://"+cfg.HTTPAddr)
	_, err := cl.GetAuthRequest(context.Background(), connect.NewRequest(&werbotv1.GetAuthRequestRequest{Id: uuid.NewString()}))
	assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}

// TestServeBearer: a CLI access token from the device flow authenticates
// Connect calls on the served API, and CLISessionService is mounted.
func TestServeBearer(t *testing.T) {
	pool, _ := dbtest.New(t)
	// development: cookie without Secure, which the jar sends over plain http.
	cfg := config.Config{HTTPAddr: freeAddr(t), Env: "development", WebEmbedded: false}
	startServe(t, cfg, pool, nil, nil, slog.New(slog.DiscardHandler))
	base := "http://" + cfg.HTTPAddr
	hc, _ := jarClient(t, http.DefaultTransport)
	ctx := context.Background()
	_, err := werbotv1connect.NewAuthServiceClient(hc, base).Setup(ctx, connect.NewRequest(ownerSetup))
	require.NoError(t, err)

	postForm := func(path string, form url.Values, out any) int {
		res, err := http.PostForm(base+path, form)
		require.NoError(t, err)
		defer func() { _ = res.Body.Close() }()
		require.NoError(t, json.NewDecoder(res.Body).Decode(out))
		return res.StatusCode
	}
	var dr struct {
		DeviceCode string `json:"device_code"`
		UserCode   string `json:"user_code"`
	}
	require.Equal(t, http.StatusOK, postForm("/oauth/device_authorization", url.Values{"client_id": {oauth.ClientCLI}, "scope": {"openid offline_access"}}, &dr))
	_, err = werbotv1connect.NewOAuthConsentServiceClient(hc, base).ApproveDeviceCode(ctx, connect.NewRequest(&werbotv1.ApproveDeviceCodeRequest{UserCode: dr.UserCode}))
	require.NoError(t, err)
	var tr struct {
		AccessToken string `json:"access_token"`
	}
	require.Equal(t, http.StatusOK, postForm("/oauth/token", url.Values{
		"grant_type": {"urn:ietf:params:oauth:grant-type:device_code"}, "client_id": {oauth.ClientCLI}, "device_code": {dr.DeviceCode},
	}, &tr))

	sess, err := werbotv1connect.NewAuthServiceClient(http.DefaultClient, base).GetSession(ctx, withBearer(&werbotv1.GetSessionRequest{}, tr.AccessToken))
	require.NoError(t, err)
	assert.Equal(t, "owner@example.com", sess.Msg.GetUser().GetEmail())
	list, err := werbotv1connect.NewCLISessionServiceClient(http.DefaultClient, base).ListCLISessions(ctx, withBearer(&werbotv1.ListCLISessionsRequest{}, tr.AccessToken))
	require.NoError(t, err)
	require.Len(t, list.Msg.GetSessions(), 1)
	assert.True(t, list.Msg.GetSessions()[0].GetCurrent())

	_, err = werbotv1connect.NewAuthServiceClient(hc, base).GetSession(ctx, withBearer(&werbotv1.GetSessionRequest{}, "garbage"))
	assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err), "a bad token is not rescued by the cookie")
}

func withBearer[T any](msg *T, token string) *connect.Request[T] {
	req := connect.NewRequest(msg)
	req.Header().Set("Authorization", "Bearer "+token)
	return req
}
