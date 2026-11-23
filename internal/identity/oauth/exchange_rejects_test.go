package oauth_test

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"maps"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	jose "github.com/go-jose/go-jose/v4"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/identity/oauth"
	"github.com/werbot/werbot/internal/platform/apperr"
)

// TestTokenExchangeRejects: anything but a live, non-delegated sponsor token
// for exactly audience werbot-api and one scope agent:<name> gets an OAuth
// error, no token and no run.
func TestTokenExchangeRejects(t *testing.T) {
	ag := newAgents(time.Hour)
	s := newAgentStand(t, ag)
	subject := sponsorToken(t, s, s.fx.MemberID)
	agentToken := postToken(t, s, exchangeForm(subject), "")
	require.Equal(t, http.StatusOK, agentToken.Status, agentToken.Description)
	priv, ok := s.key.Key().(*ecdsa.PrivateKey)
	require.True(t, ok)
	handMade := func(extra map[string]any) string {
		m := map[string]any{
			"iss": s.srv.URL, "sub": s.fx.MemberID.String(), "aud": []string{oauth.Audience}, "jti": sessionID(t, s).String(),
			"werbot_org": s.fx.OrgID.String(), "exp": time.Now().Add(time.Minute).Unix(), "iat": time.Now().Unix(),
		}
		maps.Copy(m, extra)
		return sign(t, jose.ES256, priv, s.key.ID(), m)
	}
	require.Equal(t, http.StatusOK, postToken(t, s, exchangeForm(handMade(nil)), "").Status, "control: a hand-made sponsor token passes")
	calls := ag.count()

	set := func(k string, v ...string) func(url.Values) { return func(f url.Values) { f[k] = v } }
	cases := []struct {
		name, want string
		edit       func(url.Values)
	}{
		{"no audience", "invalid_target", set("audience")},
		{"other audience", "invalid_target", set("audience", "other")},
		{"two audiences", "invalid_target", set("audience", oauth.Audience, "other")},
		{"refresh token wanted", "invalid_request", set("requested_token_type", "urn:ietf:params:oauth:token-type:refresh_token")},
		{"id token wanted", "invalid_request", set("requested_token_type", "urn:ietf:params:oauth:token-type:id_token")},
		{"actor token", "invalid_request", func(f url.Values) {
			f.Set("actor_token", subject)
			f.Set("actor_token_type", "urn:ietf:params:oauth:token-type:access_token")
		}},
		{"subject as id token", "invalid_request", set("subject_token_type", "urn:ietf:params:oauth:token-type:id_token")},
		{"garbage subject", "invalid_request", set("subject_token", "not.a.jwt")},
		{"agent subject", "invalid_grant", set("subject_token", agentToken.AccessToken)},
		{"subject aud other", "invalid_grant", set("subject_token", handMade(map[string]any{"aud": []string{"other"}}))},
		{"subject with run", "invalid_grant", set("subject_token", handMade(map[string]any{"werbot_run": uuid.NewString()}))},
		{"subject jti not uuid", "invalid_grant", set("subject_token", handMade(map[string]any{"jti": "x"}))},
		{"subject without org", "invalid_grant", set("subject_token", handMade(map[string]any{"werbot_org": nil}))},
		{"subject other session", "invalid_grant", set("subject_token", handMade(map[string]any{"jti": uuid.NewString()}))},
		{"empty scope", "invalid_scope", set("scope", "")},
		{"no name", "invalid_scope", set("scope", "agent:")},
		{"bad name", "invalid_scope", set("scope", "agent:Bad")},
		{"extra scope", "invalid_scope", set("scope", "openid agent:x")},
		{"two agents", "invalid_scope", set("scope", "agent:a agent:b")},
		{"client too long", "invalid_request", set("werbot_client", strings.Repeat("a", 129))},
		{"reason too long", "invalid_request", set("werbot_reason", strings.Repeat("a", 1025))},
	}
	for _, tc := range cases {
		form := exchangeForm(subject)
		tc.edit(form)
		res := postToken(t, s, form, "")
		assert.Equal(t, tc.want, res.Error, "%s: %s", tc.name, res.Description)
		assert.True(t, res.AccessToken == "", tc.name)
	}
	assert.Equal(t, calls, ag.count(), "no refused exchange starts a run")

	ctx := context.Background()
	_, err := s.pool.Exec(ctx, "UPDATE cli_session SET revoked_at = now(), revoke_reason = 'logout' WHERE id = $1", sessionID(t, s))
	require.NoError(t, err)
	res := postToken(t, s, exchangeForm(subject), "")
	assert.Equal(t, "invalid_grant", res.Error, "revoked cli_session: %s", res.Description)

	other := sponsorToken(t, s, s.fx.OtherMemberID)
	_, err = s.pool.Exec(ctx, "DELETE FROM membership WHERE org_id = $1 AND user_id = $2", s.fx.OrgID, s.fx.OtherMemberID)
	require.NoError(t, err)
	res = postToken(t, s, exchangeForm(other), "")
	assert.Equal(t, "invalid_grant", res.Error, "removed member: %s", res.Description)
	assert.Equal(t, calls, ag.count(), "no run for an ended subject")
}

// TestTokenExchangeAgentRefused: an agent that may not start a run (frozen
// or locked) is invalid_grant; without Agents the grant is not served.
func TestTokenExchangeAgentRefused(t *testing.T) {
	ag := newAgents(time.Hour)
	ag.err = fmt.Errorf("agent frozen: %w", apperr.ErrPermissionDenied)
	s := newAgentStand(t, ag)
	res := postToken(t, s, exchangeForm(sponsorToken(t, s, s.fx.MemberID)), "")
	assert.Equal(t, "invalid_grant", res.Error)
	assert.Equal(t, "agent is frozen or locked", res.Description)
	assert.True(t, res.AccessToken == "")

	plain := newStand(t)
	res = postToken(t, plain, exchangeForm(sponsorToken(t, plain, plain.fx.MemberID)), "")
	assert.Equal(t, "unsupported_grant_type", res.Error)
	assert.True(t, res.AccessToken == "")
}
