package policy

import (
	"context"
	"errors"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/timestamppb"

	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
	"github.com/werbot/werbot/internal/platform/apperr"
	"github.com/werbot/werbot/internal/platform/clientip"
)

func parseIDs(ss []string) ([]uuid.UUID, error) {
	var out []uuid.UUID
	for _, s := range ss {
		id, err := parseID(s)
		if err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, nil
}

func idStrings(ids []uuid.UUID) (out []string) {
	for _, id := range ids {
		out = append(out, id.String())
	}
	return out
}

func createdBy(id *uuid.UUID) string {
	if id == nil {
		return ""
	}
	return id.String()
}

func commandRuleFromProto(name string, subj *werbotv1.Subjects, sel *werbotv1.ResourceSelector, classes []string, effect, message string) (CommandRuleInput, error) {
	in := CommandRuleInput{Name: name, Subjects: Subjects{AllUsers: subj.GetAllUsers(), AllAgents: subj.GetAllAgents()},
		Selector: Selector{Labels: sel.GetLabels()}, Classes: classes, Effect: effect, Message: message}
	var e1, e2, e3 error
	in.Subjects.Users, e1 = parseIDs(subj.GetUsers())
	in.Subjects.Teams, e2 = parseIDs(subj.GetTeams())
	in.Selector.ResourceIDs, e3 = parseIDs(sel.GetResourceIds())
	return in, errors.Join(e1, e2, e3)
}

func commandRuleToProto(r CommandRule) *werbotv1.CommandRule {
	s := r.Subjects
	return &werbotv1.CommandRule{Id: r.ID.String(), Name: r.Name,
		Subjects:         &werbotv1.Subjects{Users: idStrings(s.Users), Teams: idStrings(s.Teams), AllUsers: s.AllUsers, AllAgents: s.AllAgents},
		ResourceSelector: &werbotv1.ResourceSelector{ResourceIds: idStrings(r.Selector.ResourceIDs), Labels: r.Selector.Labels},
		Classes:          r.Classes, Effect: r.Effect, Message: r.Message, CreatedBy: createdBy(r.CreatedBy), CreatedAt: timestamppb.New(r.CreatedAt)}
}

func classRuleToProto(r ClassRule) *werbotv1.ClassRule {
	return &werbotv1.ClassRule{Id: r.ID.String(), Pattern: r.Pattern, Classes: r.Classes, CreatedBy: createdBy(r.CreatedBy),
		CreatedAt: timestamppb.New(r.CreatedAt)}
}

func customPolicyToProto(p CustomPolicy) *werbotv1.CustomPolicy {
	return &werbotv1.CustomPolicy{Id: p.ID.String(), Name: p.Name, Cedar: p.Cedar, Enabled: p.Enabled, CreatedBy: createdBy(p.CreatedBy),
		UpdatedAt: timestamppb.New(p.UpdatedAt)}
}

// adminWith authorizes owner/admin and parses id; the organization comes first.
func adminWith(ctx context.Context, id string) (org, parsed uuid.UUID, err error) {
	if org, err = admin(ctx); err != nil {
		return uuid.Nil, uuid.Nil, err
	}
	parsed, err = parseID(id)
	return org, parsed, apperr.ToConnect(err)
}

// ListCommandRules lists the command rules by name.
func (h *Handler) ListCommandRules(ctx context.Context, _ *connect.Request[werbotv1.ListCommandRulesRequest]) (*connect.Response[werbotv1.ListCommandRulesResponse], error) {
	org, err := admin(ctx)
	if err != nil {
		return nil, err
	}
	rules, err := h.svc.ListCommandRules(ctx, org)
	if err != nil {
		return nil, apperr.ToConnect(err)
	}
	res := &werbotv1.ListCommandRulesResponse{}
	for _, r := range rules {
		res.Rules = append(res.Rules, commandRuleToProto(r))
	}
	return connect.NewResponse(res), nil
}

// CreateCommandRule adds a command rule.
func (h *Handler) CreateCommandRule(ctx context.Context, req *connect.Request[werbotv1.CreateCommandRuleRequest]) (*connect.Response[werbotv1.CreateCommandRuleResponse], error) {
	org, err := admin(ctx)
	if err != nil {
		return nil, err
	}
	m := req.Msg
	in, err := commandRuleFromProto(m.GetName(), m.GetSubjects(), m.GetResourceSelector(), m.GetClasses(), m.GetEffect(), m.GetMessage())
	if err != nil {
		return nil, apperr.ToConnect(err)
	}
	r, err := h.svc.CreateCommandRule(ctx, org, in)
	if err != nil {
		return nil, apperr.ToConnect(err)
	}
	return connect.NewResponse(&werbotv1.CreateCommandRuleResponse{Rule: commandRuleToProto(r)}), nil
}

// UpdateCommandRule replaces a command rule as a whole.
func (h *Handler) UpdateCommandRule(ctx context.Context, req *connect.Request[werbotv1.UpdateCommandRuleRequest]) (*connect.Response[werbotv1.UpdateCommandRuleResponse], error) {
	m := req.Msg
	org, id, err := adminWith(ctx, m.GetId())
	if err != nil {
		return nil, err
	}
	in, err := commandRuleFromProto(m.GetName(), m.GetSubjects(), m.GetResourceSelector(), m.GetClasses(), m.GetEffect(), m.GetMessage())
	if err != nil {
		return nil, apperr.ToConnect(err)
	}
	r, err := h.svc.UpdateCommandRule(ctx, org, id, in)
	if err != nil {
		return nil, apperr.ToConnect(err)
	}
	return connect.NewResponse(&werbotv1.UpdateCommandRuleResponse{Rule: commandRuleToProto(r)}), nil
}

// DeleteCommandRule removes a command rule.
func (h *Handler) DeleteCommandRule(ctx context.Context, req *connect.Request[werbotv1.DeleteCommandRuleRequest]) (*connect.Response[werbotv1.DeleteCommandRuleResponse], error) {
	org, id, err := adminWith(ctx, req.Msg.GetId())
	if err != nil {
		return nil, err
	}
	if err := h.svc.DeleteCommandRule(ctx, org, id); err != nil {
		return nil, apperr.ToConnect(err)
	}
	return connect.NewResponse(&werbotv1.DeleteCommandRuleResponse{}), nil
}

// ListClassRules lists the class rules by pattern.
func (h *Handler) ListClassRules(ctx context.Context, _ *connect.Request[werbotv1.ListClassRulesRequest]) (*connect.Response[werbotv1.ListClassRulesResponse], error) {
	org, err := admin(ctx)
	if err != nil {
		return nil, err
	}
	rules, err := h.svc.ListClassRules(ctx, org)
	if err != nil {
		return nil, apperr.ToConnect(err)
	}
	res := &werbotv1.ListClassRulesResponse{}
	for _, r := range rules {
		res.Rules = append(res.Rules, classRuleToProto(r))
	}
	return connect.NewResponse(res), nil
}

// CreateClassRule adds a class rule.
func (h *Handler) CreateClassRule(ctx context.Context, req *connect.Request[werbotv1.CreateClassRuleRequest]) (*connect.Response[werbotv1.CreateClassRuleResponse], error) {
	org, err := admin(ctx)
	if err != nil {
		return nil, err
	}
	r, err := h.svc.CreateClassRule(ctx, org, req.Msg.GetPattern(), req.Msg.GetClasses())
	if err != nil {
		return nil, apperr.ToConnect(err)
	}
	return connect.NewResponse(&werbotv1.CreateClassRuleResponse{Rule: classRuleToProto(r)}), nil
}

// DeleteClassRule removes a class rule.
func (h *Handler) DeleteClassRule(ctx context.Context, req *connect.Request[werbotv1.DeleteClassRuleRequest]) (*connect.Response[werbotv1.DeleteClassRuleResponse], error) {
	org, id, err := adminWith(ctx, req.Msg.GetId())
	if err != nil {
		return nil, err
	}
	if err := h.svc.DeleteClassRule(ctx, org, id); err != nil {
		return nil, apperr.ToConnect(err)
	}
	return connect.NewResponse(&werbotv1.DeleteClassRuleResponse{}), nil
}

// ListCustomPolicies lists the custom policies by name, with their Cedar texts.
func (h *Handler) ListCustomPolicies(ctx context.Context, _ *connect.Request[werbotv1.ListCustomPoliciesRequest]) (*connect.Response[werbotv1.ListCustomPoliciesResponse], error) {
	org, err := admin(ctx)
	if err != nil {
		return nil, err
	}
	ps, err := h.svc.ListCustomPolicies(ctx, org)
	if err != nil {
		return nil, apperr.ToConnect(err)
	}
	res := &werbotv1.ListCustomPoliciesResponse{}
	for _, p := range ps {
		res.Policies = append(res.Policies, customPolicyToProto(p))
	}
	return connect.NewResponse(res), nil
}

// CreateCustomPolicy adds a custom policy; a Cedar error is InvalidArgument with its full text.
func (h *Handler) CreateCustomPolicy(ctx context.Context, req *connect.Request[werbotv1.CreateCustomPolicyRequest]) (*connect.Response[werbotv1.CreateCustomPolicyResponse], error) {
	org, err := admin(ctx)
	if err != nil {
		return nil, err
	}
	m := req.Msg
	p, err := h.svc.CreateCustomPolicy(ctx, org, CustomPolicyInput{Name: m.GetName(), Cedar: m.GetCedar(), Enabled: m.GetEnabled()})
	if err != nil {
		return nil, apperr.ToConnect(err)
	}
	return connect.NewResponse(&werbotv1.CreateCustomPolicyResponse{Policy: customPolicyToProto(p)}), nil
}

// UpdateCustomPolicy replaces a custom policy as a whole.
func (h *Handler) UpdateCustomPolicy(ctx context.Context, req *connect.Request[werbotv1.UpdateCustomPolicyRequest]) (*connect.Response[werbotv1.UpdateCustomPolicyResponse], error) {
	m := req.Msg
	org, id, err := adminWith(ctx, m.GetId())
	if err != nil {
		return nil, err
	}
	p, err := h.svc.UpdateCustomPolicy(ctx, org, id, CustomPolicyInput{Name: m.GetName(), Cedar: m.GetCedar(), Enabled: m.GetEnabled()})
	if err != nil {
		return nil, apperr.ToConnect(err)
	}
	return connect.NewResponse(&werbotv1.UpdateCustomPolicyResponse{Policy: customPolicyToProto(p)}), nil
}

// DeleteCustomPolicy removes a custom policy.
func (h *Handler) DeleteCustomPolicy(ctx context.Context, req *connect.Request[werbotv1.DeleteCustomPolicyRequest]) (*connect.Response[werbotv1.DeleteCustomPolicyResponse], error) {
	org, id, err := adminWith(ctx, req.Msg.GetId())
	if err != nil {
		return nil, err
	}
	if err := h.svc.DeleteCustomPolicy(ctx, org, id); err != nil {
		return nil, apperr.ToConnect(err)
	}
	return connect.NewResponse(&werbotv1.DeleteCustomPolicyResponse{}), nil
}

// TestCommand probes a command for a user with the caller's address, now.
// Agents arrive in stage 12.
func (h *Handler) TestCommand(ctx context.Context, req *connect.Request[werbotv1.TestCommandRequest]) (*connect.Response[werbotv1.TestCommandResponse], error) {
	m := req.Msg
	if _, isAgent := m.GetSubject().(*werbotv1.TestCommandRequest_AgentId); isAgent {
		if _, err := admin(ctx); err != nil {
			return nil, err
		}
		return nil, connect.NewError(connect.CodeUnimplemented, errors.New("agents arrive in stage 12"))
	}
	org, user, err := adminWith(ctx, m.GetUserId())
	if err != nil {
		return nil, err
	}
	resource, err := parseID(m.GetResourceId())
	if err != nil {
		return nil, apperr.ToConnect(err)
	}
	r, err := h.svc.TestCommand(ctx, org, user, resource, m.GetCommand(), clientip.From(ctx), time.Now())
	if err != nil {
		return nil, apperr.ToConnect(err)
	}
	return connect.NewResponse(&werbotv1.TestCommandResponse{Classes: r.Classes, Allow: r.Allow, Reasons: r.Reasons,
		Obligations: r.Obligations, Message: r.Message}), nil
}
