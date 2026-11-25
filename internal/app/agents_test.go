package app

import (
	"context"
	"log/slog"
	"net/http"
	"testing"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
	"github.com/werbot/werbot/gen/werbot/v1/werbotv1connect"
	"github.com/werbot/werbot/internal/agentic"
	"github.com/werbot/werbot/internal/platform/apperr"
	"github.com/werbot/werbot/internal/platform/db/dbtest"
)

// TestAgentTokenRestricted: through the served middleware and interceptors a
// run token calls only the agent procedures (others PermissionDenied "not
// allowed for agent tokens") and no route outside Connect (403); it gets a
// certificate only while its run is active and its agent unlocked; the
// owner's own token is unchanged.
func TestAgentTokenRestricted(t *testing.T) {
	r := startAgentRig(t)
	ctx := context.Background()
	run := r.runToken(t, "claude")
	id := uuid.NewString()
	hc := http.DefaultClient
	refused := map[string]error{}
	_, refused["Approve"] = werbotv1connect.NewAccessRequestServiceClient(hc, r.base).Approve(ctx, withBearer(&werbotv1.ApproveRequest{Id: id}, run))
	_, refused["CreateLock"] = werbotv1connect.NewLockServiceClient(hc, r.base).CreateLock(ctx,
		withBearer(&werbotv1.CreateLockRequest{TargetType: "all_agents", Reason: "x"}, run))
	_, refused["DeleteLock"] = werbotv1connect.NewLockServiceClient(hc, r.base).DeleteLock(ctx, withBearer(&werbotv1.DeleteLockRequest{Id: id}, run))
	_, refused["UpdateAgent"] = werbotv1connect.NewAgentServiceClient(hc, r.base).UpdateAgent(ctx,
		withBearer(&werbotv1.UpdateAgentRequest{Id: id, Limits: &werbotv1.AgentLimits{MaxActions: 10000}}, run))
	_, refused["CreateAgent"] = werbotv1connect.NewAgentServiceClient(hc, r.base).CreateAgent(ctx, withBearer(&werbotv1.CreateAgentRequest{Name: "x"}, run))
	_, refused["ResumeRun"] = werbotv1connect.NewAgentServiceClient(hc, r.base).ResumeRun(ctx, withBearer(&werbotv1.ResumeRunRequest{Id: id}, run))
	_, refused["TestCommand"] = werbotv1connect.NewPolicyServiceClient(hc, r.base).TestCommand(ctx,
		withBearer(&werbotv1.TestCommandRequest{Subject: &werbotv1.TestCommandRequest_UserId{UserId: id}, ResourceId: id, Command: "id"}, run))
	_, refused["RevokeCLISession"] = werbotv1connect.NewCLISessionServiceClient(hc, r.base).RevokeCLISession(ctx,
		withBearer(&werbotv1.RevokeCLISessionRequest{Id: id}, run))
	_, refused["ListMembers"] = werbotv1connect.NewMemberServiceClient(hc, r.base).ListMembers(ctx, withBearer(&werbotv1.ListMembersRequest{}, run))
	for name, err := range refused {
		assert.Equal(t, connect.CodePermissionDenied, connect.CodeOf(err), name)
		assert.ErrorContains(t, err, "not allowed for agent tokens", name)
	}
	_, err := werbotv1connect.NewMemberServiceClient(hc, r.base).ListMembers(ctx, withBearer(&werbotv1.ListMembersRequest{}, r.token))
	require.NoError(t, err, "the owner's own token is not restricted")

	for _, path := range []string{"/api/recordings/" + id + ".cast", "/"} {
		req, err := http.NewRequest(http.MethodGet, r.base+path, nil)
		require.NoError(t, err)
		req.Header.Set("Authorization", "Bearer "+run)
		res, err := hc.Do(req)
		require.NoError(t, err)
		_ = res.Body.Close()
		assert.Equal(t, http.StatusForbidden, res.StatusCode, path)
	}

	require.NoError(t, r.issueCert(t, run), "a running run gets a certificate")
	runs, err := r.agents().ListRuns(ctx, connect.NewRequest(&werbotv1.ListRunsRequest{}))
	require.NoError(t, err)
	require.Len(t, runs.Msg.GetRuns(), 1)
	_, err = werbotv1connect.NewAgentServiceClient(hc, r.base).EndRun(ctx, withBearer(&werbotv1.EndRunRequest{Id: runs.Msg.GetRuns()[0].GetId()}, run))
	require.NoError(t, err)
	assert.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(r.issueCert(t, run)), "an ended run")

	next := r.runToken(t, "claude")
	_, err = werbotv1connect.NewLockServiceClient(r.web, r.base).CreateLock(ctx, connect.NewRequest(&werbotv1.CreateLockRequest{
		TargetType: "agent", TargetId: runs.Msg.GetRuns()[0].GetAgentId(), Reason: "incident"}))
	require.NoError(t, err)
	assert.Equal(t, connect.CodePermissionDenied, connect.CodeOf(r.issueCert(t, next)), "a locked agent")
	code, token, oauthErr := r.exchange(t, "claude")
	assert.Equal(t, http.StatusBadRequest, code)
	assert.Equal(t, "invalid_grant", oauthErr, "a locked agent gets no run")
	assert.Empty(t, token)
	require.NoError(t, r.issueCert(t, r.token), "the owner still gets a certificate")
}

// TestServeMountsAgentService: AgentService is served to members and the
// agent_run.expire worker is registered.
func TestServeMountsAgentService(t *testing.T) {
	r := startAgentRig(t)
	created, err := r.agents().CreateAgent(context.Background(), connect.NewRequest(&werbotv1.CreateAgentRequest{Name: "claude"}))
	require.NoError(t, err)
	list, err := r.agents().ListAgents(context.Background(), connect.NewRequest(&werbotv1.ListAgentsRequest{}))
	require.NoError(t, err)
	require.Len(t, list.Msg.GetAgents(), 1)
	assert.Equal(t, created.Msg.GetAgent().GetId(), list.Msg.GetAgents()[0].GetId())

	s := newStage4(t, slog.New(slog.DiscardHandler))
	_, err = s.d.Jobs.Insert(context.Background(), agentic.ExpireArgs{}, nil)
	require.NoError(t, err, "agent_run.expire has a worker")
}

// TestAccessibleResourcesAgentRPC: a run token lists only resources its
// sponsor reaches too, with the classes the agent may run there.
func TestAccessibleResourcesAgentRPC(t *testing.T) {
	r := startAgentRig(t)
	ctx := context.Background()
	gws := werbotv1connect.NewGatewayServiceClient(r.web, r.base)
	gw, err := gws.CreateGateway(ctx, connect.NewRequest(&werbotv1.CreateGatewayRequest{Name: "gw"}))
	require.NoError(t, err)
	res := werbotv1connect.NewResourceServiceClient(r.web, r.base)
	create := func(name string) string {
		out, err := res.CreateResource(ctx, connect.NewRequest(&werbotv1.CreateResourceRequest{Name: name, Type: "ssh",
			GatewayId: gw.Msg.GetGateway().GetId(), Config: sshConfig(t)}))
		require.NoError(t, err)
		return out.Msg.GetResource().GetId()
	}
	bound := create("bound")
	create("unbound")
	sess, err := werbotv1connect.NewAuthServiceClient(r.web, r.base).GetSession(ctx, connect.NewRequest(&werbotv1.GetSessionRequest{}))
	require.NoError(t, err)
	_, err = werbotv1connect.NewAccessServiceClient(r.web, r.base).CreateBinding(ctx, connect.NewRequest(&werbotv1.CreateBindingRequest{
		SubjectType: "user", SubjectId: sess.Msg.GetUser().GetId(), ResourceId: bound}))
	require.NoError(t, err)

	list, err := res.ListAccessibleResources(ctx, withBearer(&werbotv1.ListAccessibleResourcesRequest{}, r.runToken(t, "claude")))
	require.NoError(t, err)
	require.Len(t, list.Msg.GetResources(), 1, "a resource the sponsor cannot reach is not listed")
	got := list.Msg.GetResources()[0]
	assert.Equal(t, bound, got.GetId())
	assert.True(t, got.GetAllowed())
	assert.Equal(t, []string{"read"}, got.GetAllowedClasses(), "the agent baseline")
	assert.Empty(t, got.GetApprovalClasses())

	human, err := res.ListAccessibleResources(ctx, withBearer(&werbotv1.ListAccessibleResourcesRequest{}, r.token))
	require.NoError(t, err)
	require.Len(t, human.Msg.GetResources(), 1)
	assert.Empty(t, human.Msg.GetResources()[0].GetAllowedClasses(), "a person's list has no classes")
}

// TestOAuthAgentsStartRun: the exchange adapter starts a run of the
// sponsor's agent; a frozen agent is ErrPermissionDenied (invalid_grant),
// any other failure an error without its cause.
func TestOAuthAgentsStartRun(t *testing.T) {
	pool, _ := dbtest.New(t)
	f := dbtest.Seed(t, pool)
	a := oauthAgents{agents: agentic.New(pool, &dbtest.Mails{}, agentic.Config{}), logger: slog.New(slog.DiscardHandler)}

	agentID, runID, expires, err := a.StartAgentRun(context.Background(), f.OrgID, f.MemberID, "claude", "claude-code", "")
	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, runID)
	assert.False(t, expires.IsZero())
	_, err = pool.Exec(context.Background(), "UPDATE agent SET status = 'frozen' WHERE id = $1", agentID)
	require.NoError(t, err)
	_, _, _, err = a.StartAgentRun(context.Background(), f.OrgID, f.MemberID, "claude", "claude-code", "")
	require.ErrorIs(t, err, apperr.ErrPermissionDenied)

	_, _, _, err = a.StartAgentRun(context.Background(), f.OrgID, f.MemberID, "codex", "", "")
	require.ErrorIs(t, err, errRunNotStarted, "an invalid client")
	assert.NotErrorIs(t, err, apperr.ErrInvalidArgument, "the cause is not passed on")
}
