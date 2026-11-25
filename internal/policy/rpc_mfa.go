package policy

import (
	"context"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
	"github.com/werbot/werbot/internal/platform/apperr"
)

func mfaRuleFromProto(name string, subj *werbotv1.Subjects, sel *werbotv1.ResourceSelector, maxAge *durationpb.Duration, strength string) (MFARuleInput, error) {
	in := MFARuleInput{Name: name, MaxAge: maxAge.AsDuration(), Strength: strength}
	var err error
	in.Subjects, in.Selector, err = targetFromProto(subj, sel)
	return in, err
}

func mfaRuleToProto(r MFARule) *werbotv1.MFARule {
	return &werbotv1.MFARule{Id: r.ID.String(), Name: r.Name, Subjects: subjectsToProto(r.Subjects), ResourceSelector: selectorToProto(r.Selector),
		MaxAge: durationpb.New(r.MaxAge), Strength: r.Strength, CreatedAt: timestamppb.New(r.CreatedAt)}
}

// ListMFARules lists the MFA rules by name.
func (h *Handler) ListMFARules(ctx context.Context, _ *connect.Request[werbotv1.ListMFARulesRequest]) (*connect.Response[werbotv1.ListMFARulesResponse], error) {
	org, err := admin(ctx)
	if err != nil {
		return nil, err
	}
	rules, err := h.svc.ListMFARules(ctx, org)
	if err != nil {
		return nil, apperr.ToConnect(err)
	}
	res := &werbotv1.ListMFARulesResponse{}
	for _, r := range rules {
		res.Rules = append(res.Rules, mfaRuleToProto(r))
	}
	return connect.NewResponse(res), nil
}

// CreateMFARule adds an MFA rule.
func (h *Handler) CreateMFARule(ctx context.Context, req *connect.Request[werbotv1.CreateMFARuleRequest]) (*connect.Response[werbotv1.CreateMFARuleResponse], error) {
	org, err := admin(ctx)
	if err != nil {
		return nil, err
	}
	m := req.Msg
	in, err := mfaRuleFromProto(m.GetName(), m.GetSubjects(), m.GetResourceSelector(), m.GetMaxAge(), m.GetStrength())
	if err != nil {
		return nil, apperr.ToConnect(err)
	}
	r, err := h.svc.CreateMFARule(ctx, org, in)
	if err != nil {
		return nil, apperr.ToConnect(err)
	}
	return connect.NewResponse(&werbotv1.CreateMFARuleResponse{Rule: mfaRuleToProto(r)}), nil
}

// UpdateMFARule replaces an MFA rule as a whole.
func (h *Handler) UpdateMFARule(ctx context.Context, req *connect.Request[werbotv1.UpdateMFARuleRequest]) (*connect.Response[werbotv1.UpdateMFARuleResponse], error) {
	m := req.Msg
	org, id, err := adminWith(ctx, m.GetId())
	if err != nil {
		return nil, err
	}
	in, err := mfaRuleFromProto(m.GetName(), m.GetSubjects(), m.GetResourceSelector(), m.GetMaxAge(), m.GetStrength())
	if err != nil {
		return nil, apperr.ToConnect(err)
	}
	r, err := h.svc.UpdateMFARule(ctx, org, id, in)
	if err != nil {
		return nil, apperr.ToConnect(err)
	}
	return connect.NewResponse(&werbotv1.UpdateMFARuleResponse{Rule: mfaRuleToProto(r)}), nil
}

// DeleteMFARule removes an MFA rule.
func (h *Handler) DeleteMFARule(ctx context.Context, req *connect.Request[werbotv1.DeleteMFARuleRequest]) (*connect.Response[werbotv1.DeleteMFARuleResponse], error) {
	org, id, err := adminWith(ctx, req.Msg.GetId())
	if err != nil {
		return nil, err
	}
	if err := h.svc.DeleteMFARule(ctx, org, id); err != nil {
		return nil, apperr.ToConnect(err)
	}
	return connect.NewResponse(&werbotv1.DeleteMFARuleResponse{}), nil
}
