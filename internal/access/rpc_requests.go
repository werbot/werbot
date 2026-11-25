package access

import (
	"context"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/durationpb"

	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
	"github.com/werbot/werbot/gen/werbot/v1/werbotv1connect"
	"github.com/werbot/werbot/internal/platform/apperr"
	"github.com/werbot/werbot/internal/platform/principal"
)

// RequestHandler serves AccessRequestService; approver and owner checks are in the service.
type RequestHandler struct {
	werbotv1connect.UnimplementedAccessRequestServiceHandler
	svc *Service
}

// NewAccessRequestHandler returns the Connect handler for AccessRequestService.
func NewAccessRequestHandler(s *Service) *RequestHandler { return &RequestHandler{svc: s} }

func caller(ctx context.Context) (principal.Principal, error) {
	p, err := principal.Require(ctx)
	return p, apperr.ToConnect(err)
}

// scopes maps the API scopes; an unknown one maps to "", which ListRequests refuses.
var scopes = map[werbotv1.RequestScope]string{
	werbotv1.RequestScope_REQUEST_SCOPE_UNSPECIFIED: ScopeMine,
	werbotv1.RequestScope_REQUEST_SCOPE_MINE:        ScopeMine,
	werbotv1.RequestScope_REQUEST_SCOPE_TO_APPROVE:  ScopeToApprove,
	werbotv1.RequestScope_REQUEST_SCOPE_ALL:         ScopeAll,
}

// CreateRequest asks for access to one resource or to the resources matching a label selector.
func (h *RequestHandler) CreateRequest(ctx context.Context, req *connect.Request[werbotv1.CreateRequestRequest]) (*connect.Response[werbotv1.CreateRequestResponse], error) {
	p, err := caller(ctx)
	if err != nil {
		return nil, err
	}
	m := req.Msg
	in := CreateRequestInput{Selector: m.GetLabelSelector(), Duration: m.GetDuration().AsDuration(), Reason: m.GetReason(), BreakGlass: m.GetBreakGlass()}
	if in.ResourceID, err = optionalID(m.GetResourceId()); err != nil {
		return nil, apperr.ToConnect(err)
	}
	a, err := h.svc.CreateRequest(ctx, p, in)
	if err != nil {
		return nil, apperr.ToConnect(err)
	}
	return connect.NewResponse(&werbotv1.CreateRequestResponse{Request: h.requestToProto(RequestView{Approval: a})}), nil
}

// ListRequests returns one page of requests of the asked scope.
func (h *RequestHandler) ListRequests(ctx context.Context, req *connect.Request[werbotv1.ListRequestsRequest]) (*connect.Response[werbotv1.ListRequestsResponse], error) {
	p, err := caller(ctx)
	if err != nil {
		return nil, err
	}
	m := req.Msg
	vs, next, err := h.svc.ListRequests(ctx, p, RequestFilter{Scope: scopes[m.GetScope()], Status: m.GetStatus(),
		Page: int(m.GetPage()), PageSize: int(m.GetPageSize())})
	if err != nil {
		return nil, apperr.ToConnect(err)
	}
	res := &werbotv1.ListRequestsResponse{NextPage: int32(next)} //nolint:gosec // next <= maxPage+1
	for _, v := range vs {
		res.Requests = append(res.Requests, h.requestToProto(v))
	}
	return connect.NewResponse(res), nil
}

// GetRequest returns one request the caller may see.
func (h *RequestHandler) GetRequest(ctx context.Context, req *connect.Request[werbotv1.GetRequestRequest]) (*connect.Response[werbotv1.GetRequestResponse], error) {
	p, err := caller(ctx)
	if err != nil {
		return nil, err
	}
	id, err := parseID(req.Msg.GetId())
	if err != nil {
		return nil, apperr.ToConnect(err)
	}
	v, err := h.svc.GetRequest(ctx, p, id)
	if err != nil {
		return nil, apperr.ToConnect(err)
	}
	return connect.NewResponse(&werbotv1.GetRequestResponse{Request: h.requestToProto(v)}), nil
}

// decide approves or denies request id as the caller.
func (h *RequestHandler) decide(ctx context.Context, rawID, comment string, approve bool) (*werbotv1.AccessRequest, error) {
	p, err := caller(ctx)
	if err != nil {
		return nil, err
	}
	id, err := parseID(rawID)
	if err != nil {
		return nil, apperr.ToConnect(err)
	}
	a, err := h.svc.Decide(ctx, p, id, approve, comment)
	if err != nil {
		return nil, apperr.ToConnect(err)
	}
	return h.requestToProto(RequestView{Approval: a}), nil
}

// Approve approves a pending request the caller may decide.
func (h *RequestHandler) Approve(ctx context.Context, req *connect.Request[werbotv1.ApproveRequest]) (*connect.Response[werbotv1.ApproveResponse], error) {
	out, err := h.decide(ctx, req.Msg.GetId(), req.Msg.GetComment(), true)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&werbotv1.ApproveResponse{Request: out}), nil
}

// Deny denies a pending request the caller may decide.
func (h *RequestHandler) Deny(ctx context.Context, req *connect.Request[werbotv1.DenyRequest]) (*connect.Response[werbotv1.DenyResponse], error) {
	out, err := h.decide(ctx, req.Msg.GetId(), req.Msg.GetComment(), false)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&werbotv1.DenyResponse{Request: out}), nil
}

// Cancel withdraws a pending request of the caller.
func (h *RequestHandler) Cancel(ctx context.Context, req *connect.Request[werbotv1.CancelRequest]) (*connect.Response[werbotv1.CancelResponse], error) {
	p, err := caller(ctx)
	if err != nil {
		return nil, err
	}
	id, err := parseID(req.Msg.GetId())
	if err != nil {
		return nil, apperr.ToConnect(err)
	}
	if err := h.svc.Cancel(ctx, p, id); err != nil {
		return nil, apperr.ToConnect(err)
	}
	return connect.NewResponse(&werbotv1.CancelResponse{}), nil
}

// ListGrants lists grants: owner and admin of the organization, a member only own ones.
func (h *RequestHandler) ListGrants(ctx context.Context, req *connect.Request[werbotv1.ListGrantsRequest]) (*connect.Response[werbotv1.ListGrantsResponse], error) {
	p, err := caller(ctx)
	if err != nil {
		return nil, err
	}
	f := GrantFilter{Active: req.Msg.GetActive()}
	if f.UserID, err = optionalID(req.Msg.GetUserId()); err == nil {
		f.ResourceID, err = optionalID(req.Msg.GetResourceId())
	}
	if err != nil {
		return nil, apperr.ToConnect(err)
	}
	gs, err := h.svc.ListGrants(ctx, p, f)
	if err != nil {
		return nil, apperr.ToConnect(err)
	}
	res := &werbotv1.ListGrantsResponse{}
	for _, g := range gs {
		res.Grants = append(res.Grants, grantToProto(g))
	}
	return connect.NewResponse(res), nil
}

// RevokeGrant ends a grant now.
func (h *RequestHandler) RevokeGrant(ctx context.Context, req *connect.Request[werbotv1.RevokeGrantRequest]) (*connect.Response[werbotv1.RevokeGrantResponse], error) {
	p, err := caller(ctx)
	if err != nil {
		return nil, err
	}
	id, err := parseID(req.Msg.GetId())
	if err != nil {
		return nil, apperr.ToConnect(err)
	}
	if err := h.svc.RevokeGrant(ctx, p, id, req.Msg.GetReason()); err != nil {
		return nil, apperr.ToConnect(err)
	}
	return connect.NewResponse(&werbotv1.RevokeGrantResponse{}), nil
}

// ReviewBreakGlass marks a break-glass grant of another user reviewed; owner and admin only.
func (h *RequestHandler) ReviewBreakGlass(ctx context.Context, req *connect.Request[werbotv1.ReviewBreakGlassRequest]) (*connect.Response[werbotv1.ReviewBreakGlassResponse], error) {
	p, err := adminPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	id, err := parseID(req.Msg.GetGrantId())
	if err != nil {
		return nil, apperr.ToConnect(err)
	}
	if err := h.svc.ReviewBreakGlass(ctx, p, id, req.Msg.GetComment()); err != nil {
		return nil, apperr.ToConnect(err)
	}
	return connect.NewResponse(&werbotv1.ReviewBreakGlassResponse{}), nil
}

// optionalID parses s; empty is nil.
func optionalID(s string) (*uuid.UUID, error) {
	if s == "" {
		return nil, nil
	}
	id, err := parseID(s)
	return &id, err
}

func (h *RequestHandler) requestToProto(v RequestView) *werbotv1.AccessRequest {
	out := &werbotv1.AccessRequest{Id: v.ID.String(), Kind: string(v.Kind), RequesterUserId: v.RequesterID.String(),
		RequesterLogin: v.RequesterLogin, ResourceId: idString(v.ResourceID), ResourceName: v.ResourceName, LabelSelector: v.Selector,
		Command: v.Command, Classes: v.Classes, Reason: v.Reason, PolicyId: idString(v.PolicyID), BreakGlass: v.BreakGlass,
		Status: v.Status, DecidedBy: idString(v.DecidedBy), DecidedAt: timestamp(v.DecidedAt), Comment: v.Comment,
		ExpiresAt: timestamp(&v.ExpiresAt), CreatedAt: timestamp(&v.CreatedAt), Url: h.svc.RequestURL(v.ID), CanDecide: v.CanDecide}
	if v.Kind == KindAccess {
		out.Duration = durationpb.New(v.Duration)
	}
	return out
}

func grantToProto(g GrantView) *werbotv1.Grant {
	return &werbotv1.Grant{Id: g.ID.String(), UserId: g.UserID.String(), UserLogin: g.UserLogin, ResourceId: g.ResourceID.String(),
		ResourceName: g.ResourceName, ApprovalId: idString(g.ApprovalID), ExpiresAt: timestamp(&g.ExpiresAt),
		RevokedAt: timestamp(g.RevokedAt), BreakGlass: g.BreakGlass, NeedsReview: g.NeedsReview, CreatedBy: idString(g.CreatedBy),
		CreatedAt: timestamp(&g.CreatedAt)}
}
