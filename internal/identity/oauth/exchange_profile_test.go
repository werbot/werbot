package oauth_test

import (
	"context"
	"crypto/ecdsa"
	"net/http"
	"net/url"
	"testing"
	"time"

	jose "github.com/go-jose/go-jose/v4"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/identity/oauth"
	"github.com/werbot/werbot/internal/platform/apperr"
	"github.com/werbot/werbot/internal/platform/db/dbtest"
)

// TestAgentProfileRefreshRotates: a profile's refresh rotates its refresh
// token and gives an access token for the exchange only (aud
// werbot-agent-exchange, jti the profile) that VerifyAccessToken refuses; a
// person's session keeps aud werbot-api.
func TestAgentProfileRefreshRotates(t *testing.T) {
	s := newStand(t)
	prof, token := newProfile(t, s, dbtest.Agent(t, s.pool, s.fx.OrgID, s.fx.MemberID, "ops"))

	tr := refresh(t, s, token)
	require.Equal(t, http.StatusOK, tr.Status, tr.Error)
	assert.NotEqual(t, token, tr.RefreshToken)
	claims := jwtPayload(t, tr.AccessToken)
	assert.Equal(t, []any{oauth.AudienceAgentExchange}, claims["aud"])
	assert.Equal(t, prof.ID.String(), claims["jti"])
	assert.Equal(t, s.fx.OrgID.String(), claims["werbot_org"])
	_, err := s.p.VerifyAccessToken(context.Background(), tr.AccessToken)
	require.ErrorIs(t, err, apperr.ErrUnauthenticated, "no procedure accepts a profile token")

	human := newSession(t, s)
	assert.Equal(t, []any{oauth.Audience}, jwtPayload(t, refresh(t, s, human.RefreshToken).AccessToken)["aud"])
}

// TestAgentProfileRefreshReuse: a rotated refresh token coming back revokes
// the profile (refresh_reuse): its newest refresh token and its access token
// stop working.
func TestAgentProfileRefreshReuse(t *testing.T) {
	ag := newAgents(time.Hour)
	s := newAgentStand(t, ag)
	prof, token := newProfile(t, s, dbtest.Agent(t, s.pool, s.fx.OrgID, s.fx.MemberID, "ops"))
	tr := refresh(t, s, token)
	require.Equal(t, http.StatusOK, tr.Status, tr.Error)

	requireInvalidGrant(t, refresh(t, s, token))
	var reason string
	require.NoError(t, s.pool.QueryRow(context.Background(), "SELECT revoke_reason FROM cli_session WHERE id = $1", prof.ID).Scan(&reason))
	assert.Equal(t, "refresh_reuse", reason)
	requireInvalidGrant(t, refresh(t, s, tr.RefreshToken))
	assert.Equal(t, "invalid_grant", postToken(t, s, agentForm(tr.AccessToken, "ops"), "").Error)
	assert.Zero(t, ag.count())
}

// agentForm is the exchange form of subject for a run of agent name.
func agentForm(subject, name string) url.Values {
	f := exchangeForm(subject)
	f.Set("scope", oauth.AgentScopePrefix+name)
	return f
}

// TestAgentProfileExchangeOtherAgent: a profile exchanges only for its own
// agent (another name is invalid_scope, no run); the subject's aud must
// match whether its session is a profile (invalid_grant otherwise).
func TestAgentProfileExchangeOtherAgent(t *testing.T) {
	ag := newAgents(time.Hour)
	s := newAgentStand(t, ag)
	prof, token := newProfile(t, s, dbtest.Agent(t, s.pool, s.fx.OrgID, s.fx.MemberID, "ops"))
	subject := refresh(t, s, token).AccessToken

	res := postToken(t, s, agentForm(subject, "claude"), "")
	assert.Equal(t, "invalid_scope", res.Error)
	assert.Equal(t, "agent profile is bound to another agent", res.Description)
	assert.Empty(t, res.AccessToken)
	assert.Zero(t, ag.count(), "no run for another agent")

	human := newSession(t, s)
	humanID := sessionOf(t, human.AccessToken)
	priv, ok := s.key.Key().(*ecdsa.PrivateKey)
	require.True(t, ok)
	handMade := func(aud []string, jti uuid.UUID) string {
		return sign(t, jose.ES256, priv, s.key.ID(), map[string]any{
			"iss": s.srv.URL, "sub": s.fx.MemberID.String(), "aud": aud, "jti": jti.String(),
			"werbot_org": s.fx.OrgID.String(), "exp": time.Now().Add(time.Minute).Unix(), "iat": time.Now().Unix(),
		})
	}
	for name, subject := range map[string]string{
		"werbot-api on a profile":     handMade([]string{oauth.Audience}, prof.ID),
		"agent exchange on a person":  handMade([]string{oauth.AudienceAgentExchange}, humanID),
		"both audiences on a profile": handMade([]string{oauth.Audience, oauth.AudienceAgentExchange}, prof.ID),
		"no audience on a profile":    handMade([]string{}, prof.ID),
	} {
		res := postToken(t, s, agentForm(subject, "ops"), "")
		assert.Equal(t, "invalid_grant", res.Error, name)
		assert.Empty(t, res.AccessToken, name)
	}
	assert.Zero(t, ag.count(), "no refused exchange starts a run")

	res = postToken(t, s, agentForm(subject, "ops"), "")
	require.Equal(t, http.StatusOK, res.Status, res.Description)
	assert.Empty(t, res.RefreshToken, "the exchange never issues a refresh token")
	assert.Equal(t, prof.ID.String(), jwtPayload(t, res.AccessToken)["jti"], "the run token's session is the profile")
	assert.Equal(t, "ops", ag.calls[0].name)

	res = postToken(t, s, agentForm(handMade([]string{oauth.Audience, oauth.ClientCLI}, humanID), "ops"), "")
	assert.Equal(t, http.StatusOK, res.Status, "a person's device-flow token (aud werbot-api and the client): %s", res.Description)
}

// sessionOf is the cli_session (jti) of an access token.
func sessionOf(t *testing.T, token string) uuid.UUID {
	t.Helper()
	id, err := uuid.Parse(jwtPayload(t, token)["jti"].(string))
	require.NoError(t, err)
	return id
}

// TestAgentProfileFailsClosed: a revoked or expired profile, one of a
// deleted or frozen agent and one of a sponsor no longer a member neither
// refreshes nor exchanges (invalid_grant), and no run starts.
func TestAgentProfileFailsClosed(t *testing.T) {
	ag := newAgents(time.Hour)
	s := newAgentStand(t, ag)
	ctx := context.Background()
	cases := map[string]func(prof oauth.AgentProfile){
		"revoked": func(prof oauth.AgentProfile) {
			require.NoError(t, s.p.RevokeAgentProfile(ctx, s.fx.Principal(s.fx.MemberID), prof.ID))
		},
		"expired": func(prof oauth.AgentProfile) {
			exec(t, s, "UPDATE cli_session SET absolute_expires_at = now() - interval '1 second' WHERE id = $1", prof.ID)
		},
		"agent deleted": func(prof oauth.AgentProfile) { exec(t, s, "DELETE FROM agent WHERE id = $1", prof.AgentID) },
		"agent frozen": func(prof oauth.AgentProfile) {
			tx, err := s.pool.Begin(ctx)
			require.NoError(t, err)
			_, err = tx.Exec(ctx, "UPDATE agent SET status = 'frozen' WHERE id = $1", prof.AgentID)
			require.NoError(t, err)
			require.NoError(t, oauth.RevokeAgentProfiles(ctx, tx, s.fx.OrgID, prof.AgentID, nil, "agent_frozen"))
			require.NoError(t, tx.Commit(ctx))
		},
		"sponsor removed": func(oauth.AgentProfile) {
			exec(t, s, "DELETE FROM membership WHERE org_id = $1 AND user_id = $2", s.fx.OrgID, s.fx.MemberID)
		},
	}
	for _, name := range []string{"revoked", "expired", "agent deleted", "agent frozen", "sponsor removed"} {
		prof, token := newProfile(t, s, dbtest.Agent(t, s.pool, s.fx.OrgID, s.fx.MemberID, "a"+uuid.NewString()[:8]))
		tr := refresh(t, s, token)
		require.Equal(t, http.StatusOK, tr.Status, name)
		cases[name](prof)
		requireInvalidGrant(t, refresh(t, s, tr.RefreshToken))
		assert.Equal(t, "invalid_grant", postToken(t, s, agentForm(tr.AccessToken, prof.AgentName), "").Error, name)
	}
	assert.Zero(t, ag.count())
	var n int
	require.NoError(t, s.pool.QueryRow(ctx, `SELECT count(*) FROM audit_event WHERE action = 'agent.profile.revoke'
		AND actor_kind = 'system' AND data->>'reason' = 'agent_frozen'`).Scan(&n))
	assert.Equal(t, 1, n)
}
