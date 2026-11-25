package app

import (
	"bytes"
	"context"
	"net/http"
	"path/filepath"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
	"github.com/werbot/werbot/gen/werbot/v1/werbotv1connect"
	"github.com/werbot/werbot/internal/clientauth"
	"github.com/werbot/werbot/internal/mcpserver"
)

// agentProfileDir runs werbot agents profile create for agent e2e as the
// sponsor (home) into a new directory and loads the profile from it, as
// WERBOT_HOME=<dir> werbot mcp does.
func agentProfileDir(t *testing.T, home string) *clientauth.Profile {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "agent")
	require.NoError(t, Run(context.Background(), []string{"agents", "profile", "create", "e2e", "--dir", dir},
		env(map[string]string{"WERBOT_HOME": home}), &bytes.Buffer{}))
	p, err := clientauth.Current(dir, env(nil))
	require.NoError(t, err)
	require.Equal(t, "e2e", p.Agent)
	return p
}

// TestE2EAgentProfileIsolated: an agent profile directory starts runs of its
// one agent through werbot mcp; its own token opens no API; once revoked,
// the live run token and the profile fail closed; freezing the agent revokes
// its other profiles.
func TestE2EAgentProfileIsolated(t *testing.T) {
	s, _ := cliStack(t)
	s.keyReady(t)
	home, _ := s.cliLogin(t, false)
	ap := agentProfileDir(t, home)
	ctx := context.Background()

	m := s.startMCP(t, mcpserver.Options{Profile: ap})
	assert.Equal(t, "ok", m.exec(t, "uname").Status)
	var sessions int
	require.NoError(t, s.pool.QueryRow(ctx, "SELECT count(*) FROM session WHERE agent_run_id = $1", s.agentRun(t)).Scan(&sessions))
	assert.Equal(t, 1, sessions)

	token, err := ap.Token(ctx)
	require.NoError(t, err)
	_, err = werbotv1connect.NewResourceServiceClient(http.DefaultClient, s.url).ListAccessibleResources(ctx,
		withBearer(&werbotv1.ListAccessibleResourcesRequest{}, token))
	assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err), "the profile token is for the exchange only")

	profiles := werbotv1connect.NewAgentProfileServiceClient(s.aliceHTTP(t), s.url)
	list, err := profiles.ListAgentProfiles(ctx, connect.NewRequest(&werbotv1.ListAgentProfilesRequest{}))
	require.NoError(t, err)
	require.Len(t, list.Msg.GetProfiles(), 1)
	require.NoError(t, Run(ctx, []string{"agents", "profile", "revoke", list.Msg.GetProfiles()[0].GetId()},
		env(map[string]string{"WERBOT_HOME": home}), &bytes.Buffer{}))
	assert.Contains(t, m.execError(t), "unauthenticated", "the live run token of a revoked profile")
	assert.Contains(t, s.startMCP(t, mcpserver.Options{Profile: ap}).execError(t), "agent token exchange",
		"a revoked profile starts no run")

	agentProfileDir(t, home)
	_, err = s.agentsAPI(t).UpdateAgent(ctx, connect.NewRequest(&werbotv1.UpdateAgentRequest{
		Id: list.Msg.GetProfiles()[0].GetAgentId(), Status: "frozen"}))
	require.NoError(t, err)
	var revoked int
	require.NoError(t, s.pool.QueryRow(ctx, `SELECT count(*) FROM audit_event WHERE action = 'agent.profile.revoke'
		AND data->>'reason' = 'agent_frozen'`).Scan(&revoked))
	assert.Equal(t, 1, revoked, "freezing the agent revokes its live profile")
}
