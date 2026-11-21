package oauth_test

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/identity/oauth"
)

func TestDiscovery(t *testing.T) {
	srv := newStand(t).srv
	oidcBody, doc := getJSON(t, srv.URL+"/.well-known/openid-configuration")
	asBody, _ := getJSON(t, srv.URL+"/.well-known/oauth-authorization-server")
	assert.JSONEq(t, string(oidcBody), string(asBody), "RFC 8414 document is the same")

	issuer := srv.URL
	assert.Equal(t, issuer, doc["issuer"])
	assert.Equal(t, issuer+"/oauth/authorize", doc["authorization_endpoint"])
	assert.Equal(t, issuer+"/oauth/token", doc["token_endpoint"])
	assert.Equal(t, issuer+"/oauth/device_authorization", doc["device_authorization_endpoint"])
	assert.Equal(t, issuer+"/oauth/revoke", doc["revocation_endpoint"])
	assert.Equal(t, issuer+"/oauth/keys", doc["jwks_uri"])
	assert.Equal(t, []any{"S256"}, doc["code_challenge_methods_supported"])
	assert.Subset(t, doc["grant_types_supported"], []any{"refresh_token", "urn:ietf:params:oauth:grant-type:device_code"})
}

func TestJWKS(t *testing.T) {
	s := newStand(t)
	_, doc := getJSON(t, s.srv.URL+"/oauth/keys")
	keys, ok := doc["keys"].([]any)
	require.True(t, ok)
	require.Len(t, keys, 1)
	jwk := keys[0].(map[string]any)
	assert.Equal(t, "EC", jwk["kty"])
	assert.Equal(t, "P-256", jwk["crv"])
	assert.Equal(t, "ES256", jwk["alg"])
	assert.Equal(t, s.key.ID(), jwk["kid"])
	assert.NotContains(t, jwk, "d")
}

func TestUnknownClientRejected(t *testing.T) {
	srv := newStand(t).srv
	q := url.Values{
		"client_id": {"x"}, "response_type": {"code"}, "scope": {"openid offline_access"},
		"redirect_uri": {"http://127.0.0.1:1/callback"}, "code_challenge": {strings.Repeat("a", 43)}, "code_challenge_method": {"S256"},
	}
	res, err := noRedirect.Get(srv.URL + "/oauth/authorize?" + q.Encode())
	require.NoError(t, err)
	_ = res.Body.Close()
	assert.Equal(t, http.StatusBadRequest, res.StatusCode)
	assert.Empty(t, res.Header.Get("Location"), "no redirect, least of all to the login page")
}

func TestNewRejectsBadIssuer(t *testing.T) {
	s := newStand(t)
	for _, issuer := range []string{"", "ftp://werbot.test", "https://"} {
		_, err := oauth.New(s.pool, s.key, nil, oauth.Config{Issuer: issuer, WebURL: testWebURL})
		assert.Error(t, err, issuer)
	}
	_, err := oauth.New(s.pool, s.key, nil, oauth.Config{Issuer: "https://werbot.test", WebURL: testWebURL})
	assert.NoError(t, err)
}

// TestNoCORS: the CLI never needs CORS and the web UI reaches /oauth/ same-origin
// through werbot web, so no cross-origin page may read these endpoints.
func TestNoCORS(t *testing.T) {
	srv := newStand(t).srv
	for _, c := range []struct{ method, path, body string }{
		{http.MethodGet, "/.well-known/openid-configuration", ""},
		{http.MethodPost, "/oauth/token", "grant_type=refresh_token&client_id=werbot-cli&refresh_token=x"},
		{http.MethodOptions, "/oauth/token", ""},
	} {
		req, err := http.NewRequest(c.method, srv.URL+c.path, strings.NewReader(c.body))
		require.NoError(t, err)
		req.Header.Set("Origin", "https://evil.example")
		req.Header.Set("Access-Control-Request-Method", http.MethodPost)
		if c.body != "" {
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		}
		res, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		_ = res.Body.Close()
		assert.Empty(t, res.Header.Get("Access-Control-Allow-Origin"), c.method+" "+c.path)
		assert.Empty(t, res.Header.Get("Access-Control-Allow-Credentials"), c.method+" "+c.path)
	}
}
