package identity

import (
	"context"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
	"github.com/werbot/werbot/internal/platform/apperr"
	"github.com/werbot/werbot/internal/platform/principal"
)

// ListMembers returns the organization's members and pending invites.
func (h *Handler) ListMembers(ctx context.Context, _ *connect.Request[werbotv1.ListMembersRequest]) (*connect.Response[werbotv1.ListMembersResponse], error) {
	if _, err := principal.RequireRole(ctx, principal.RoleOwner, principal.RoleAdmin); err != nil {
		return nil, apperr.ToConnect(err)
	}
	members, invites, err := h.svc.ListMembers(ctx)
	if err != nil {
		return nil, apperr.ToConnect(err)
	}
	res := &werbotv1.ListMembersResponse{}
	for _, m := range members {
		res.Members = append(res.Members, &werbotv1.Member{User: userToProto(m.User), Role: roleToProto(m.Role), JoinedAt: timestamppb.New(m.JoinedAt)})
	}
	for _, i := range invites {
		res.Invites = append(res.Invites, &werbotv1.Invite{Email: i.Email, Role: roleToProto(i.Role), ExpiresAt: timestamppb.New(i.ExpiresAt)})
	}
	return connect.NewResponse(res), nil
}

// InviteMember emails an invitation to join the organization.
func (h *Handler) InviteMember(ctx context.Context, req *connect.Request[werbotv1.InviteMemberRequest]) (*connect.Response[werbotv1.InviteMemberResponse], error) {
	if _, err := principal.RequireRole(ctx, principal.RoleOwner, principal.RoleAdmin); err != nil {
		return nil, apperr.ToConnect(err)
	}
	if err := h.svc.InviteMember(ctx, req.Msg.GetEmail(), roleFromProto(req.Msg.GetRole())); err != nil {
		return nil, apperr.ToConnect(err)
	}
	return connect.NewResponse(&werbotv1.InviteMemberResponse{}), nil
}

// AcceptInvite is public: the one-time token is the credential. A new
// session sets the cookie; a signed-in caller keeps the current one.
func (h *Handler) AcceptInvite(ctx context.Context, req *connect.Request[werbotv1.AcceptInviteRequest]) (*connect.Response[werbotv1.AcceptInviteResponse], error) {
	m := req.Msg
	token, err := h.svc.AcceptInvite(ctx, AcceptInput{
		Token: m.GetToken(), Login: m.GetLogin(), Name: m.GetName(), Password: m.GetPassword(),
	}, req.Header().Get("User-Agent"))
	if err != nil {
		return nil, apperr.ToConnect(err)
	}
	res := connect.NewResponse(&werbotv1.AcceptInviteResponse{})
	if token != "" {
		res.Header().Add("Set-Cookie", h.sessionCookie(token, sessionMaxAge))
	}
	return res, nil
}

// UpdateMemberRole changes a member's role.
func (h *Handler) UpdateMemberRole(ctx context.Context, req *connect.Request[werbotv1.UpdateMemberRoleRequest]) (*connect.Response[werbotv1.UpdateMemberRoleResponse], error) {
	if _, err := principal.RequireRole(ctx, principal.RoleOwner, principal.RoleAdmin); err != nil {
		return nil, apperr.ToConnect(err)
	}
	id, err := parseID(req.Msg.GetUserId())
	if err != nil {
		return nil, apperr.ToConnect(err)
	}
	if err := h.svc.UpdateMemberRole(ctx, id, roleFromProto(req.Msg.GetRole())); err != nil {
		return nil, apperr.ToConnect(err)
	}
	return connect.NewResponse(&werbotv1.UpdateMemberRoleResponse{}), nil
}

// RemoveMember removes a member from the organization.
func (h *Handler) RemoveMember(ctx context.Context, req *connect.Request[werbotv1.RemoveMemberRequest]) (*connect.Response[werbotv1.RemoveMemberResponse], error) {
	if _, err := principal.RequireRole(ctx, principal.RoleOwner, principal.RoleAdmin); err != nil {
		return nil, apperr.ToConnect(err)
	}
	id, err := parseID(req.Msg.GetUserId())
	if err != nil {
		return nil, apperr.ToConnect(err)
	}
	if err := h.svc.RemoveMember(ctx, id); err != nil {
		return nil, apperr.ToConnect(err)
	}
	return connect.NewResponse(&werbotv1.RemoveMemberResponse{}), nil
}
