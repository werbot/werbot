package oidclogin

import (
	"bytes"
	"context"
	"net/http"
	"net/url"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/platform/db/dbtest"
	"github.com/werbot/werbot/internal/platform/principal"
)

func TestOIDCLoginAppliesMapping(t *testing.T) {
	s := newStand(t)
	ctx := context.Background()
	bob := s.bob()
	ops := dbtest.Team(t, s.pool, s.org)
	s.idp.Mapping = []MappingRule{{Claim: "groups", Regex: "ops", TeamID: &ops}, {Claim: "groups", Regex: "ops", Role: "admin"}}
	_, err := s.store.Update(ctx, s.admin, s.idp)
	require.NoError(t, err)
	s.op.SetUser("sub-bob", map[string]any{"email": "bob@example.com", "email_verified": true, "groups": []string{"ops"}})

	res := s.login("/oauth/authorize?x=1", nil)
	require.Equal(t, http.StatusFound, res.StatusCode)
	assert.Equal(t, "/oauth/authorize?x=1", res.Location)
	p, ok := s.session(res)
	require.True(t, ok)
	assert.Equal(t, bob, p.UserID)
	assert.Equal(t, principal.RoleAdmin, p.Role)
	teams, err := s.svc.UserTeams(ctx, s.org, bob)
	require.NoError(t, err)
	assert.Equal(t, []uuid.UUID{ops}, teams)
}

func TestRefreshTokenSealed(t *testing.T) {
	s := newStand(t)
	s.bob()
	s.op.SetUser("sub-bob", map[string]any{"email": "bob@example.com", "email_verified": true})
	_, ok := s.session(s.login("/", nil))
	require.True(t, ok)

	var sealed []byte
	var keyID string
	require.NoError(t, s.pool.QueryRow(context.Background(),
		"SELECT refresh_ciphertext, key_id FROM user_idp_link WHERE subject = 'sub-bob'").Scan(&sealed, &keyID))
	assert.Equal(t, s.box.KeyID(), keyID)
	refresh, err := s.box.Open(sealed, keyID)
	require.NoError(t, err)
	require.NotEmpty(t, refresh)
	assert.False(t, bytes.Contains(sealed, refresh), "the refresh token is stored sealed")
}

func TestCallbackUnverifiedEmailDoesNotLink(t *testing.T) {
	s := newStand(t)
	s.bob()
	s.op.SetUser("sub-bob", map[string]any{"email": "bob@example.com", "email_verified": false})

	res := s.login("/", nil)
	assert.Equal(t, http.StatusFound, res.StatusCode)
	assert.Equal(t, loginFailed, res.Location)
	_, ok := s.session(res)
	assert.False(t, ok, "no session")
	var links int
	require.NoError(t, s.pool.QueryRow(context.Background(), "SELECT count(*) FROM user_idp_link").Scan(&links))
	assert.Zero(t, links)
}

func TestOIDCPKCEAndState(t *testing.T) {
	s := newStand(t)
	s.bob()
	s.op.SetUser("sub-bob", map[string]any{"email": "bob@example.com", "email_verified": true})

	res := s.login("/", func(cb *url.URL) {
		q := cb.Query()
		q.Set("state", s.idp.ID.String()+".other")
		cb.RawQuery = q.Encode()
	})
	assert.Equal(t, "S256", s.op.ChallengeMethod())
	assert.Equal(t, http.StatusBadRequest, res.StatusCode, "a state that is not the browser's")
	_, ok := s.session(res)
	assert.False(t, ok)

	res = s.get(s.browser(), s.srv.URL+"/api/oidc/callback?code=x&state="+s.idp.ID.String()+".x")
	assert.Equal(t, http.StatusBadRequest, res.StatusCode, "no state cookie")
}

func TestOIDCNonceMismatch(t *testing.T) {
	s := newStand(t)
	s.bob()
	s.op.SetUser("sub-bob", map[string]any{"email": "bob@example.com", "email_verified": true})
	for _, nonce := range []string{"other", ""} { // another nonce, or none in the ID token
		s.op.SetNonce(nonce)
		res := s.login("/", nil)
		assert.Equal(t, http.StatusBadRequest, res.StatusCode, "%q", nonce)
		_, ok := s.session(res)
		assert.False(t, ok, "%q", nonce)
	}
}

func TestOIDCDisabledIdP(t *testing.T) {
	s := newStand(t)
	ctx := context.Background()
	s.bob()
	s.op.SetUser("sub-bob", map[string]any{"email": "bob@example.com", "email_verified": true})

	res := s.login("/", func(*url.URL) {
		s.idp.Enabled = false
		_, err := s.store.Update(ctx, s.admin, s.idp)
		require.NoError(t, err)
	})
	assert.Equal(t, loginFailed, res.Location, "disabled between start and callback")
	_, ok := s.session(res)
	assert.False(t, ok)

	res = s.get(s.browser(), s.srv.URL+"/api/oidc/"+s.idp.ID.String()+"/start")
	assert.Equal(t, http.StatusNotFound, res.StatusCode)
	res = s.get(s.browser(), s.srv.URL+"/api/oidc/"+uuid.NewString()+"/start")
	assert.Equal(t, http.StatusNotFound, res.StatusCode, "unknown IdP")
}

func TestCallbackRejectsExternalNext(t *testing.T) {
	s := newStand(t)
	s.bob()
	s.op.SetUser("sub-bob", map[string]any{"email": "bob@example.com", "email_verified": true})

	for _, next := range []string{"//evil.test/x", "https://evil.test", "/\\evil.test"} {
		res := s.login(next, nil)
		assert.Equal(t, "/", res.Location, next)
		_, ok := s.session(res)
		assert.True(t, ok, next)
	}
}

func TestOIDCIdPDownRefusesLogin(t *testing.T) {
	s := newStand(t)
	s.bob()
	s.op.SetUser("sub-bob", map[string]any{"email": "bob@example.com", "email_verified": true})
	first, ok := s.session(s.login("/", nil))
	require.True(t, ok)

	s.op.SetDown(true)
	res := s.get(s.browser(), s.srv.URL+"/api/oidc/"+s.idp.ID.String()+"/start")
	assert.Equal(t, http.StatusFound, res.StatusCode)
	assert.Equal(t, loginFailed, res.Location)
	var sessions int
	require.NoError(t, s.pool.QueryRow(context.Background(), "SELECT count(*) FROM user_session WHERE id = $1", first.SessionID).Scan(&sessions))
	assert.Equal(t, 1, sessions, "an unreachable IdP revokes nothing")
}
