package app

import (
	"context"
	"net/http"
	"net/url"
	"testing"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
	"github.com/werbot/werbot/gen/werbot/v1/werbotv1connect"
	"github.com/werbot/werbot/internal/identity"
	"github.com/werbot/werbot/internal/identity/oauth"
	"github.com/werbot/werbot/internal/platform/sshca"
)

// profiles is the AgentProfileService client of the owner's browser session.
func (r *agentRig) profiles() werbotv1connect.AgentProfileServiceClient {
	return werbotv1connect.NewAgentProfileServiceClient(r.web, r.base)
}

// newAgentProfile creates agent name and a profile of it; it returns the
// agent id, the profile and its refresh token.
func (r *agentRig) newAgentProfile(t *testing.T, name string) (string, *werbotv1.AgentProfile, string) {
	t.Helper()
	ctx := context.Background()
	a, err := r.agents().CreateAgent(ctx, connect.NewRequest(&werbotv1.CreateAgentRequest{Name: name}))
	require.NoError(t, err)
	id := a.Msg.GetAgent().GetId()
	p, err := r.profiles().CreateAgentProfile(ctx, connect.NewRequest(&werbotv1.CreateAgentProfileRequest{AgentId: id, Label: "box"}))
	require.NoError(t, err)
	return id, p.Msg.GetProfile(), p.Msg.GetRefreshToken()
}

// refreshProfile refreshes a profile: the access token, or the OAuth error.
func (r *agentRig) refreshProfile(t *testing.T, refresh string) (access, oauthErr string) {
	t.Helper()
	var out struct {
		AccessToken string `json:"access_token"`
		Error       string `json:"error"`
	}
	r.post(t, "/oauth/token", url.Values{"grant_type": {"refresh_token"}, "client_id": {oauth.ClientCLI}, "refresh_token": {refresh}}, false, &out)
	return out.AccessToken, out.Error
}

// exchangeAs exchanges subject (instead of the owner's token) for a run of agent name.
func (r *agentRig) exchangeAs(t *testing.T, subject, name string) (code int, token, oauthErr string) {
	t.Helper()
	as := *r
	as.token = subject
	return as.exchange(t, name)
}

// TestAgentProfileTokenOnlyExchanges: through the served middleware and
// interceptors a profile's access token opens no Connect procedure and no
// /api/ route (Unauthenticated); it exchanges for a run of its own agent.
func TestAgentProfileTokenOnlyExchanges(t *testing.T) {
	r := startAgentRig(t)
	ctx := context.Background()
	_, _, refresh := r.newAgentProfile(t, "ops")
	access, oauthErr := r.refreshProfile(t, refresh)
	require.Empty(t, oauthErr)
	id := uuid.NewString()
	hc := http.DefaultClient

	refused := map[string]error{}
	_, refused["ListAccessibleResources"] = werbotv1connect.NewResourceServiceClient(hc, r.base).ListAccessibleResources(ctx,
		withBearer(&werbotv1.ListAccessibleResourcesRequest{}, access))
	refused["IssueUserCert"] = r.issueCert(t, access)
	_, refused["ListAgentProfiles"] = werbotv1connect.NewAgentProfileServiceClient(hc, r.base).ListAgentProfiles(ctx,
		withBearer(&werbotv1.ListAgentProfilesRequest{}, access))
	_, refused["CreateAgentProfile"] = werbotv1connect.NewAgentProfileServiceClient(hc, r.base).CreateAgentProfile(ctx,
		withBearer(&werbotv1.CreateAgentProfileRequest{AgentId: id}, access))
	_, refused["RevokeAgentProfile"] = werbotv1connect.NewAgentProfileServiceClient(hc, r.base).RevokeAgentProfile(ctx,
		withBearer(&werbotv1.RevokeAgentProfileRequest{Id: id}, access))
	_, refused["ListCLISessions"] = werbotv1connect.NewCLISessionServiceClient(hc, r.base).ListCLISessions(ctx,
		withBearer(&werbotv1.ListCLISessionsRequest{}, access))
	_, refused["ListAgents"] = werbotv1connect.NewAgentServiceClient(hc, r.base).ListAgents(ctx, withBearer(&werbotv1.ListAgentsRequest{}, access))
	for name, err := range refused {
		assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err), name)
	}
	req, err := http.NewRequest(http.MethodGet, r.base+"/api/recordings/"+id+".cast", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+access)
	res, err := hc.Do(req)
	require.NoError(t, err)
	_ = res.Body.Close()
	assert.Equal(t, http.StatusUnauthorized, res.StatusCode, "recordings")

	code, _, oauthErr := r.exchangeAs(t, access, "claude")
	assert.Equal(t, http.StatusBadRequest, code)
	assert.Equal(t, "invalid_scope", oauthErr, "another agent")
	code, run, oauthErr := r.exchangeAs(t, access, "ops")
	require.Equal(t, http.StatusOK, code, oauthErr)
	require.NoError(t, r.issueCert(t, run), "the run of the profile's agent gets a certificate")
}

// TestFreezeRevokesAgentProfiles: freezing an agent revokes every live
// profile of it (agent_frozen, one agent.profile.revoke each) and no other;
// unfreezing restores none.
func TestFreezeRevokesAgentProfiles(t *testing.T) {
	r := startAgentRig(t)
	ctx := context.Background()
	ops, _, refresh := r.newAgentProfile(t, "ops")
	_, err := r.profiles().CreateAgentProfile(ctx, connect.NewRequest(&werbotv1.CreateAgentProfileRequest{AgentId: ops}))
	require.NoError(t, err)
	_, other, _ := r.newAgentProfile(t, "other")

	_, err = r.agents().UpdateAgent(ctx, connect.NewRequest(&werbotv1.UpdateAgentRequest{Id: ops, Status: "frozen"}))
	require.NoError(t, err)
	list, err := r.profiles().ListAgentProfiles(ctx, connect.NewRequest(&werbotv1.ListAgentProfilesRequest{}))
	require.NoError(t, err)
	require.Len(t, list.Msg.GetProfiles(), 1)
	assert.Equal(t, other.GetId(), list.Msg.GetProfiles()[0].GetId(), "another agent's profile stays")
	var n int
	require.NoError(t, r.pool.QueryRow(ctx, `SELECT count(*) FROM audit_event WHERE action = 'agent.profile.revoke'
		AND data->>'reason' = 'agent_frozen' AND data->>'agent_id' = $1`, ops).Scan(&n))
	assert.Equal(t, 2, n)
	_, oauthErr := r.refreshProfile(t, refresh)
	assert.Equal(t, "invalid_grant", oauthErr)

	_, err = r.agents().UpdateAgent(ctx, connect.NewRequest(&werbotv1.UpdateAgentRequest{Id: ops, Status: "active"}))
	require.NoError(t, err)
	list, err = r.profiles().ListAgentProfiles(ctx, connect.NewRequest(&werbotv1.ListAgentProfilesRequest{AgentId: ops}))
	require.NoError(t, err)
	assert.Empty(t, list.Msg.GetProfiles(), "unfreezing restores none")
}

// TestAgentProfileRunTokenFailsClosed: once its profile is revoked, a run
// token from it is Unauthenticated and the SSH certificate of its run no
// longer authenticates.
func TestAgentProfileRunTokenFailsClosed(t *testing.T) {
	r := startAgentRig(t)
	ctx := context.Background()
	_, prof, refresh := r.newAgentProfile(t, "ops")
	access, _ := r.refreshProfile(t, refresh)
	code, run, oauthErr := r.exchangeAs(t, access, "ops")
	require.Equal(t, http.StatusOK, code, oauthErr)
	require.NoError(t, r.issueCert(t, run))

	sess, err := werbotv1connect.NewAuthServiceClient(r.web, r.base).GetSession(ctx, connect.NewRequest(&werbotv1.GetSessionRequest{}))
	require.NoError(t, err)
	org, user := uuid.MustParse(sess.Msg.GetOrganization().GetId()), uuid.MustParse(sess.Msg.GetUser().GetId())
	cert := sshca.Identity{OrgID: org, UserID: user, CLISessionID: uuid.MustParse(prof.GetId()), Login: sess.Msg.GetUser().GetLogin()}
	ids := identity.New(r.pool, nil, identity.Config{})
	_, err = ids.AuthenticateCert(ctx, org, cert)
	require.NoError(t, err, "control: the run's certificate authenticates")

	_, err = r.profiles().RevokeAgentProfile(ctx, connect.NewRequest(&werbotv1.RevokeAgentProfileRequest{Id: prof.GetId()}))
	require.NoError(t, err)
	assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(r.issueCert(t, run)), "run token of a revoked profile")
	_, err = ids.AuthenticateCert(ctx, org, cert)
	assert.Error(t, err, "certificate of a revoked profile")
	code, _, _ = r.exchangeAs(t, access, "ops")
	assert.Equal(t, http.StatusBadRequest, code, "no new run")
}
