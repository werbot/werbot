package agentic_test

import (
	"context"
	"errors"
	"testing"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
	"github.com/werbot/werbot/gen/werbot/v1/werbotv1connect"
	"github.com/werbot/werbot/internal/agentic"
	"github.com/werbot/werbot/internal/platform/db/dbtest"
	"github.com/werbot/werbot/internal/platform/principal"
)

// callThrough runs procedure through RestrictAgentTokens(svc) in ctx and
// reports the interceptor's error (nil: the handler ran).
func callThrough(ctx context.Context, svc *agentic.Service, procedure string) error {
	ran := errors.New("handler ran")
	next := connect.UnaryFunc(func(context.Context, connect.AnyRequest) (connect.AnyResponse, error) { return nil, ran })
	_, err := agentic.RestrictAgentTokens(svc).WrapUnary(next)(ctx, specRequest(procedure))
	if errors.Is(err, ran) {
		return nil
	}
	return err
}

// specRequest is a request whose Spec names procedure.
func specRequest(procedure string) connect.AnyRequest {
	return &procRequest{Request: connect.NewRequest(&werbotv1.ListAgentsRequest{}), proc: procedure}
}

type procRequest struct {
	*connect.Request[werbotv1.ListAgentsRequest]
	proc string
}

func (r *procRequest) Spec() connect.Spec { return connect.Spec{Procedure: r.proc} }

// TestAgentTokenProcedureAllowList: a run token reaches only the agent
// procedures (others PermissionDenied "not allowed for agent tokens");
// certificates, resources, access requests and tool calls also need an
// active run; a person's token is never restricted.
func TestAgentTokenProcedureAllowList(t *testing.T) {
	svc, pool, f := newTestService(t)
	agent := dbtest.Agent(t, pool, f.OrgID, f.AdminID, "claude")
	run := dbtest.AgentRun(t, pool, f.OrgID, agent, 10, 4)
	p := f.Principal(f.AdminID)
	p.AgentID, p.RunID = &agent, &run
	ctx := principal.With(bg, p)

	refused := []string{
		werbotv1connect.AccessRequestServiceApproveProcedure, werbotv1connect.LockServiceCreateLockProcedure,
		werbotv1connect.LockServiceDeleteLockProcedure, werbotv1connect.AgentServiceUpdateAgentProcedure,
		werbotv1connect.AgentServiceCreateAgentProcedure, werbotv1connect.AgentServiceResumeRunProcedure,
		werbotv1connect.PolicyServiceTestCommandProcedure, werbotv1connect.CLISessionServiceRevokeCLISessionProcedure,
		werbotv1connect.MemberServiceListMembersProcedure, "/werbot.v1.Unknown/Call",
	}
	for _, proc := range refused {
		err := callThrough(ctx, svc, proc)
		assert.Equal(t, connect.CodePermissionDenied, connect.CodeOf(err), proc)
		assert.ErrorContains(t, err, "not allowed for agent tokens", proc)
		assert.NoError(t, callThrough(as(f, f.AdminID), svc, proc), "a person's token: "+proc)
	}
	for proc := range agentic.AgentProcedures {
		assert.NoError(t, callThrough(ctx, svc, proc), proc)
	}
	assert.Len(t, agentic.AgentProcedures, 10)

	end(t, pool, run, "1 second")
	for _, proc := range []string{werbotv1connect.CertServiceIssueUserCertProcedure, werbotv1connect.ResourceServiceListAccessibleResourcesProcedure,
		werbotv1connect.AccessRequestServiceCreateRequestProcedure, werbotv1connect.AgentServiceRecordToolCallProcedure} {
		assert.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(callThrough(ctx, svc, proc)), "ended run: "+proc)
	}
	for _, proc := range []string{werbotv1connect.AgentServiceGetRunProcedure, werbotv1connect.AgentServiceEndRunProcedure,
		werbotv1connect.AgentServiceFinishToolCallProcedure, werbotv1connect.AccessRequestServiceListRequestsProcedure} {
		assert.NoError(t, callThrough(ctx, svc, proc), "an ended run still reads and ends itself: "+proc)
	}

	locked := dbtest.AgentRun(t, pool, f.OrgID, agent, 10, 4)
	f.Lock(t, pool, "agent", &agent)
	p.RunID = &locked
	err := callThrough(principal.With(bg, p), svc, werbotv1connect.CertServiceIssueUserCertProcedure)
	assert.Equal(t, connect.CodePermissionDenied, connect.CodeOf(err), "a locked agent")

	half := f.Principal(f.MemberID)
	half.AgentID = new(uuid.New())
	require.Equal(t, connect.CodePermissionDenied, connect.CodeOf(callThrough(principal.With(bg, half), svc,
		werbotv1connect.AgentServiceGetRunProcedure)), "an agent without a run")
}
