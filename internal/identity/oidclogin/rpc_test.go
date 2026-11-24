package oidclogin

import (
	"context"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
	"github.com/werbot/werbot/internal/platform/db/dbtest"
	"github.com/werbot/werbot/internal/platform/principal"
)

func TestIdentityProviderServiceCRUD(t *testing.T) {
	s := newStand(t)
	cfg := s.cfg
	cfg.SecureCookies = true // production: https issuers only
	h := NewIdentityProviderHandler(s.store, cfg)
	admin := principal.With(context.Background(), s.admin)
	member := principal.With(context.Background(), principal.Principal{OrgID: s.org,
		UserID: dbtest.User(t, s.pool, s.org, principal.RoleMember), Role: principal.RoleMember})
	team := dbtest.Team(t, s.pool, s.org)
	create := &werbotv1.CreateIdentityProviderRequest{Name: "okta", Issuer: "https://okta.test", ClientId: "c1", ClientSecret: "s3cret-1",
		ClaimsMapping: []*werbotv1.ClaimMapping{{Claim: "groups", Regex: "ops", Target: &werbotv1.ClaimMapping_TeamId{TeamId: team.String()}}},
		Enabled:       true}

	_, err := h.CreateIdentityProvider(member, connect.NewRequest(create))
	assert.Equal(t, connect.CodePermissionDenied, connect.CodeOf(err))
	_, err = h.ListIdentityProviders(member, connect.NewRequest(&werbotv1.ListIdentityProvidersRequest{}))
	assert.Equal(t, connect.CodePermissionDenied, connect.CodeOf(err))

	res, err := h.CreateIdentityProvider(admin, connect.NewRequest(create))
	require.NoError(t, err)
	got := res.Msg.GetIdentityProvider()
	assert.Empty(t, got.GetClientSecret(), "the secret is never returned")
	assert.Equal(t, s.srv.URL+"/api/oidc/callback", got.GetRedirectUri())
	assert.Equal(t, DefaultScopes, got.GetScopes())
	assert.Equal(t, team.String(), got.GetClaimsMapping()[0].GetTeamId())
	id := got.GetId()

	_, err = h.CreateIdentityProvider(admin, connect.NewRequest(create))
	assert.Equal(t, connect.CodeAlreadyExists, connect.CodeOf(err), "same name")

	update := &werbotv1.UpdateIdentityProviderRequest{Id: id, Name: "okta", Issuer: "https://okta.test", ClientId: "c2",
		ClaimsMapping: []*werbotv1.ClaimMapping{{Claim: "groups", Regex: "admins", Target: &werbotv1.ClaimMapping_Role{Role: "admin"}}}}
	upd, err := h.UpdateIdentityProvider(admin, connect.NewRequest(update))
	require.NoError(t, err)
	assert.Equal(t, "c2", upd.Msg.GetIdentityProvider().GetClientId())
	assert.False(t, upd.Msg.GetIdentityProvider().GetEnabled())
	stored, err := s.store.Get(context.Background(), uuid.MustParse(id))
	require.NoError(t, err)
	assert.Equal(t, "s3cret-1", stored.ClientSecret, "an empty secret keeps the stored one")

	got2, err := h.GetIdentityProvider(admin, connect.NewRequest(&werbotv1.GetIdentityProviderRequest{Id: id}))
	require.NoError(t, err)
	assert.Equal(t, "admin", got2.Msg.GetIdentityProvider().GetClaimsMapping()[0].GetRole())
	list, err := h.ListIdentityProviders(admin, connect.NewRequest(&werbotv1.ListIdentityProvidersRequest{}))
	require.NoError(t, err)
	assert.Len(t, list.Msg.GetIdentityProviders(), 2, "corp of the stand and okta")

	otherTeam := dbtest.Team(t, s.pool, dbtest.Org(t, s.pool))
	for name, mutate := range map[string]func(*werbotv1.CreateIdentityProviderRequest){
		"team of another org": func(r *werbotv1.CreateIdentityProviderRequest) {
			r.ClaimsMapping = []*werbotv1.ClaimMapping{{Claim: "g", Regex: "x", Target: &werbotv1.ClaimMapping_TeamId{TeamId: otherTeam.String()}}}
		},
		"role owner": func(r *werbotv1.CreateIdentityProviderRequest) {
			r.ClaimsMapping = []*werbotv1.ClaimMapping{{Claim: "g", Regex: "x", Target: &werbotv1.ClaimMapping_Role{Role: "owner"}}}
		},
		"no target": func(r *werbotv1.CreateIdentityProviderRequest) {
			r.ClaimsMapping = []*werbotv1.ClaimMapping{{Claim: "g", Regex: "x"}}
		},
		"bad regex":         func(r *werbotv1.CreateIdentityProviderRequest) { r.ClaimsMapping[0].Regex = "(" },
		"http issuer":       func(r *werbotv1.CreateIdentityProviderRequest) { r.Issuer = "http://x" },
		"issuer with query": func(r *werbotv1.CreateIdentityProviderRequest) { r.Issuer = "https://x?a=1" },
		"no openid scope":   func(r *werbotv1.CreateIdentityProviderRequest) { r.Scopes = []string{"email"} },
		"no secret":         func(r *werbotv1.CreateIdentityProviderRequest) { r.ClientSecret = "" },
	} {
		r := &werbotv1.CreateIdentityProviderRequest{Name: "x-" + strings.ReplaceAll(name, " ", "-"), Issuer: "https://x.test", ClientId: "c",
			ClientSecret: "s", ClaimsMapping: []*werbotv1.ClaimMapping{{Claim: "g", Regex: "x", Target: &werbotv1.ClaimMapping_Role{Role: "member"}}}}
		mutate(r)
		_, err := h.CreateIdentityProvider(admin, connect.NewRequest(r))
		assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err), name)
	}

	_, err = h.DeleteIdentityProvider(admin, connect.NewRequest(&werbotv1.DeleteIdentityProviderRequest{Id: id}))
	require.NoError(t, err)
	_, err = h.GetIdentityProvider(admin, connect.NewRequest(&werbotv1.GetIdentityProviderRequest{Id: id}))
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))

	var actions []string
	var data string
	require.NoError(t, s.pool.QueryRow(context.Background(), `SELECT array_agg(action ORDER BY created_at, action), string_agg(data::text, ' ')
		FROM audit_event WHERE action LIKE 'idp.%' AND target_id = $1`, id).Scan(&actions, &data))
	assert.Equal(t, []string{"idp.create", "idp.update", "idp.delete"}, actions)
	assert.NotContains(t, data, "s3cret", "no secret in the audit")
}

func TestStoreLinks(t *testing.T) {
	s := newStand(t)
	ctx := context.Background()
	s.bob()
	s.op.SetUser("sub-bob", map[string]any{"email": "bob@example.com", "email_verified": true})
	_, ok := s.session(s.login("/", nil))
	require.True(t, ok)

	links, err := s.store.StaleLinks(ctx, 0, 10)
	require.NoError(t, err)
	require.Len(t, links, 1)
	l := links[0]
	assert.Equal(t, "sub-bob", l.Subject)
	assert.NotEmpty(t, l.Refresh, "the refresh token is opened")
	links, err = s.store.StaleLinks(ctx, time.Hour, 10)
	require.NoError(t, err)
	assert.Empty(t, links, "checked just now")

	rotated := "rotated-token"
	require.NoError(t, s.store.UpdateLink(ctx, l, &rotated, time.Now().Add(-2*time.Hour)))
	links, err = s.store.StaleLinks(ctx, time.Hour, 10)
	require.NoError(t, err)
	require.Len(t, links, 1)
	assert.Equal(t, rotated, links[0].Refresh)

	require.NoError(t, s.store.UpdateLink(ctx, l, nil, time.Now().Add(-2*time.Hour)))
	links, err = s.store.StaleLinks(ctx, time.Hour, 10)
	require.NoError(t, err)
	assert.Equal(t, rotated, links[0].Refresh, "nil keeps the token")

	empty := ""
	require.NoError(t, s.store.UpdateLink(ctx, l, &empty, time.Now().Add(-2*time.Hour)))
	links, err = s.store.StaleLinks(ctx, time.Hour, 10)
	require.NoError(t, err)
	assert.Empty(t, links, "an empty token is dropped: nothing to recheck")
}
