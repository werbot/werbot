package app

import (
	"context"
	"log/slog"
	"net/http"
	"testing"
	"testing/fstest"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
	"github.com/werbot/werbot/gen/werbot/v1/werbotv1connect"
	"github.com/werbot/werbot/internal/platform/config"
	"github.com/werbot/werbot/internal/platform/db/dbtest"
)

// TestServeOIDCRoutes: the served stack has IdentityProviderService (owner),
// the public AuthService.ListIdentityProviders and the /api/oidc/ routes,
// which refuse a login the IdP cannot finish.
func TestServeOIDCRoutes(t *testing.T) {
	pool, _ := dbtest.New(t)
	cfg := config.Config{HTTPAddr: freeAddr(t), Env: "development", WebEmbedded: false}
	startServe(t, cfg, pool, nil, fstest.MapFS{}, slog.New(slog.DiscardHandler))
	base := "http://" + cfg.HTTPAddr
	hc, _ := jarClient(t, http.DefaultTransport)
	ctx := context.Background()
	_, err := werbotv1connect.NewAuthServiceClient(hc, base).Setup(ctx, connect.NewRequest(ownerSetup))
	require.NoError(t, err)

	idps := werbotv1connect.NewIdentityProviderServiceClient(hc, base)
	res, err := idps.CreateIdentityProvider(ctx, connect.NewRequest(&werbotv1.CreateIdentityProviderRequest{
		Name: "corp", Issuer: "http://" + freeAddr(t), ClientId: "werbot", ClientSecret: "s", Enabled: true}))
	require.NoError(t, err, "development allows an http issuer")
	idp := res.Msg.GetIdentityProvider()
	assert.Equal(t, testPublicURL+"/api/oidc/callback", idp.GetRedirectUri())

	public := werbotv1connect.NewAuthServiceClient(http.DefaultClient, base)
	list, err := public.ListIdentityProviders(ctx, connect.NewRequest(&werbotv1.AuthServiceListIdentityProvidersRequest{}))
	require.NoError(t, err, "public: no session")
	require.Len(t, list.Msg.GetProviders(), 1)
	assert.Equal(t, idp.GetId(), list.Msg.GetProviders()[0].GetId())

	noFollow := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	r, err := noFollow.Get(base + "/api/oidc/" + idp.GetId() + "/start?next=/")
	require.NoError(t, err)
	require.NoError(t, r.Body.Close())
	assert.Equal(t, http.StatusFound, r.StatusCode)
	assert.Equal(t, "/login?error=oidc", r.Header.Get("Location"), "an unreachable IdP refuses the login")
	r, err = noFollow.Get(base + "/api/oidc/callback?code=x&state=y")
	require.NoError(t, err)
	require.NoError(t, r.Body.Close())
	assert.Equal(t, http.StatusBadRequest, r.StatusCode)
}
