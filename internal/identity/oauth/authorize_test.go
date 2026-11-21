package oauth_test

import (
	"context"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/identity/oauth"
)

// TestAuthorizeRequiresPKCES256: without an S256 challenge the request is
// refused at the (valid) loopback redirect; with one the browser goes to consent.
func TestAuthorizeRequiresPKCES256(t *testing.T) {
	s := newStand(t)
	_, challenge := pkce()
	for name, extra := range map[string]url.Values{
		"no challenge": {"code_challenge": {""}, "code_challenge_method": {""}},
		"plain":        {"code_challenge_method": {"plain"}},
	} {
		code, loc := location(t, authorizeURL(s.srv, testRedirect, challenge, extra))
		assert.Equal(t, http.StatusFound, code, name)
		assert.Equal(t, testRedirect, loc.Scheme+"://"+loc.Host+loc.Path, name)
		assert.Equal(t, "invalid_request", loc.Query().Get("error"), name)
		assert.Equal(t, "st-1", loc.Query().Get("state"), name)
	}
	authorize(t, s.srv, challenge, nil) // S256 → https://ui.werbot.test/cli/authorize?id=<uuid>
}

// TestAuthorizeRequiresOfflineAccess: the CLI always needs a refresh token.
func TestAuthorizeRequiresOfflineAccess(t *testing.T) {
	s := newStand(t)
	_, challenge := pkce()
	for _, scope := range []string{"openid", "openid offline_access profile"} {
		code, loc := location(t, authorizeURL(s.srv, testRedirect, challenge, url.Values{"scope": {scope}}))
		assert.Equal(t, http.StatusFound, code, scope)
		assert.Equal(t, "invalid_scope", loc.Query().Get("error"), scope)
	}
}

// TestAuthorizeRedirectURI: only http://127.0.0.1 or http://[::1], any port,
// path /callback, no query; anything else is a 400 with no Location at all.
func TestAuthorizeRedirectURI(t *testing.T) {
	s := newStand(t)
	_, challenge := pkce()
	for _, ok := range []string{"http://127.0.0.1:53111/callback", "http://[::1]:1/callback", "http://127.0.0.1/callback"} {
		code, loc := location(t, authorizeURL(s.srv, ok, challenge, nil))
		assert.Equal(t, http.StatusFound, code, ok)
		assert.Equal(t, testWebURL, loc.Scheme+"://"+loc.Host, ok)
	}
	bad := []string{
		"http://localhost:1/callback", "https://127.0.0.1/callback", "http://127.0.0.1:1/other",
		"http://evil.example/callback", "http://127.0.0.1:1/callback?x=1", "http://127.0.0.1:1/callback#f",
		"http://127.0.0.2:1/callback", "http://user@127.0.0.1:1/callback", "http://127.0.0.1:1/callback/",
		"http://[0:0:0:0:0:0:0:1]:1/callback", "", "/callback",
	}
	for _, uri := range bad {
		code, loc := location(t, authorizeURL(s.srv, uri, challenge, nil))
		assert.Equal(t, http.StatusBadRequest, code, uri)
		assert.Empty(t, loc.String(), uri)
	}
	// A second redirect_uri or client_id (op's decoder takes the last one) is refused too.
	u := authorizeURL(s.srv, testRedirect, challenge, nil) + "&redirect_uri=" + url.QueryEscape("http://localhost:1/callback")
	code, loc := location(t, u)
	assert.Equal(t, http.StatusBadRequest, code)
	assert.Empty(t, loc.String())
	code, loc = location(t, authorizeURL(s.srv, testRedirect, challenge, nil)+"&client_id=werbot-cli")
	assert.Equal(t, http.StatusBadRequest, code)
	assert.Empty(t, loc.String())
}

// TestAuthorizeStoresClientInfo: the consent page shows the sanitized
// werbot_hostname, the client IP and the scopes.
func TestAuthorizeStoresClientInfo(t *testing.T) {
	s := newStand(t)
	_, challenge := pkce()
	id := authorize(t, s.srv, challenge, url.Values{"werbot_hostname": {"laptop-1\x07"}})
	info, err := s.p.GetAuthRequest(context.Background(), cookieAs(s.fx, s.fx.MemberID), id)
	require.NoError(t, err)
	assert.Equal(t, oauth.ClientInfo{
		ClientID: oauth.ClientCLI, Hostname: "laptop-1", IP: netip.MustParseAddr("127.0.0.1"),
		Scopes: []string{"openid", "offline_access"},
	}, info)

	long := strings.Repeat("h", 300)
	id = authorize(t, s.srv, challenge, url.Values{"werbot_hostname": {long}})
	info, err = s.p.GetAuthRequest(context.Background(), cookieAs(s.fx, s.fx.MemberID), id)
	require.NoError(t, err)
	assert.Equal(t, long[:255], info.Hostname)
}

// TestOAuthBodyCap: /oauth/ forms are read up to 64 KiB.
func TestOAuthBodyCap(t *testing.T) {
	s := newStand(t)
	body := "grant_type=authorization_code&client_id=werbot-cli&code=" + strings.Repeat("a", 64<<10)
	res, err := http.Post(s.srv.URL+"/oauth/token", "application/x-www-form-urlencoded", strings.NewReader(body))
	require.NoError(t, err)
	_ = res.Body.Close()
	assert.Equal(t, http.StatusRequestEntityTooLarge, res.StatusCode)
}
