package agentic

import (
	"context"

	"connectrpc.com/connect"
	"github.com/google/uuid"

	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
	"github.com/werbot/werbot/gen/werbot/v1/werbotv1connect"
	"github.com/werbot/werbot/internal/platform/apperr"
	"github.com/werbot/werbot/internal/platform/principal"
)

// Handler serves AgentService; roles and run scoping are in the service.
type Handler struct {
	werbotv1connect.UnimplementedAgentServiceHandler
	svc *Service
}

// NewHandler returns the Connect handler for AgentService.
func NewHandler(s *Service) *Handler { return &Handler{svc: s} }

// call authenticates the caller and parses id ("" when the request has none).
func call(ctx context.Context, id string) (principal.Principal, uuid.UUID, error) {
	p, err := principal.Require(ctx)
	if err != nil {
		return p, uuid.Nil, apperr.ToConnect(err)
	}
	if id == "" {
		return p, uuid.Nil, nil
	}
	parsed, err := parseID(id)
	return p, parsed, apperr.ToConnect(err)
}

// ListAgents lists the caller's agents (owner and admin: all).
func (h *Handler) ListAgents(ctx context.Context, _ *connect.Request[werbotv1.ListAgentsRequest]) (*connect.Response[werbotv1.ListAgentsResponse], error) {
	p, _, err := call(ctx, "")
	if err != nil {
		return nil, err
	}
	vs, err := h.svc.ListAgents(ctx, p)
	if err != nil {
		return nil, apperr.ToConnect(err)
	}
	res := &werbotv1.ListAgentsResponse{}
	for _, v := range vs {
		res.Agents = append(res.Agents, agentToProto(v))
	}
	return connect.NewResponse(res), nil
}

// CreateAgent creates an agent of the caller (owner and admin: of any member).
func (h *Handler) CreateAgent(ctx context.Context, req *connect.Request[werbotv1.CreateAgentRequest]) (*connect.Response[werbotv1.CreateAgentResponse], error) {
	p, _, err := call(ctx, "")
	if err != nil {
		return nil, err
	}
	in := CreateAgentInput{Name: req.Msg.GetName(), Limits: limitsFromProto(req.Msg.GetLimits())}
	if in.SponsorID, err = optionalID(req.Msg.GetSponsorUserId()); err != nil {
		return nil, apperr.ToConnect(err)
	}
	v, err := h.svc.CreateAgent(ctx, p, in)
	if err != nil {
		return nil, apperr.ToConnect(err)
	}
	return connect.NewResponse(&werbotv1.CreateAgentResponse{Agent: agentToProto(v)}), nil
}

// UpdateAgent changes the limits or status of an agent.
func (h *Handler) UpdateAgent(ctx context.Context, req *connect.Request[werbotv1.UpdateAgentRequest]) (*connect.Response[werbotv1.UpdateAgentResponse], error) {
	p, id, err := call(ctx, req.Msg.GetId())
	if err != nil {
		return nil, err
	}
	in := UpdateAgentInput{ID: id, Status: req.Msg.GetStatus()}
	if req.Msg.Limits != nil {
		in.Limits = new(limitsFromProto(req.Msg.GetLimits()))
	}
	v, err := h.svc.UpdateAgent(ctx, p, in)
	if err != nil {
		return nil, apperr.ToConnect(err)
	}
	return connect.NewResponse(&werbotv1.UpdateAgentResponse{Agent: agentToProto(v)}), nil
}

// DeleteAgent deletes an agent with its runs.
func (h *Handler) DeleteAgent(ctx context.Context, req *connect.Request[werbotv1.DeleteAgentRequest]) (*connect.Response[werbotv1.DeleteAgentResponse], error) {
	p, id, err := call(ctx, req.Msg.GetId())
	if err != nil {
		return nil, err
	}
	if err := h.svc.DeleteAgent(ctx, p, id); err != nil {
		return nil, apperr.ToConnect(err)
	}
	return connect.NewResponse(&werbotv1.DeleteAgentResponse{}), nil
}

// ListRuns returns one page of the runs the caller may see.
func (h *Handler) ListRuns(ctx context.Context, req *connect.Request[werbotv1.ListRunsRequest]) (*connect.Response[werbotv1.ListRunsResponse], error) {
	p, _, err := call(ctx, "")
	if err != nil {
		return nil, err
	}
	m := req.Msg
	f := RunFilter{Status: m.GetStatus(), Page: int(m.GetPage()), PageSize: int(m.GetPageSize())}
	if f.AgentID, err = optionalID(m.GetAgentId()); err != nil {
		return nil, apperr.ToConnect(err)
	}
	rs, next, err := h.svc.ListRuns(ctx, p, f)
	if err != nil {
		return nil, apperr.ToConnect(err)
	}
	res := &werbotv1.ListRunsResponse{NextPage: int32(next)} //nolint:gosec // next <= maxPage+1
	for _, r := range rs {
		res.Runs = append(res.Runs, runToProto(r))
	}
	return connect.NewResponse(res), nil
}

// GetRun returns a run with its timeline.
func (h *Handler) GetRun(ctx context.Context, req *connect.Request[werbotv1.GetRunRequest]) (*connect.Response[werbotv1.GetRunResponse], error) {
	p, id, err := call(ctx, req.Msg.GetId())
	if err != nil {
		return nil, err
	}
	tl, err := h.svc.Timeline(ctx, p, id)
	if err != nil {
		return nil, apperr.ToConnect(err)
	}
	res, err := timelineToProto(tl)
	if err != nil {
		return nil, apperr.ToConnect(err)
	}
	return connect.NewResponse(res), nil
}

// EndRun ends a run: client_exit with its run token, else ended_by_user.
func (h *Handler) EndRun(ctx context.Context, req *connect.Request[werbotv1.EndRunRequest]) (*connect.Response[werbotv1.EndRunResponse], error) {
	p, id, err := call(ctx, req.Msg.GetId())
	if err != nil {
		return nil, err
	}
	if err := h.svc.StopRun(ctx, p, id); err != nil {
		return nil, apperr.ToConnect(err)
	}
	return connect.NewResponse(&werbotv1.EndRunResponse{}), nil
}

// ResumeRun moves a paused run back to running.
func (h *Handler) ResumeRun(ctx context.Context, req *connect.Request[werbotv1.ResumeRunRequest]) (*connect.Response[werbotv1.ResumeRunResponse], error) {
	p, id, err := call(ctx, req.Msg.GetId())
	if err != nil {
		return nil, err
	}
	if err := h.svc.ResumeRun(ctx, p, id); err != nil {
		return nil, apperr.ToConnect(err)
	}
	return connect.NewResponse(&werbotv1.ResumeRunResponse{}), nil
}

// RecordToolCall stores a running tool call of the caller's run (run tokens only).
func (h *Handler) RecordToolCall(ctx context.Context, req *connect.Request[werbotv1.RecordToolCallRequest]) (*connect.Response[werbotv1.RecordToolCallResponse], error) {
	p, _, err := call(ctx, "")
	if err != nil {
		return nil, err
	}
	m := req.Msg
	id, err := h.svc.RecordToolCall(ctx, p, ToolCallInput{Tool: m.GetTool(), Arguments: m.GetArguments().AsMap(),
		Traceparent: m.GetTraceparent()})
	if err != nil {
		return nil, apperr.ToConnect(err)
	}
	return connect.NewResponse(&werbotv1.RecordToolCallResponse{Id: id.String()}), nil
}

// FinishToolCall ends a running tool call of the caller's run (run tokens only).
func (h *Handler) FinishToolCall(ctx context.Context, req *connect.Request[werbotv1.FinishToolCallRequest]) (*connect.Response[werbotv1.FinishToolCallResponse], error) {
	p, id, err := call(ctx, req.Msg.GetId())
	if err != nil {
		return nil, err
	}
	if err := h.svc.FinishToolCall(ctx, p, id, req.Msg.GetStatus()); err != nil {
		return nil, apperr.ToConnect(err)
	}
	return connect.NewResponse(&werbotv1.FinishToolCallResponse{}), nil
}
