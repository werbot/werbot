package policy

import (
	"context"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
	"github.com/werbot/werbot/internal/platform/apperr"
)

func pathRuleFromProto(name string, subj *werbotv1.Subjects, sel *werbotv1.ResourceSelector, paths []string, access, effect, message string) (PathRuleInput, error) {
	in := PathRuleInput{Name: name, Paths: paths, Access: access, Effect: effect, Message: message}
	var err error
	in.Subjects, in.Selector, err = targetFromProto(subj, sel)
	return in, err
}

func pathRuleToProto(r PathRule) *werbotv1.PathRule {
	return &werbotv1.PathRule{Id: r.ID.String(), Name: r.Name, Subjects: subjectsToProto(r.Subjects), ResourceSelector: selectorToProto(r.Selector),
		Paths: r.Paths, Access: r.Access, Effect: r.Effect, Message: r.Message, CreatedBy: createdBy(r.CreatedBy), CreatedAt: timestamppb.New(r.CreatedAt)}
}

func terminalRuleToProto(r TerminalRule) *werbotv1.TerminalRule {
	return &werbotv1.TerminalRule{Id: r.ID.String(), Name: r.Name, Subjects: subjectsToProto(r.Subjects), ResourceSelector: selectorToProto(r.Selector),
		Mode: r.Mode, CreatedBy: createdBy(r.CreatedBy), CreatedAt: timestamppb.New(r.CreatedAt)}
}

// ListPathRules lists the path rules by name.
func (h *Handler) ListPathRules(ctx context.Context, _ *connect.Request[werbotv1.ListPathRulesRequest]) (*connect.Response[werbotv1.ListPathRulesResponse], error) {
	org, err := admin(ctx)
	if err != nil {
		return nil, err
	}
	rules, err := h.svc.ListPathRules(ctx, org)
	if err != nil {
		return nil, apperr.ToConnect(err)
	}
	res := &werbotv1.ListPathRulesResponse{}
	for _, r := range rules {
		res.Rules = append(res.Rules, pathRuleToProto(r))
	}
	return connect.NewResponse(res), nil
}

// CreatePathRule adds a path rule; an invalid glob is InvalidArgument with its text.
func (h *Handler) CreatePathRule(ctx context.Context, req *connect.Request[werbotv1.CreatePathRuleRequest]) (*connect.Response[werbotv1.CreatePathRuleResponse], error) {
	org, err := admin(ctx)
	if err != nil {
		return nil, err
	}
	m := req.Msg
	in, err := pathRuleFromProto(m.GetName(), m.GetSubjects(), m.GetResourceSelector(), m.GetPaths(), m.GetAccess(), m.GetEffect(), m.GetMessage())
	if err != nil {
		return nil, apperr.ToConnect(err)
	}
	r, err := h.svc.CreatePathRule(ctx, org, in)
	if err != nil {
		return nil, apperr.ToConnect(err)
	}
	return connect.NewResponse(&werbotv1.CreatePathRuleResponse{Rule: pathRuleToProto(r)}), nil
}

// UpdatePathRule replaces a path rule as a whole.
func (h *Handler) UpdatePathRule(ctx context.Context, req *connect.Request[werbotv1.UpdatePathRuleRequest]) (*connect.Response[werbotv1.UpdatePathRuleResponse], error) {
	m := req.Msg
	org, id, err := adminWith(ctx, m.GetId())
	if err != nil {
		return nil, err
	}
	in, err := pathRuleFromProto(m.GetName(), m.GetSubjects(), m.GetResourceSelector(), m.GetPaths(), m.GetAccess(), m.GetEffect(), m.GetMessage())
	if err != nil {
		return nil, apperr.ToConnect(err)
	}
	r, err := h.svc.UpdatePathRule(ctx, org, id, in)
	if err != nil {
		return nil, apperr.ToConnect(err)
	}
	return connect.NewResponse(&werbotv1.UpdatePathRuleResponse{Rule: pathRuleToProto(r)}), nil
}

// DeletePathRule removes a path rule.
func (h *Handler) DeletePathRule(ctx context.Context, req *connect.Request[werbotv1.DeletePathRuleRequest]) (*connect.Response[werbotv1.DeletePathRuleResponse], error) {
	org, id, err := adminWith(ctx, req.Msg.GetId())
	if err != nil {
		return nil, err
	}
	if err := h.svc.DeletePathRule(ctx, org, id); err != nil {
		return nil, apperr.ToConnect(err)
	}
	return connect.NewResponse(&werbotv1.DeletePathRuleResponse{}), nil
}

// ListTerminalRules lists the terminal rules by name.
func (h *Handler) ListTerminalRules(ctx context.Context, _ *connect.Request[werbotv1.ListTerminalRulesRequest]) (*connect.Response[werbotv1.ListTerminalRulesResponse], error) {
	org, err := admin(ctx)
	if err != nil {
		return nil, err
	}
	rules, err := h.svc.ListTerminalRules(ctx, org)
	if err != nil {
		return nil, apperr.ToConnect(err)
	}
	res := &werbotv1.ListTerminalRulesResponse{}
	for _, r := range rules {
		res.Rules = append(res.Rules, terminalRuleToProto(r))
	}
	return connect.NewResponse(res), nil
}

// CreateTerminalRule adds a terminal rule.
func (h *Handler) CreateTerminalRule(ctx context.Context, req *connect.Request[werbotv1.CreateTerminalRuleRequest]) (*connect.Response[werbotv1.CreateTerminalRuleResponse], error) {
	org, err := admin(ctx)
	if err != nil {
		return nil, err
	}
	m := req.Msg
	in := TerminalRuleInput{Name: m.GetName(), Mode: m.GetMode()}
	if in.Subjects, in.Selector, err = targetFromProto(m.GetSubjects(), m.GetResourceSelector()); err != nil {
		return nil, apperr.ToConnect(err)
	}
	r, err := h.svc.CreateTerminalRule(ctx, org, in)
	if err != nil {
		return nil, apperr.ToConnect(err)
	}
	return connect.NewResponse(&werbotv1.CreateTerminalRuleResponse{Rule: terminalRuleToProto(r)}), nil
}

// DeleteTerminalRule removes a terminal rule.
func (h *Handler) DeleteTerminalRule(ctx context.Context, req *connect.Request[werbotv1.DeleteTerminalRuleRequest]) (*connect.Response[werbotv1.DeleteTerminalRuleResponse], error) {
	org, id, err := adminWith(ctx, req.Msg.GetId())
	if err != nil {
		return nil, err
	}
	if err := h.svc.DeleteTerminalRule(ctx, org, id); err != nil {
		return nil, apperr.ToConnect(err)
	}
	return connect.NewResponse(&werbotv1.DeleteTerminalRuleResponse{}), nil
}
