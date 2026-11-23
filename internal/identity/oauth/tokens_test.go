package oauth_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/identity/oauth"
)

func countSessions(t *testing.T, s stand) int {
	t.Helper()
	var n int
	require.NoError(t, s.pool.QueryRow(context.Background(), "SELECT count(*) FROM cli_session").Scan(&n))
	return n
}

// TestApproveIssuesCodeAndToken: consent → code on the loopback → tokens and
// a cli_session of the consenting member, audited as cli.login.
func TestApproveIssuesCodeAndToken(t *testing.T) {
	s := newStand(t)
	ctx := context.Background()
	verifier, challenge := pkce()
	id := authorize(t, s.srv, challenge, url.Values{"werbot_hostname": {"laptop-1"}})
	redirect, err := s.p.ApproveAuthRequest(ctx, cookieAs(s.fx, s.fx.MemberID), id)
	require.NoError(t, err)
	loc := callback(t, s.srv, redirect)
	assert.Equal(t, testRedirect, loc.Scheme+"://"+loc.Host+loc.Path)
	assert.Equal(t, "st-1", loc.Query().Get("state"))
	code := loc.Query().Get("code")
	require.NotEmpty(t, code)

	tr := exchangeCode(t, s.srv, code, verifier, testRedirect)
	require.Equal(t, http.StatusOK, tr.Status, tr.Error)
	assert.NotEmpty(t, tr.AccessToken)
	assert.Regexp(t, `^wrt_[A-Za-z0-9_-]{43}$`, tr.RefreshToken)
	assert.InDelta(t, 600, tr.ExpiresIn, 2)
	assert.Equal(t, "Bearer", tr.TokenType)

	var (
		sessID, org, user uuid.UUID
		host, ip          string
		ttl, abs          time.Duration
		refreshHash       []byte
	)
	require.NoError(t, s.pool.QueryRow(ctx, `SELECT id, org_id, user_id, hostname, host(ip), refresh_hash,
		expires_at - now(), absolute_expires_at - now() FROM cli_session`).Scan(&sessID, &org, &user, &host, &ip, &refreshHash, &ttl, &abs))
	assert.Equal(t, s.fx.OrgID, org)
	assert.Equal(t, s.fx.MemberID, user)
	assert.Equal(t, "laptop-1", host)
	assert.Equal(t, "127.0.0.1", ip)
	assert.InDelta(t, (7 * 24 * time.Hour).Seconds(), ttl.Seconds(), 60)
	assert.InDelta(t, (30 * 24 * time.Hour).Seconds(), abs.Seconds(), 60)
	assert.NotContains(t, string(refreshHash), tr.RefreshToken, "only the hash is stored")

	var (
		actor, target string
		data          map[string]any
	)
	require.NoError(t, s.pool.QueryRow(ctx, `SELECT actor_user_id::text, target_id, data FROM audit_event
		WHERE action = 'cli.login' AND org_id = $1`, s.fx.OrgID).Scan(&actor, &target, &data))
	assert.Equal(t, s.fx.MemberID.String(), actor)
	assert.Equal(t, sessID.String(), target)
	assert.Equal(t, map[string]any{"client_id": oauth.ClientCLI, "hostname": "laptop-1", "flow": "loopback"}, data)

	var rows int
	require.NoError(t, s.pool.QueryRow(ctx, "SELECT count(*) FROM oauth_auth_request WHERE id = $1", id).Scan(&rows))
	assert.Zero(t, rows, "the exchange consumes the request")
}

// TestCodeExchangeWrongVerifier: PKCE binds the code to the CLI that asked.
func TestCodeExchangeWrongVerifier(t *testing.T) {
	s := newStand(t)
	code, _ := login(t, s, s.fx.MemberID)
	other, _ := pkce()
	tr := exchangeCode(t, s.srv, code, other, testRedirect)
	assert.Equal(t, http.StatusBadRequest, tr.Status)
	assert.Equal(t, "invalid_grant", tr.Error)
	assert.Empty(t, tr.AccessToken)
	assert.Zero(t, countSessions(t, s))
}

// TestCodeSingleUse: of 10 parallel exchanges of one code exactly one wins;
// a replay is invalid_grant.
func TestCodeSingleUse(t *testing.T) {
	s := newStand(t)
	code, verifier := login(t, s, s.fx.MemberID)
	const n = 10
	results := make([]tokenResponse, n)
	var wg sync.WaitGroup
	for i := range n {
		wg.Go(func() { results[i] = exchangeCode(t, s.srv, code, verifier, testRedirect) })
	}
	wg.Wait()
	ok := 0
	for _, r := range results {
		if r.Status == http.StatusOK {
			ok++
			continue
		}
		assert.Equal(t, "invalid_grant", r.Error)
	}
	assert.Equal(t, 1, ok)
	assert.Equal(t, 1, countSessions(t, s))
	tr := exchangeCode(t, s.srv, code, verifier, testRedirect)
	assert.Equal(t, "invalid_grant", tr.Error)
	assert.Equal(t, 1, countSessions(t, s))
}

// TestCodeOnlyRedirectURI: the exchange names the redirect_uri of the request.
func TestCodeOnlyRedirectURI(t *testing.T) {
	s := newStand(t)
	code, verifier := login(t, s, s.fx.MemberID)
	tr := exchangeCode(t, s.srv, code, verifier, "http://127.0.0.1:1/callback")
	assert.Equal(t, "invalid_grant", tr.Error)
	assert.Zero(t, countSessions(t, s))
}

// TestTwoProvidersShareState: every step of the flow may land on another replica.
func TestTwoProvidersShareState(t *testing.T) {
	s := newStand(t)
	ctx := context.Background()
	b, srvB := serveProvider(t, s.pool, s.key, nil, s.srv.URL) // replica B: same issuer, own listener
	verifier, challenge := pkce()
	id := authorize(t, s.srv, challenge, nil)
	redirect, err := b.ApproveAuthRequest(ctx, cookieAs(s.fx, s.fx.MemberID), id)
	require.NoError(t, err)
	loc := callback(t, s.srv, redirect)
	tr := exchangeCode(t, srvB, loc.Query().Get("code"), verifier, testRedirect)
	require.Equal(t, http.StatusOK, tr.Status, tr.Error)
	claims, err := s.p.VerifyAccessToken(ctx, tr.AccessToken)
	require.NoError(t, err)
	assert.Equal(t, s.fx.MemberID, claims.UserID)
}

// TestTokenHasNoSecrets: the access token is a JWT of ids only.
func TestTokenHasNoSecrets(t *testing.T) {
	s := newStand(t)
	code, verifier := login(t, s, s.fx.MemberID)
	tr := exchangeCode(t, s.srv, code, verifier, testRedirect)
	require.Equal(t, http.StatusOK, tr.Status, tr.Error)
	payload := jwtPayload(t, tr.AccessToken)
	raw, err := json.Marshal(payload)
	require.NoError(t, err)
	assert.NotContains(t, string(raw), tr.RefreshToken)
	assert.NotContains(t, string(raw), code)
	assert.Equal(t, s.fx.OrgID.String(), payload["werbot_org"])
	assert.Equal(t, []any{oauth.Audience}, payload["aud"])
}
