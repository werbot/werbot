package agentic

import (
	"context"
	"errors"

	"connectrpc.com/connect"

	"github.com/werbot/werbot/gen/werbot/v1/werbotv1connect"
	"github.com/werbot/werbot/internal/platform/apperr"
	"github.com/werbot/werbot/internal/platform/principal"
)

// AgentProcedures are the only procedures a run token may call: a
// certificate, its resources, access requests of its run, its tool calls and
// its own run. An agent never approves, unlocks, changes limits or resumes.
var AgentProcedures = map[string]bool{
	werbotv1connect.CertServiceIssueUserCertProcedure:               true,
	werbotv1connect.ResourceServiceListAccessibleResourcesProcedure: true,
	werbotv1connect.AccessRequestServiceCreateRequestProcedure:      true,
	werbotv1connect.AccessRequestServiceGetRequestProcedure:         true,
	werbotv1connect.AccessRequestServiceListRequestsProcedure:       true,
	werbotv1connect.AccessRequestServiceCancelProcedure:             true,
	werbotv1connect.AgentServiceRecordToolCallProcedure:             true,
	werbotv1connect.AgentServiceFinishToolCallProcedure:             true,
	werbotv1connect.AgentServiceGetRunProcedure:                     true,
	werbotv1connect.AgentServiceEndRunProcedure:                     true,
}

// activeRunProcedures act for the run: they also need it active (CheckRun).
var activeRunProcedures = map[string]bool{
	werbotv1connect.CertServiceIssueUserCertProcedure:               true,
	werbotv1connect.ResourceServiceListAccessibleResourcesProcedure: true,
	werbotv1connect.AccessRequestServiceCreateRequestProcedure:      true,
	werbotv1connect.AgentServiceRecordToolCallProcedure:             true,
}

var errNotForAgents = errors.New("not allowed for agent tokens")

// restrict is the RestrictAgentTokens interceptor.
type restrict struct{ s *Service }

// RestrictAgentTokens lets a run token (principal with AgentID) call only
// AgentProcedures, those of activeRunProcedures only while CheckRun passes;
// any other procedure, streaming included, is PermissionDenied "not allowed
// for agent tokens". Other principals pass unchanged.
func RestrictAgentTokens(s *Service) connect.Interceptor { return restrict{s: s} }

func (r restrict) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		if err := r.admit(ctx, req.Spec().Procedure); err != nil {
			return nil, err
		}
		return next(ctx, req)
	}
}

func (restrict) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	return next
}

func (r restrict) WrapStreamingHandler(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	return func(ctx context.Context, conn connect.StreamingHandlerConn) error {
		if err := r.admit(ctx, ""); err != nil { // no streaming procedure is an agent procedure
			return err
		}
		return next(ctx, conn)
	}
}

// admit refuses procedure to a run token outside AgentProcedures, or with
// an inactive run where activeRunProcedures need one.
func (r restrict) admit(ctx context.Context, procedure string) error {
	p, ok := principal.From(ctx)
	switch {
	case !ok || (p.AgentID == nil && p.RunID == nil):
		return nil
	case p.AgentID == nil || p.RunID == nil || !AgentProcedures[procedure]:
		return connect.NewError(connect.CodePermissionDenied, errNotForAgents)
	case activeRunProcedures[procedure]:
		_, err := r.s.CheckRun(ctx, p.OrgID, *p.RunID)
		return apperr.ToConnect(err)
	}
	return nil
}
