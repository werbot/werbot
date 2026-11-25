package oauth_test

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/identity/oauth"
	"github.com/werbot/werbot/internal/platform/apperr"
	"github.com/werbot/werbot/internal/platform/clientip"
	"github.com/werbot/werbot/internal/platform/db/dbtest"
	"github.com/werbot/werbot/internal/platform/principal"
	"github.com/werbot/werbot/internal/platform/secretbox"
)

const (
	testWebURL   = "https://ui.werbot.test"
	testRedirect = "http://127.0.0.1:53111/callback"
)

func testBox(t *testing.T) *secretbox.Box {
	t.Helper()
	box, err := secretbox.New(bytes.Repeat([]byte{7}, 32), nil)
	require.NoError(t, err)
	return box
}

// serveProvider starts one more provider on pool, behind clientip like the
// server. Its issuer is its own URL unless issuer is set (another replica);
// agents nil serves no token exchange.
func serveProvider(t *testing.T, pool *pgxpool.Pool, key *oauth.SigningKey, agents oauth.Agents, issuer string) (*oauth.Provider, *httptest.Server) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	if issuer == "" {
		issuer = "http://" + ln.Addr().String()
	}
	p, err := oauth.New(pool, key, dbUsers{pool}, agents, oauth.Config{Issuer: issuer, WebURL: testWebURL})
	require.NoError(t, err)
	srv := &httptest.Server{Listener: ln, Config: &http.Server{Handler: clientip.Middleware(nil)(p.Handler())}} //nolint:gosec // test server
	srv.Start()
	t.Cleanup(srv.Close)
	return p, srv
}

// stand is a provider with a seeded organization.
type stand struct {
	p    *oauth.Provider
	key  *oauth.SigningKey
	pool *pgxpool.Pool
	srv  *httptest.Server
	fx   dbtest.Fixture
}

func newStand(t *testing.T) stand {
	t.Helper()
	return newAgentStand(t, nil)
}

// newAgentStand is a stand whose provider exchanges tokens through agents.
func newAgentStand(t *testing.T, agents oauth.Agents) stand {
	t.Helper()
	pool, _ := dbtest.New(t)
	key, err := oauth.LoadSigningKey(context.Background(), pool, testBox(t))
	require.NoError(t, err)
	p, srv := serveProvider(t, pool, key, agents, "")
	return stand{p: p, key: key, pool: pool, srv: srv, fx: dbtest.Seed(t, pool)}
}

// dbUsers is oauth.Users over the membership table (*identity.Service in the server).
type dbUsers struct{ pool *pgxpool.Pool }

func (u dbUsers) Membership(ctx context.Context, orgID, userID uuid.UUID) (string, error) {
	var role string
	err := u.pool.QueryRow(ctx, "SELECT role FROM membership WHERE org_id = $1 AND user_id = $2", orgID, userID).Scan(&role)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", apperr.ErrNotFound
	}
	return role, err
}

// cookieAs is a browser (cookie) principal of userID.
func cookieAs(fx dbtest.Fixture, userID uuid.UUID) principal.Principal {
	p := fx.Principal(userID)
	p.AuthMethod = principal.AuthMethodCookie
	return p
}

// pkce returns a code verifier and its S256 challenge.
func pkce() (verifier, challenge string) {
	verifier = rand.Text() + rand.Text()
	sum := sha256.Sum256([]byte(verifier))
	return verifier, base64.RawURLEncoding.EncodeToString(sum[:])
}

func authorizeURL(srv *httptest.Server, redirectURI, challenge string, extra url.Values) string {
	q := url.Values{
		"client_id": {oauth.ClientCLI}, "response_type": {"code"}, "scope": {"openid offline_access"},
		"redirect_uri": {redirectURI}, "state": {"st-1"}, "code_challenge": {challenge}, "code_challenge_method": {"S256"},
	}
	for k, v := range extra {
		q[k] = v
	}
	return srv.URL + "/oauth/authorize?" + q.Encode()
}

// noRedirect does not follow redirects, so tests see each Location.
var noRedirect = &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}

// location GETs u without following redirects: status and Location.
func location(t *testing.T, u string) (int, *url.URL) {
	t.Helper()
	res, err := noRedirect.Get(u)
	require.NoError(t, err)
	_ = res.Body.Close()
	loc, err := url.Parse(res.Header.Get("Location"))
	require.NoError(t, err)
	return res.StatusCode, loc
}

// authorize starts a loopback request on srv and returns its id from the consent URL.
func authorize(t *testing.T, srv *httptest.Server, challenge string, extra url.Values) uuid.UUID {
	t.Helper()
	code, loc := location(t, authorizeURL(srv, testRedirect, challenge, extra))
	require.Equal(t, http.StatusFound, code)
	require.Equal(t, testWebURL+"/cli/authorize", loc.Scheme+"://"+loc.Host+loc.Path)
	id, err := uuid.Parse(loc.Query().Get("id"))
	require.NoError(t, err)
	return id
}

// callback follows a consent redirect_url (on WebURL) on srv: the loopback redirect.
func callback(t *testing.T, srv *httptest.Server, redirectURL string) *url.URL {
	t.Helper()
	require.True(t, strings.HasPrefix(redirectURL, testWebURL+"/oauth/authorize/callback?id="), redirectURL)
	code, loc := location(t, srv.URL+strings.TrimPrefix(redirectURL, testWebURL))
	require.Equal(t, http.StatusFound, code)
	return loc
}

// login runs authorize, consent by userID and the callback; it returns the code and verifier.
func login(t *testing.T, s stand, userID uuid.UUID) (code, verifier string) {
	t.Helper()
	verifier, challenge := pkce()
	id := authorize(t, s.srv, challenge, url.Values{"werbot_hostname": {"laptop-1"}})
	redirect, err := s.p.ApproveAuthRequest(context.Background(), cookieAs(s.fx, userID), id)
	require.NoError(t, err)
	loc := callback(t, s.srv, redirect)
	require.Empty(t, loc.Query().Get("error"))
	return loc.Query().Get("code"), verifier
}

type tokenResponse struct {
	Status       int    `json:"-"`
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	IDToken      string `json:"id_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	Error        string `json:"error"`
}

func exchangeCode(t *testing.T, srv *httptest.Server, code, verifier, redirectURI string) tokenResponse {
	t.Helper()
	res, err := http.PostForm(srv.URL+"/oauth/token", url.Values{
		"grant_type": {"authorization_code"}, "client_id": {oauth.ClientCLI}, "code": {code},
		"code_verifier": {verifier}, "redirect_uri": {redirectURI},
	})
	require.NoError(t, err)
	defer func() { _ = res.Body.Close() }()
	tr := tokenResponse{Status: res.StatusCode}
	require.NoError(t, json.NewDecoder(res.Body).Decode(&tr))
	return tr
}

func getJSON(t *testing.T, url string) (body []byte, doc map[string]any) {
	t.Helper()
	res, err := http.Get(url) //nolint:gosec // loopback test server
	require.NoError(t, err)
	defer func() { _ = res.Body.Close() }()
	require.Equal(t, http.StatusOK, res.StatusCode, url)
	require.NoError(t, json.NewDecoder(res.Body).Decode(&doc))
	body, err = json.Marshal(doc)
	require.NoError(t, err)
	return body, doc
}
