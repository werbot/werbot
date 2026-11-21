package access

import (
	"context"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
	"github.com/werbot/werbot/gen/werbot/v1/werbotv1connect"
	"github.com/werbot/werbot/internal/platform/apperr"
	"github.com/werbot/werbot/internal/platform/principal"
	"github.com/werbot/werbot/internal/platform/rpcid"
)

// ApprovalPolicyHandler serves ApprovalPolicyService.
type ApprovalPolicyHandler struct {
	werbotv1connect.UnimplementedApprovalPolicyServiceHandler
	svc *Service
}

// NewApprovalPolicyHandler returns the Connect handler for ApprovalPolicyService.
func NewApprovalPolicyHandler(s *Service) *ApprovalPolicyHandler {
	return &ApprovalPolicyHandler{svc: s}
}

func adminPrincipal(ctx context.Context) (principal.Principal, error) {
	p, err := principal.RequireRole(ctx, "owner", "admin")
	return p, apperr.ToConnect(err)
}

// ListApprovalPolicies lists the policies by name.
func (h *ApprovalPolicyHandler) ListApprovalPolicies(ctx context.Context, _ *connect.Request[werbotv1.ListApprovalPoliciesRequest]) (*connect.Response[werbotv1.ListApprovalPoliciesResponse], error) {
	p, err := adminPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	ps, err := h.svc.ListApprovalPolicies(ctx, p.OrgID)
	if err != nil {
		return nil, apperr.ToConnect(err)
	}
	res := &werbotv1.ListApprovalPoliciesResponse{}
	for _, pol := range ps {
		res.ApprovalPolicies = append(res.ApprovalPolicies, policyToProto(pol))
	}
	return connect.NewResponse(res), nil
}

// CreateApprovalPolicy adds a policy.
func (h *ApprovalPolicyHandler) CreateApprovalPolicy(ctx context.Context, req *connect.Request[werbotv1.CreateApprovalPolicyRequest]) (*connect.Response[werbotv1.CreateApprovalPolicyResponse], error) {
	p, err := adminPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	m := req.Msg
	in, err := policyFromProto(m.GetName(), m.GetMode(), m.GetResourceSelector(), m.GetRequesters(), m.GetApprovers(), m.GetMaxDuration(),
		m.GetRequireReason(), m.GetFourEyes(), m.GetBreakGlass())
	if err != nil {
		return nil, apperr.ToConnect(err)
	}
	out, err := h.svc.CreateApprovalPolicy(ctx, p, in)
	if err != nil {
		return nil, apperr.ToConnect(err)
	}
	return connect.NewResponse(&werbotv1.CreateApprovalPolicyResponse{ApprovalPolicy: policyToProto(out)}), nil
}

// UpdateApprovalPolicy replaces a policy as a whole.
func (h *ApprovalPolicyHandler) UpdateApprovalPolicy(ctx context.Context, req *connect.Request[werbotv1.UpdateApprovalPolicyRequest]) (*connect.Response[werbotv1.UpdateApprovalPolicyResponse], error) {
	p, err := adminPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	m := req.Msg
	in, err := policyFromProto(m.GetName(), m.GetMode(), m.GetResourceSelector(), m.GetRequesters(), m.GetApprovers(), m.GetMaxDuration(),
		m.GetRequireReason(), m.GetFourEyes(), m.GetBreakGlass())
	if err == nil {
		in.ID, err = parseID(m.GetId())
	}
	if err != nil {
		return nil, apperr.ToConnect(err)
	}
	out, err := h.svc.UpdateApprovalPolicy(ctx, p, in)
	if err != nil {
		return nil, apperr.ToConnect(err)
	}
	return connect.NewResponse(&werbotv1.UpdateApprovalPolicyResponse{ApprovalPolicy: policyToProto(out)}), nil
}

// DeleteApprovalPolicy removes a policy.
func (h *ApprovalPolicyHandler) DeleteApprovalPolicy(ctx context.Context, req *connect.Request[werbotv1.DeleteApprovalPolicyRequest]) (*connect.Response[werbotv1.DeleteApprovalPolicyResponse], error) {
	p, err := adminPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	id, err := parseID(req.Msg.GetId())
	if err != nil {
		return nil, apperr.ToConnect(err)
	}
	if err := h.svc.DeleteApprovalPolicy(ctx, p, id); err != nil {
		return nil, apperr.ToConnect(err)
	}
	return connect.NewResponse(&werbotv1.DeleteApprovalPolicyResponse{}), nil
}

func subjectsFromProto(m *werbotv1.Subjects) (s Subjects, err error) {
	if s.Users, err = rpcid.Parse(m.GetUsers()); err != nil {
		return Subjects{}, err
	}
	if s.Teams, err = rpcid.Parse(m.GetTeams()); err != nil {
		return Subjects{}, err
	}
	s.AllUsers, s.AllAgents = m.GetAllUsers(), m.GetAllAgents()
	return s, nil
}

func policyFromProto(name, mode string, sel *werbotv1.ResourceSelector, requesters, approvers *werbotv1.Subjects, maxDuration *durationpb.Duration,
	requireReason, fourEyes, breakGlass bool) (ApprovalPolicy, error) {
	in := ApprovalPolicy{Name: name, Mode: mode, Selector: Selector{Labels: sel.GetLabels()}, MaxDuration: maxDuration.AsDuration(),
		RequireReason: requireReason, FourEyes: fourEyes, BreakGlass: breakGlass}
	var err error
	if in.Selector.ResourceIDs, err = rpcid.Parse(sel.GetResourceIds()); err != nil {
		return ApprovalPolicy{}, err
	}
	if in.Requesters, err = subjectsFromProto(requesters); err != nil {
		return ApprovalPolicy{}, err
	}
	if in.Approvers, err = subjectsFromProto(approvers); err != nil {
		return ApprovalPolicy{}, err
	}
	return in, nil
}

func subjectsToProto(s Subjects) *werbotv1.Subjects {
	return &werbotv1.Subjects{Users: rpcid.Strings(s.Users), Teams: rpcid.Strings(s.Teams), AllUsers: s.AllUsers, AllAgents: s.AllAgents}
}

func policyToProto(p ApprovalPolicy) *werbotv1.ApprovalPolicy {
	out := &werbotv1.ApprovalPolicy{Id: p.ID.String(), Name: p.Name,
		ResourceSelector: &werbotv1.ResourceSelector{ResourceIds: rpcid.Strings(p.Selector.ResourceIDs), Labels: p.Selector.Labels},
		Requesters:       subjectsToProto(p.Requesters), Approvers: subjectsToProto(p.Approvers), Mode: p.Mode,
		MaxDuration: durationpb.New(p.MaxDuration), RequireReason: p.RequireReason, FourEyes: p.FourEyes, BreakGlass: p.BreakGlass,
		CreatedAt: timestamppb.New(p.CreatedAt)}
	if p.CreatedBy != nil {
		out.CreatedBy = p.CreatedBy.String()
	}
	return out
}
