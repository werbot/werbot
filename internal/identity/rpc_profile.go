package identity

import (
	"context"

	"connectrpc.com/connect"

	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
	"github.com/werbot/werbot/internal/platform/apperr"
	"github.com/werbot/werbot/internal/platform/principal"
)

// GetProfile returns the caller's user.
func (h *Handler) GetProfile(ctx context.Context, _ *connect.Request[werbotv1.GetProfileRequest]) (*connect.Response[werbotv1.GetProfileResponse], error) {
	if _, err := principal.Require(ctx); err != nil {
		return nil, apperr.ToConnect(err)
	}
	u, err := h.svc.Profile(ctx)
	if err != nil {
		return nil, apperr.ToConnect(err)
	}
	return connect.NewResponse(&werbotv1.GetProfileResponse{User: userToProto(u)}), nil
}

// UpdateProfile sets the caller's name and email.
func (h *Handler) UpdateProfile(ctx context.Context, req *connect.Request[werbotv1.UpdateProfileRequest]) (*connect.Response[werbotv1.UpdateProfileResponse], error) {
	if _, err := principal.Require(ctx); err != nil {
		return nil, apperr.ToConnect(err)
	}
	u, err := h.svc.UpdateProfile(ctx, req.Msg.GetName(), req.Msg.GetEmail())
	if err != nil {
		return nil, apperr.ToConnect(err)
	}
	return connect.NewResponse(&werbotv1.UpdateProfileResponse{User: userToProto(u)}), nil
}

// ChangePassword sets a new password and ends the caller's other sessions.
func (h *Handler) ChangePassword(ctx context.Context, req *connect.Request[werbotv1.ChangePasswordRequest]) (*connect.Response[werbotv1.ChangePasswordResponse], error) {
	if _, err := principal.Require(ctx); err != nil {
		return nil, apperr.ToConnect(err)
	}
	if err := h.svc.ChangePassword(ctx, req.Msg.GetCurrentPassword(), req.Msg.GetNewPassword()); err != nil {
		return nil, apperr.ToConnect(err)
	}
	return connect.NewResponse(&werbotv1.ChangePasswordResponse{}), nil
}

// RequestAccountDeletion emails the caller a deletion link.
func (h *Handler) RequestAccountDeletion(ctx context.Context, _ *connect.Request[werbotv1.RequestAccountDeletionRequest]) (*connect.Response[werbotv1.RequestAccountDeletionResponse], error) {
	if _, err := principal.Require(ctx); err != nil {
		return nil, apperr.ToConnect(err)
	}
	if err := h.svc.RequestAccountDeletion(ctx); err != nil {
		return nil, apperr.ToConnect(err)
	}
	return connect.NewResponse(&werbotv1.RequestAccountDeletionResponse{}), nil
}

// ConfirmAccountDeletion deletes the caller's account and clears the cookie.
func (h *Handler) ConfirmAccountDeletion(ctx context.Context, req *connect.Request[werbotv1.ConfirmAccountDeletionRequest]) (*connect.Response[werbotv1.ConfirmAccountDeletionResponse], error) {
	if _, err := principal.Require(ctx); err != nil {
		return nil, apperr.ToConnect(err)
	}
	if err := h.svc.ConfirmAccountDeletion(ctx, req.Msg.GetToken()); err != nil {
		return nil, apperr.ToConnect(err)
	}
	res := connect.NewResponse(&werbotv1.ConfirmAccountDeletionResponse{})
	res.Header().Add("Set-Cookie", h.sessionCookie("", -1))
	return res, nil
}
