package oauth_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/identity/oauth"
)

// fakeAgents is oauth.Agents: it records each call and starts a run that
// ends expires after the call, or fails with err.
type fakeAgents struct {
	mu             sync.Mutex
	calls          []startCall
	agentID, runID uuid.UUID
	expires        time.Duration
	err            error
}

type startCall struct {
	org, user            uuid.UUID
	name, client, reason string
}

func newAgents(expires time.Duration) *fakeAgents {
	return &fakeAgents{agentID: uuid.New(), runID: uuid.New(), expires: expires}
}

func (f *fakeAgents) StartAgentRun(_ context.Context, org, user uuid.UUID, name, client, reason string) (uuid.UUID, uuid.UUID, time.Time, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, startCall{org, user, name, client, reason})
	if f.err != nil {
		return uuid.Nil, uuid.Nil, time.Time{}, f.err
	}
	return f.agentID, f.runID, time.Now().Add(f.expires), nil
}

func (f *fakeAgents) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.calls)
}

// sponsorToken signs userID in over the loopback flow: its access token.
func sponsorToken(t *testing.T, s stand, userID uuid.UUID) string {
	t.Helper()
	code, verifier := login(t, s, userID)
	tr := exchangeCode(t, s.srv, code, verifier, testRedirect)
	require.Equal(t, http.StatusOK, tr.Status, tr.Error)
	return tr.AccessToken
}

type exchangeResponse struct {
	Status          int    `json:"-"`
	AccessToken     string `json:"access_token"`
	IssuedTokenType string `json:"issued_token_type"`
	TokenType       string `json:"token_type"`
	ExpiresIn       int    `json:"expires_in"`
	RefreshToken    string `json:"refresh_token"`
	Scope           string `json:"scope"`
	Error           string `json:"error"`
	Description     string `json:"error_description"`
}

// exchangeForm is the form werbot mcp sends to exchange subject for a run of
// agent claude.
func exchangeForm(subject string) url.Values {
	return url.Values{
		"grant_type": {"urn:ietf:params:oauth:grant-type:token-exchange"}, "client_id": {oauth.ClientCLI},
		"subject_token": {subject}, "subject_token_type": {"urn:ietf:params:oauth:token-type:access_token"},
		"scope": {"agent:claude"}, "audience": {oauth.Audience},
		"werbot_client": {"claude-code"}, "werbot_reason": {"fix nginx"},
	}
}

// postToken posts form to /oauth/token, with the Authorization header authz if set.
func postToken(t *testing.T, s stand, form url.Values, authz string) exchangeResponse {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, s.srv.URL+"/oauth/token", strings.NewReader(form.Encode()))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if authz != "" {
		req.Header.Set("Authorization", authz)
	}
	res, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { _ = res.Body.Close() }()
	er := exchangeResponse{Status: res.StatusCode}
	require.NoError(t, json.NewDecoder(res.Body).Decode(&er))
	return er
}

func sessionID(t *testing.T, s stand) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	require.NoError(t, s.pool.QueryRow(context.Background(), "SELECT id FROM cli_session").Scan(&id))
	return id
}

// TestOPTokenExchangeKeepsActClaim: op signs the private act claim as is
// (no actor_token) and takes aud from the audience parameter; the token names
// the sponsor's organization and cli_session.
func TestOPTokenExchangeKeepsActClaim(t *testing.T) {
	ag := newAgents(time.Hour)
	s := newAgentStand(t, ag)
	res := postToken(t, s, exchangeForm(sponsorToken(t, s, s.fx.MemberID)), "")
	require.Equal(t, http.StatusOK, res.Status, res.Description)

	claims := jwtPayload(t, res.AccessToken)
	assert.Equal(t, map[string]any{"sub": "agent:" + ag.agentID.String()}, claims["act"])
	assert.Equal(t, ag.runID.String(), claims["werbot_run"])
	assert.Equal(t, []any{oauth.Audience}, claims["aud"])
	assert.Equal(t, s.fx.OrgID.String(), claims["werbot_org"])
	assert.Equal(t, sessionID(t, s).String(), claims["jti"])
	assert.Equal(t, s.fx.MemberID.String(), claims["sub"])
}

// TestTokenExchange: a sponsor's token and scope agent:<name> give one access
// token of the run, no refresh token; Agents gets the sponsor, the name and
// the client's werbot_client and werbot_reason.
func TestTokenExchange(t *testing.T) {
	ag := newAgents(time.Hour)
	s := newAgentStand(t, ag)
	res := postToken(t, s, exchangeForm(sponsorToken(t, s, s.fx.MemberID)), "")
	require.Equal(t, http.StatusOK, res.Status, res.Description)
	assert.Equal(t, "urn:ietf:params:oauth:token-type:access_token", res.IssuedTokenType)
	assert.Equal(t, "Bearer", res.TokenType)
	assert.Equal(t, "agent:claude", res.Scope)
	assert.Empty(t, res.RefreshToken)
	assert.Equal(t, 1, countSessions(t, s), "the exchange opens no cli_session")

	c, err := s.p.VerifyAccessToken(context.Background(), res.AccessToken)
	require.NoError(t, err)
	assert.Equal(t, s.fx.MemberID, c.UserID)
	assert.Equal(t, s.fx.OrgID, c.OrgID)
	assert.Equal(t, sessionID(t, s), c.CLISessionID)
	require.NotNil(t, c.AgentID)
	require.NotNil(t, c.RunID)
	assert.Equal(t, ag.agentID, *c.AgentID)
	assert.Equal(t, ag.runID, *c.RunID)
	assert.Equal(t, []startCall{{s.fx.OrgID, s.fx.MemberID, "claude", "claude-code", "fix nginx"}}, ag.calls)

	form := exchangeForm(sponsorToken(t, s, s.fx.MemberID))
	form.Del("werbot_client")
	form.Set("werbot_reason", "a\x00b")
	res = postToken(t, s, form, "")
	require.Equal(t, http.StatusOK, res.Status, res.Description)
	assert.Equal(t, startCall{s.fx.OrgID, s.fx.MemberID, "claude", "unknown", "ab"}, ag.calls[1],
		"no werbot_client is unknown; control characters are dropped")
}

// TestTokenExchangeExpiry: the token ends with the run, at most AgentTokenMaxTTL from now.
func TestTokenExchangeExpiry(t *testing.T) {
	for _, tc := range []struct {
		run, want time.Duration
	}{{10 * time.Minute, 10 * time.Minute}, {2 * time.Hour, oauth.AgentTokenMaxTTL}} {
		s := newAgentStand(t, newAgents(tc.run))
		res := postToken(t, s, exchangeForm(sponsorToken(t, s, s.fx.MemberID)), "")
		require.Equal(t, http.StatusOK, res.Status, res.Description)
		assert.InDelta(t, tc.want.Seconds(), float64(res.ExpiresIn), 2, tc.run)
		exp, ok := jwtPayload(t, res.AccessToken)["exp"].(float64)
		require.True(t, ok)
		assert.InDelta(t, float64(time.Now().Add(tc.want).Unix()), exp, 2, tc.run)
	}
}

// TestTokenExchangeClientAuth: the exchange authenticates werbot-cli with an
// empty secret only, sent as Basic or as one form client_id.
func TestTokenExchangeClientAuth(t *testing.T) {
	s := newAgentStand(t, newAgents(time.Hour))
	subject := sponsorToken(t, s, s.fx.MemberID)
	basic := func(id, secret string) string {
		r, _ := http.NewRequest(http.MethodGet, "/", nil)
		r.SetBasicAuth(id, secret)
		return r.Header.Get("Authorization")
	}
	noClient := exchangeForm(subject)
	noClient.Del("client_id")
	twoClients := exchangeForm(subject)
	twoClients["client_id"] = []string{oauth.ClientCLI, oauth.ClientCLI}
	otherClient := exchangeForm(subject)
	otherClient.Set("client_id", "other")

	ok := []struct {
		name  string
		form  url.Values
		authz string
	}{
		{"form client_id", exchangeForm(subject), ""},
		{"basic without secret", noClient, basic(oauth.ClientCLI, "")},
	}
	for _, tc := range ok {
		res := postToken(t, s, tc.form, tc.authz)
		assert.Equal(t, http.StatusOK, res.Status, "%s: %s", tc.name, res.Description)
	}
	bad := []struct {
		name  string
		form  url.Values
		authz string
	}{
		{"basic with secret", noClient, basic(oauth.ClientCLI, "secret")},
		{"basic other client", noClient, basic("other", "")},
		{"header wins over form", exchangeForm(subject), basic("other", "")},
		{"form other client", otherClient, ""},
		{"two form client_ids", twoClients, ""},
		{"no client", noClient, ""},
	}
	for _, tc := range bad {
		res := postToken(t, s, tc.form, tc.authz)
		assert.Equal(t, "invalid_client", res.Error, tc.name)
		assert.True(t, res.AccessToken == "", tc.name)
	}

	// Outside the exchange werbot-cli still cannot authenticate.
	req, err := http.NewRequest(http.MethodPost, s.srv.URL+"/oauth/introspect", strings.NewReader(url.Values{"token": {subject}}.Encode()))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Authorization", basic(oauth.ClientCLI, ""))
	res, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	_ = res.Body.Close()
	assert.Equal(t, http.StatusUnauthorized, res.StatusCode, "introspection")
}
