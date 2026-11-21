package identity

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
	"github.com/werbot/werbot/gen/werbot/v1/werbotv1connect"
	"github.com/werbot/werbot/internal/identity/oauth"
	"github.com/werbot/werbot/internal/platform/principal"
)

// signES256 signs claims as an ES256 JWT with key (stdlib only: identity
// tests do not import go-jose).
func signES256(t *testing.T, key *ecdsa.PrivateKey, kid string, claims map[string]any) string {
	t.Helper()
	enc := func(v any) string {
		b, err := json.Marshal(v)
		require.NoError(t, err)
		return base64.RawURLEncoding.EncodeToString(b)
	}
	signing := enc(map[string]string{"alg": "ES256", "typ": "JWT", "kid": kid}) + "." + enc(claims)
	sum := sha256.Sum256([]byte(signing))
	r, sv, err := ecdsa.Sign(rand.Reader, key, sum[:])
	require.NoError(t, err)
	sig := make([]byte, 64)
	r.FillBytes(sig[:32])
	sv.FillBytes(sig[32:])
	return signing + "." + base64.RawURLEncoding.EncodeToString(sig)
}

func TestBearerPrincipal(t *testing.T) {
	s := newCLIStand(t)
	org := seedOrg(t, s.pool, "Acme")
	alice := seedUser(t, s.pool, org, "alice", principal.RoleAdmin)
	tr := s.cliLogin(t, org, alice)

	res, err := s.auth().GetSession(context.Background(), bearer(&werbotv1.GetSessionRequest{}, tr.AccessToken))
	require.NoError(t, err)
	assert.Equal(t, "alice@example.com", res.Msg.GetUser().GetEmail())
	assert.Equal(t, "Acme", res.Msg.GetOrganization().GetName())
	assert.Equal(t, werbotv1.Role_ROLE_ADMIN, res.Msg.GetRole())

	p, ok := whoami(t, s.srv, bearerHeader(tr.AccessToken))
	require.True(t, ok)
	assert.Equal(t, s.asBearer(t, tr.AccessToken, principal.RoleAdmin), p)
	assert.Equal(t, uuid.Nil, p.SessionID)
	var sessID uuid.UUID
	require.NoError(t, s.pool.QueryRow(context.Background(), "SELECT id FROM cli_session").Scan(&sessID))
	assert.Equal(t, sessID, p.CLISessionID, "CLISessionID is the token's jti")
}

func TestBearerRevokedNextRequest(t *testing.T) {
	s := newCLIStand(t)
	org := seedOrg(t, s.pool, "Acme")
	alice := seedUser(t, s.pool, org, "alice", principal.RoleMember)
	tr := s.cliLogin(t, org, alice)
	ctx := context.Background()
	_, err := s.auth().GetSession(ctx, bearer(&werbotv1.GetSessionRequest{}, tr.AccessToken))
	require.NoError(t, err)

	id := s.asBearer(t, tr.AccessToken, "").CLISessionID.String()
	_, err = s.cliSessions().RevokeCLISession(ctx, bearer(&werbotv1.RevokeCLISessionRequest{Id: id}, tr.AccessToken))
	require.NoError(t, err)

	_, err = s.auth().GetSession(ctx, bearer(&werbotv1.GetSessionRequest{}, tr.AccessToken))
	assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err), "the unexpired JWT of a revoked session")
}

func TestBearerIgnoresCookie(t *testing.T) {
	s := newCLIStand(t)
	org := seedOrg(t, s.pool, "Acme")
	alice := seedUser(t, s.pool, org, "alice", principal.RoleOwner)
	seedUser(t, s.pool, org, "bob", principal.RoleMember)
	_, bobCookie := loginCtx(t, s.svc, "bob")
	cookie := sessionCookieName(true) + "=" + bobCookie

	p, ok := whoami(t, s.srv, http.Header{"Cookie": {cookie}})
	require.True(t, ok, "the cookie alone works")
	assert.Equal(t, principal.AuthMethodCookie, p.AuthMethod)

	for _, authz := range []string{"Bearer garbage", "Basic Ym9iOnB3", "Bearer "} {
		_, ok := whoami(t, s.srv, http.Header{"Cookie": {cookie}, "Authorization": {authz}})
		assert.False(t, ok, "%q: the cookie is not read when Authorization is present", authz)
	}
	req := withCookie(&werbotv1.GetSessionRequest{}, sessionCookieName(true), bobCookie)
	req.Header().Set("Authorization", "Bearer garbage")
	_, err := s.auth().GetSession(context.Background(), req)
	assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))

	tr := s.cliLogin(t, org, alice)
	h := bearerHeader(tr.AccessToken)
	h.Set("Cookie", cookie)
	p, ok = whoami(t, s.srv, h)
	require.True(t, ok)
	assert.Equal(t, alice, p.UserID, "Bearer alice + cookie bob → alice")
	assert.Equal(t, principal.AuthMethodOAuth, p.AuthMethod)
}

func TestBearerRejectsBadToken(t *testing.T) {
	s := newCLIStand(t)
	org := seedOrg(t, s.pool, "Acme")
	alice := seedUser(t, s.pool, org, "alice", principal.RoleOwner)
	tr := s.cliLogin(t, org, alice)
	sess := s.asBearer(t, tr.AccessToken, "").CLISessionID
	priv, ok := s.key.Key().(*ecdsa.PrivateKey)
	require.True(t, ok)
	other, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	claims := func(k string, v any) map[string]any {
		m := map[string]any{
			"iss": s.srv.URL, "sub": alice.String(), "aud": []string{oauth.Audience}, "jti": sess.String(),
			"werbot_org": org.String(), "exp": time.Now().Add(time.Minute).Unix(), "iat": time.Now().Unix(),
		}
		if k != "" {
			m[k] = v
		}
		return m
	}
	_, ok = whoami(t, s.srv, bearerHeader(signES256(t, priv, s.key.ID(), claims("", nil))))
	require.True(t, ok, "control: a token signed here with the right claims passes")

	for name, token := range map[string]string{
		"aud other":         signES256(t, priv, s.key.ID(), claims("aud", []string{"other"})),
		"expired":           signES256(t, priv, s.key.ID(), claims("exp", time.Now().Add(-time.Minute).Unix())),
		"foreign signature": signES256(t, other, s.key.ID(), claims("", nil)),
		"id token":          tr.IDToken,
		"garbage":           "not.a.jwt",
	} {
		_, err := s.auth().GetSession(context.Background(), bearer(&werbotv1.GetSessionRequest{}, token))
		assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err), name)
	}
}

func TestMiddlewareNilBearer(t *testing.T) {
	svc, pool, _ := newTestService(t)
	seedUser(t, pool, seedOrg(t, pool, "Acme"), "alice", principal.RoleOwner)
	_, token := loginCtx(t, svc, "alice")
	mux := http.NewServeMux()
	mux.HandleFunc("/whoami", whoamiHandler)
	srv := httptest.NewServer(Middleware(svc, nil)(mux))
	t.Cleanup(srv.Close)

	p, ok := whoami(t, srv, http.Header{"Cookie": {sessionCookieName(true) + "=" + token}, "Authorization": {"Bearer garbage"}})
	require.True(t, ok, "no verifier: Authorization is ignored, the cookie works")
	assert.Equal(t, principal.AuthMethodCookie, p.AuthMethod)
	_, ok = whoami(t, srv, bearerHeader("garbage"))
	assert.False(t, ok)
}

func TestBearerStoreFailureUnavailable(t *testing.T) {
	s := newCLIStand(t)
	org := seedOrg(t, s.pool, "Acme")
	tr := s.cliLogin(t, org, seedUser(t, s.pool, org, "alice", principal.RoleOwner))
	var logs bytes.Buffer
	s.svc.cfg.Logger = slog.New(slog.NewTextHandler(&logs, nil))
	mux := http.NewServeMux()
	mux.Handle(werbotv1connect.NewAuthServiceHandler(NewHandler(s.svc)))
	mux.HandleFunc("/whoami", whoamiHandler)
	srv := httptest.NewServer(Middleware(s.svc, s.prov)(mux)) // built after the logger is set
	t.Cleanup(srv.Close)
	s.pool.Close() // the cli_session lookup now fails with a store error, not "no session"

	auth := werbotv1connect.NewAuthServiceClient(srv.Client(), srv.URL)
	_, err := auth.GetSession(context.Background(), bearer(&werbotv1.GetSessionRequest{}, tr.AccessToken))
	assert.Equal(t, connect.CodeUnavailable, connect.CodeOf(err))
	_, ok := whoami(t, srv, bearerHeader(tr.AccessToken))
	assert.False(t, ok, "other paths are served without a subject")
	assert.Contains(t, logs.String(), "session lookup failed")
	assert.NotContains(t, logs.String(), tr.AccessToken, "the token never reaches the log")
}

func TestRemovedMemberBearerFails(t *testing.T) {
	s := newCLIStand(t)
	org := seedOrg(t, s.pool, "Acme")
	seedUser(t, s.pool, org, "alice", principal.RoleOwner)
	bob := seedUser(t, s.pool, org, "bob", principal.RoleMember)
	tr := s.cliLogin(t, org, bob)
	ctx, _ := loginCtx(t, s.svc, "alice")

	require.NoError(t, s.svc.RemoveMember(ctx, bob))
	_, err := s.auth().GetSession(context.Background(), bearer(&werbotv1.GetSessionRequest{}, tr.AccessToken))
	assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}
