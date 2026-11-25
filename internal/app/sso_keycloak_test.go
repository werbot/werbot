//go:build unix

package app

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"html"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
	"github.com/werbot/werbot/gen/werbot/v1/werbotv1connect"
	"github.com/werbot/werbot/internal/clientauth"
	"github.com/werbot/werbot/internal/identity/oidclogin"
)

// keycloak is the base URL of the stand of deploy/e2e/keycloak.yaml (make
// e2e-sso); empty skips the SSO tests.
var keycloak = flag.String("keycloak", "", "Keycloak base URL with the werbot realm, e.g. http://127.0.0.1:18180")

// Values of deploy/e2e/keycloak (test only).
const (
	kcClientSecret = "werbot-test-secret" //nolint:gosec // test realm, not a credential
	kcAdmin        = "admin"
)

// kcAPI is the admin REST API of the Keycloak stand, realm werbot.
type kcAPI struct {
	t     *testing.T
	token string
}

// keycloakAdmin skips the test without -keycloak and signs in as the admin.
func keycloakAdmin(t *testing.T) *kcAPI {
	t.Helper()
	if *keycloak == "" {
		t.Skip("no -keycloak: run make e2e-sso")
	}
	res, err := http.PostForm(*keycloak+"/realms/master/protocol/openid-connect/token", url.Values{ //nolint:noctx // test stand
		"grant_type": {"password"}, "client_id": {"admin-cli"}, "username": {kcAdmin}, "password": {kcAdmin}})
	require.NoError(t, err)
	defer func() { _ = res.Body.Close() }()
	var tok struct {
		AccessToken string `json:"access_token"`
	}
	require.NoError(t, json.NewDecoder(res.Body).Decode(&tok))
	require.NotEmpty(t, tok.AccessToken, "admin token: %d", res.StatusCode)
	return &kcAPI{t: t, token: tok.AccessToken}
}

// do calls the admin API of realm werbot and decodes the answer into out.
func (k *kcAPI) do(method, path string, body, out any) {
	k.t.Helper()
	var in io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		require.NoError(k.t, err)
		in = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(context.Background(), method, *keycloak+"/admin/realms/werbot"+path, in)
	require.NoError(k.t, err)
	req.Header.Set("Authorization", "Bearer "+k.token)
	req.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	require.NoError(k.t, err)
	defer func() { _ = res.Body.Close() }()
	require.Less(k.t, res.StatusCode, 300, "%s %s", method, path)
	if out != nil {
		require.NoError(k.t, json.NewDecoder(res.Body).Decode(out))
	}
}

// id is the id of the only user or group the query finds.
func (k *kcAPI) id(path string) string {
	k.t.Helper()
	var found []struct{ ID string }
	k.do(http.MethodGet, path, nil, &found)
	require.Len(k.t, found, 1, path)
	return found[0].ID
}

func (k *kcAPI) user(name string) string { return k.id("/users?exact=true&username=" + name) }

// setGroup puts user in the top-level group or takes it out.
func (k *kcAPI) setGroup(user, group string, member bool) {
	k.t.Helper()
	method := map[bool]string{true: http.MethodPut, false: http.MethodDelete}[member]
	k.do(method, "/users/"+k.user(user)+"/groups/"+k.id("/groups?exact=true&search="+group), nil, nil)
}

// endSessions removes the online SSO sessions of user as their idle timeout
// does (Keycloak 26 keeps none after an offline_access login); unlike
// POST /users/{id}/logout it leaves the user's notBefore, which makes every
// token issued in an earlier second stale, offline ones included.
func (k *kcAPI) endSessions(user string) {
	k.t.Helper()
	var sessions []struct{ ID string }
	k.do(http.MethodGet, "/users/"+k.user(user)+"/sessions", nil, &sessions)
	for _, s := range sessions {
		k.do(http.MethodDelete, "/sessions/"+s.ID, nil, nil)
	}
}

// setEnabled enables or disables user; the rest of the user is sent back as read.
func (k *kcAPI) setEnabled(user string, enabled bool) {
	k.t.Helper()
	path := "/users/" + k.user(user)
	var rep map[string]any
	k.do(http.MethodGet, path, nil, &rep)
	rep["enabled"] = enabled
	k.do(http.MethodPut, path, rep, nil)
}

// createIDP adds the Keycloak realm as an IdP of alice's organization with
// mapping, offline_access for a refresh token that outlives the SSO session.
func createIDP(t *testing.T, s *stack, mapping ...*werbotv1.ClaimMapping) string {
	t.Helper()
	res, err := werbotv1connect.NewIdentityProviderServiceClient(s.aliceHTTP(t), s.url).CreateIdentityProvider(context.Background(),
		connect.NewRequest(&werbotv1.CreateIdentityProviderRequest{Name: "keycloak", Issuer: *keycloak + "/realms/werbot",
			ClientId: "werbot", ClientSecret: kcClientSecret, Scopes: []string{"openid", "email", "profile", "offline_access"},
			ClaimsMapping: mapping, AllowSignup: true, Enabled: true}))
	require.NoError(t, err)
	return res.Msg.GetIdentityProvider().GetId()
}

var kcLoginForm = regexp.MustCompile(`id="kc-form-login"[^>]*action="([^"]+)"`)

// ssoLogin signs user in through IdP idp in a new browser: the Keycloak
// login form is posted and the redirects end at werbot's "/".
func ssoLogin(t *testing.T, s *stack, idp, user string) *http.Client {
	t.Helper()
	hc, _ := jarClient(t, http.DefaultTransport)
	res, err := hc.Get(s.url + "/api/oidc/" + idp + "/start?next=/") //nolint:noctx // test browser
	require.NoError(t, err)
	page, err := io.ReadAll(res.Body)
	require.NoError(t, err)
	_ = res.Body.Close()
	form := kcLoginForm.FindSubmatch(page)
	require.NotNil(t, form, "no Keycloak login form at %s", res.Request.URL)
	res, err = hc.PostForm(html.UnescapeString(string(form[1])), url.Values{ //nolint:noctx // test browser
		"username": {user}, "password": {user + "-password"}, "credentialId": {""}})
	require.NoError(t, err)
	_ = res.Body.Close()
	require.Equal(t, s.url+"/", res.Request.URL.String(), "the login ends at next")
	return hc
}

// member reads the role of the user with email and the number of their teams.
func member(t *testing.T, s *stack, email string) (role string, teams int) {
	t.Helper()
	require.NoError(t, s.pool.QueryRow(context.Background(), `SELECT m.role, (SELECT count(*) FROM team_member tm WHERE tm.user_id = u.id)
		FROM users u JOIN membership m ON m.user_id = u.id WHERE u.email = $1`, email).Scan(&role, &teams))
	return role, teams
}

// A Keycloak login provisions the user with the mapped team and role; a
// later login without the group takes the team away.
func TestKeycloakLoginMapsGroups(t *testing.T) {
	kc := keycloakAdmin(t)
	s := startStack(t)
	team, err := werbotv1connect.NewTeamServiceClient(s.aliceHTTP(t), s.url).CreateTeam(context.Background(),
		connect.NewRequest(&werbotv1.CreateTeamRequest{Name: "ops"}))
	require.NoError(t, err)
	idp := createIDP(t, s,
		&werbotv1.ClaimMapping{Claim: "groups", Regex: "^/ops$", Target: &werbotv1.ClaimMapping_TeamId{TeamId: team.Msg.GetTeam().GetId()}},
		&werbotv1.ClaimMapping{Claim: "groups", Regex: "^/admins$", Target: &werbotv1.ClaimMapping_Role{Role: "admin"}})

	hc := ssoLogin(t, s, idp, "sso-alice")
	me, err := werbotv1connect.NewAuthServiceClient(hc, s.url).GetSession(context.Background(), connect.NewRequest(&werbotv1.GetSessionRequest{}))
	require.NoError(t, err)
	assert.Equal(t, "sso-alice@example.com", me.Msg.GetUser().GetEmail())
	role, teams := member(t, s, "sso-alice@example.com")
	assert.Equal(t, []any{"admin", 1}, []any{role, teams})

	kc.setGroup("sso-alice", "ops", false)
	t.Cleanup(func() { kc.setGroup("sso-alice", "ops", true) })
	ssoLogin(t, s, idp, "sso-alice")
	role, teams = member(t, s, "sso-alice@example.com")
	assert.Equal(t, []any{"admin", 0}, []any{role, teams}, "the team follows the groups")
}

// idp.recheck keeps an SSO user whose Keycloak session ended (offline
// token) and ends every session of one Keycloak disabled.
func TestKeycloakDisabledUserRevoked(t *testing.T) {
	kc := keycloakAdmin(t)
	s := startStack(t)
	ctx := context.Background()
	idp := createIDP(t, s)
	hc := ssoLogin(t, s, idp, "sso-bob")
	b := browser{hc: hc, consent: werbotv1connect.NewOAuthConsentServiceClient(hc, s.url)}
	_, err := clientauth.Login(ctx, t.TempDir(), s.url, clientauth.LoginOptions{OpenBrowser: b.approveLoopback(t)})
	require.NoError(t, err)
	var bob string
	require.NoError(t, s.pool.QueryRow(ctx, "SELECT id::text FROM users WHERE email = 'sso-bob@example.com'").Scan(&bob))
	svc := s.services(t)
	recheck := func() {
		t.Helper()
		_, err := s.pool.Exec(ctx, "UPDATE user_idp_link SET last_checked_at = now() - interval '2 hours'")
		require.NoError(t, err)
		require.NoError(t, oidclogin.Recheck(ctx, svc.IdPs, svc.Identity, svc.OIDC))
	}
	revoked := "SELECT count(*) FROM cli_session WHERE user_id = $1 AND revoke_reason = 'idp'"

	kc.endSessions("sso-bob")
	recheck()
	assert.Equal(t, 1, s.count(t, "SELECT count(*) FROM user_session WHERE user_id = $1", bob))
	assert.Zero(t, s.count(t, revoked, bob))
	assert.Equal(t, 1, s.count(t, "SELECT count(*) FROM user_idp_link WHERE last_checked_at > now() - interval '1 minute'"),
		"the offline token was refreshed")

	kc.setEnabled("sso-bob", false)
	t.Cleanup(func() { kc.setEnabled("sso-bob", true) })
	recheck()
	assert.Zero(t, s.count(t, "SELECT count(*) FROM user_session WHERE user_id = $1", bob))
	assert.Equal(t, 1, s.count(t, revoked, bob))
	assert.Equal(t, 1, s.count(t, "SELECT count(*) FROM audit_event WHERE action = 'idp.recheck.revoke' AND target_id = $1", bob))
}
