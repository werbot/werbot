package agentic_test

import (
	"context"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/structpb"

	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
	"github.com/werbot/werbot/internal/agentic"
	"github.com/werbot/werbot/internal/platform/db/dbtest"
	"github.com/werbot/werbot/internal/platform/principal"
)

// as is a request context of userID of f's organization.
func as(f dbtest.Fixture, userID uuid.UUID) context.Context {
	return principal.With(bg, f.Principal(userID))
}

func req[T any](m *T) *connect.Request[T] { return connect.NewRequest(m) }

// TestAgentRPCRoles: a member manages only own agents (another sponsor's is
// NotFound, creating for another sponsor PermissionDenied); owner and admin
// manage any; limits are checked; changes are audited.
func TestAgentRPCRoles(t *testing.T) {
	svc, pool, f := newTestService(t)
	h := agentic.NewHandler(svc)
	member, admin := as(f, f.MemberID), as(f, f.AdminID)

	created, err := h.CreateAgent(member, req(&werbotv1.CreateAgentRequest{Name: "claude",
		Limits: &werbotv1.AgentLimits{MaxActions: 20, MaxDuration: durationpb.New(10 * time.Minute)}}))
	require.NoError(t, err)
	a := created.Msg.GetAgent()
	assert.Equal(t, "claude", a.GetName())
	assert.Equal(t, f.MemberID.String(), a.GetSponsorUserId())
	assert.NotEmpty(t, a.GetSponsorLogin())
	assert.Equal(t, "active", a.GetStatus())
	assert.Equal(t, int32(20), a.GetLimits().GetMaxActions())
	assert.Equal(t, 10*time.Minute, a.GetLimits().GetMaxDuration().AsDuration())
	assert.Len(t, events(t, pool, "agent.create"), 1)
	_, err = h.CreateAgent(member, req(&werbotv1.CreateAgentRequest{Name: "claude"}))
	assert.Equal(t, connect.CodeAlreadyExists, connect.CodeOf(err))
	_, err = h.CreateAgent(member, req(&werbotv1.CreateAgentRequest{Name: "codex", SponsorUserId: f.OtherMemberID.String()}))
	assert.Equal(t, connect.CodePermissionDenied, connect.CodeOf(err), "a member creates only own agents")
	other, err := h.CreateAgent(admin, req(&werbotv1.CreateAgentRequest{Name: "codex", SponsorUserId: f.OtherMemberID.String()}))
	require.NoError(t, err, "an admin names any member")
	_, err = h.CreateAgent(admin, req(&werbotv1.CreateAgentRequest{Name: "x", SponsorUserId: dbtest.Seed(t, pool).MemberID.String()}))
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err), "a sponsor of another organization")

	list, err := h.ListAgents(member, req(&werbotv1.ListAgentsRequest{}))
	require.NoError(t, err)
	require.Len(t, list.Msg.GetAgents(), 1, "a member lists own agents")
	list, err = h.ListAgents(admin, req(&werbotv1.ListAgentsRequest{}))
	require.NoError(t, err)
	assert.Len(t, list.Msg.GetAgents(), 2, "an admin lists every agent")

	upd, err := h.UpdateAgent(member, req(&werbotv1.UpdateAgentRequest{Id: a.GetId(), Status: "frozen"}))
	require.NoError(t, err)
	assert.Equal(t, "frozen", upd.Msg.GetAgent().GetStatus())
	assert.Equal(t, int32(20), upd.Msg.GetAgent().GetLimits().GetMaxActions(), "limits unset: unchanged")
	upd, err = h.UpdateAgent(member, req(&werbotv1.UpdateAgentRequest{Id: a.GetId(), Limits: &werbotv1.AgentLimits{}}))
	require.NoError(t, err)
	assert.Zero(t, upd.Msg.GetAgent().GetLimits().GetMaxActions(), "0: the default")
	assert.Equal(t, "frozen", upd.Msg.GetAgent().GetStatus(), "status empty: unchanged")
	ev := events(t, pool, "agent.update")
	require.Len(t, ev, 2)
	assert.Equal(t, "frozen", ev[0].Data["status"])
	assert.Equal(t, &f.MemberID, ev[0].Actor)

	_, err = h.UpdateAgent(member, req(&werbotv1.UpdateAgentRequest{Id: other.Msg.GetAgent().GetId(), Status: "frozen"}))
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err), "another sponsor's agent")
	_, err = h.DeleteAgent(member, req(&werbotv1.DeleteAgentRequest{Id: other.Msg.GetAgent().GetId()}))
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err), "another sponsor's agent")
	_, err = h.UpdateAgent(admin, req(&werbotv1.UpdateAgentRequest{Id: a.GetId(), Status: "active"}))
	require.NoError(t, err, "an admin updates any agent")
	_, err = h.UpdateAgent(admin, req(&werbotv1.UpdateAgentRequest{Id: a.GetId(), Limits: &werbotv1.AgentLimits{MaxConcurrent: 33}}))
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
	_, err = h.UpdateAgent(admin, req(&werbotv1.UpdateAgentRequest{Id: a.GetId(), Status: "paused"}))
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
	_, err = h.ListAgents(bg, req(&werbotv1.ListAgentsRequest{}))
	assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}

// TestDeleteAgentCascades: deleting an agent deletes its runs and tool calls,
// keeps their sessions without the run and is audited.
func TestDeleteAgentCascades(t *testing.T) {
	svc, pool, f := newTestService(t)
	agent := dbtest.Agent(t, pool, f.OrgID, f.MemberID, "claude")
	run := dbtest.AgentRun(t, pool, f.OrgID, agent, 10, 4)
	_, err := svc.RecordToolCall(bg, agentPrincipal(f, agent, run), agentic.ToolCallInput{Tool: "ssh_exec"})
	require.NoError(t, err)
	session := f.StartSession(t, pool, f.MemberID)
	exec(t, pool, "UPDATE session SET agent_run_id = $2 WHERE id = $1", session, run)

	_, err = agentic.NewHandler(svc).DeleteAgent(as(f, f.MemberID), req(&werbotv1.DeleteAgentRequest{Id: agent.String()}))
	require.NoError(t, err)
	var runs, calls int
	var linked *uuid.UUID
	require.NoError(t, pool.QueryRow(bg, "SELECT (SELECT count(*) FROM agent_run), (SELECT count(*) FROM agent_tool_call), "+
		"(SELECT agent_run_id FROM session WHERE id = $1)", session).Scan(&runs, &calls, &linked))
	assert.Zero(t, runs)
	assert.Zero(t, calls)
	assert.Nil(t, linked, "the session stays, without its run")
	ev := events(t, pool, "agent.delete")
	require.Len(t, ev, 1)
	assert.Equal(t, agent.String(), ev[0].TargetID)
	assert.Equal(t, "claude", ev[0].Data["name"])
}

// TestEndResumeRunRPC: a person ends a run as ended_by_user, the run token
// as client_exit; a paused run resumes; a member never touches another
// sponsor's run.
func TestEndResumeRunRPC(t *testing.T) {
	svc, pool, f := newTestService(t)
	h := agentic.NewHandler(svc)
	agent := dbtest.Agent(t, pool, f.OrgID, f.MemberID, "claude")
	byUser, byAgent, paused := dbtest.AgentRun(t, pool, f.OrgID, agent, 10, 4),
		dbtest.AgentRun(t, pool, f.OrgID, agent, 10, 4), dbtest.AgentRun(t, pool, f.OrgID, agent, 10, 4)

	_, err := h.EndRun(as(f, f.OtherMemberID), req(&werbotv1.EndRunRequest{Id: byUser.String()}))
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err), "another sponsor's run")
	_, err = h.EndRun(as(f, f.MemberID), req(&werbotv1.EndRunRequest{Id: byUser.String()}))
	require.NoError(t, err)
	assert.Equal(t, "ended_by_user", *stateOf(t, pool, byUser).EndReason)
	_, err = h.EndRun(principal.With(bg, agentPrincipal(f, agent, byAgent)), req(&werbotv1.EndRunRequest{Id: byAgent.String()}))
	require.NoError(t, err)
	assert.Equal(t, "client_exit", *stateOf(t, pool, byAgent).EndReason)

	pause(t, pool, paused, 5)
	_, err = h.ResumeRun(as(f, f.OtherMemberID), req(&werbotv1.ResumeRunRequest{Id: paused.String()}))
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
	_, err = h.ResumeRun(as(f, f.MemberID), req(&werbotv1.ResumeRunRequest{Id: paused.String()}))
	require.NoError(t, err)
	assert.Equal(t, runState{Status: "running"}, stateOf(t, pool, paused))
}

// TestAgentTokenScopedToRun: a run token reads and ends only its own run,
// never another run of its sponsor; it never manages agents or lists runs.
func TestAgentTokenScopedToRun(t *testing.T) {
	svc, pool, f := newTestService(t)
	h := agentic.NewHandler(svc)
	agent := dbtest.Agent(t, pool, f.OrgID, f.AdminID, "claude")
	own, sibling := dbtest.AgentRun(t, pool, f.OrgID, agent, 10, 4), dbtest.AgentRun(t, pool, f.OrgID, agent, 10, 4)
	p := f.Principal(f.AdminID) // an admin's agent: the role never widens a run token
	p.AgentID, p.RunID = &agent, &own
	ctx := principal.With(bg, p)

	got, err := h.GetRun(ctx, req(&werbotv1.GetRunRequest{Id: own.String()}))
	require.NoError(t, err)
	assert.Equal(t, own.String(), got.Msg.GetRun().GetId())
	_, err = h.GetRun(ctx, req(&werbotv1.GetRunRequest{Id: sibling.String()}))
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err), "another run of the sponsor")
	_, err = h.EndRun(ctx, req(&werbotv1.EndRunRequest{Id: sibling.String()}))
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err), "another run of the sponsor")
	assert.Equal(t, "running", stateOf(t, pool, sibling).Status)

	_, err = h.ListAgents(ctx, req(&werbotv1.ListAgentsRequest{}))
	assert.Equal(t, connect.CodePermissionDenied, connect.CodeOf(err))
	_, err = h.ListRuns(ctx, req(&werbotv1.ListRunsRequest{}))
	assert.Equal(t, connect.CodePermissionDenied, connect.CodeOf(err))
	_, err = h.UpdateAgent(ctx, req(&werbotv1.UpdateAgentRequest{Id: agent.String(), Status: "active"}))
	assert.Equal(t, connect.CodePermissionDenied, connect.CodeOf(err))
	_, err = h.ResumeRun(ctx, req(&werbotv1.ResumeRunRequest{Id: own.String()}))
	assert.Equal(t, connect.CodePermissionDenied, connect.CodeOf(err))
}

// TestToolCallRPCRequiresAgentToken: tool calls are recorded only with a run
// token; arguments above 16 KiB of JSON are refused.
func TestToolCallRPCRequiresAgentToken(t *testing.T) {
	svc, pool, f := newTestService(t)
	h := agentic.NewHandler(svc)
	agent := dbtest.Agent(t, pool, f.OrgID, f.MemberID, "claude")
	run := dbtest.AgentRun(t, pool, f.OrgID, agent, 10, 4)

	_, err := h.RecordToolCall(as(f, f.MemberID), req(&werbotv1.RecordToolCallRequest{Tool: "ssh_exec"}))
	assert.Equal(t, connect.CodePermissionDenied, connect.CodeOf(err), "a person's session")
	_, err = h.FinishToolCall(as(f, f.MemberID), req(&werbotv1.FinishToolCallRequest{Id: uuid.NewString(), Status: "ok"}))
	assert.Equal(t, connect.CodePermissionDenied, connect.CodeOf(err), "a person's session")

	ctx := principal.With(bg, agentPrincipal(f, agent, run))
	args, err := structpb.NewStruct(map[string]any{"resource": "web", "command": "uptime"})
	require.NoError(t, err)
	rec, err := h.RecordToolCall(ctx, req(&werbotv1.RecordToolCallRequest{Tool: "ssh_exec", Arguments: args}))
	require.NoError(t, err)
	assert.Equal(t, map[string]any{"resource": "web", "command": "uptime"}, toolCallOf(t, pool, uuid.MustParse(rec.Msg.GetId())).Arguments)
	_, err = h.FinishToolCall(ctx, req(&werbotv1.FinishToolCallRequest{Id: rec.Msg.GetId(), Status: "ok"}))
	require.NoError(t, err)

	big, err := structpb.NewStruct(map[string]any{"command": string(make([]byte, agentic.MaxToolArguments))})
	require.NoError(t, err)
	_, err = h.RecordToolCall(ctx, req(&werbotv1.RecordToolCallRequest{Tool: "ssh_exec", Arguments: big}))
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err), "arguments above 16 KiB")
}
