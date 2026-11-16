package identity

import (
	"context"

	"connectrpc.com/connect"

	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
	"github.com/werbot/werbot/internal/platform/apperr"
	"github.com/werbot/werbot/internal/platform/principal"
)

// GetSetupState is public: the UI shows the setup page until the first owner exists.
func (h *Handler) GetSetupState(ctx context.Context, _ *connect.Request[werbotv1.GetSetupStateRequest]) (*connect.Response[werbotv1.GetSetupStateResponse], error) {
	needs, err := h.svc.NeedsSetup(ctx)
	if err != nil {
		return nil, apperr.ToConnect(err)
	}
	return connect.NewResponse(&werbotv1.GetSetupStateResponse{NeedsSetup: needs}), nil
}

// Setup is public: it creates the first owner and signs them in.
func (h *Handler) Setup(ctx context.Context, req *connect.Request[werbotv1.SetupRequest]) (*connect.Response[werbotv1.SetupResponse], error) {
	m := req.Msg
	token, err := h.svc.Setup(ctx, SetupInput{
		Email: m.GetEmail(), Login: m.GetLogin(), Name: m.GetName(), Password: m.GetPassword(), OrganizationName: m.GetOrganizationName(),
	}, req.Header().Get("User-Agent"))
	if err != nil {
		return nil, apperr.ToConnect(err)
	}
	res := connect.NewResponse(&werbotv1.SetupResponse{})
	res.Header().Add("Set-Cookie", h.sessionCookie(token, sessionMaxAge))
	return res, nil
}

// Login is public: it sets the session cookie.
func (h *Handler) Login(ctx context.Context, req *connect.Request[werbotv1.LoginRequest]) (*connect.Response[werbotv1.LoginResponse], error) {
	token, err := h.svc.Login(ctx, req.Msg.GetEmail(), req.Msg.GetPassword(), req.Header().Get("User-Agent"))
	if err != nil {
		return nil, apperr.ToConnect(err)
	}
	res := connect.NewResponse(&werbotv1.LoginResponse{})
	res.Header().Add("Set-Cookie", h.sessionCookie(token, sessionMaxAge))
	return res, nil
}

// Logout deletes the session and the cookie.
func (h *Handler) Logout(ctx context.Context, _ *connect.Request[werbotv1.LogoutRequest]) (*connect.Response[werbotv1.LogoutResponse], error) {
	if _, err := principal.Require(ctx); err != nil {
		return nil, apperr.ToConnect(err)
	}
	if err := h.svc.Logout(ctx); err != nil {
		return nil, apperr.ToConnect(err)
	}
	res := connect.NewResponse(&werbotv1.LogoutResponse{})
	res.Header().Add("Set-Cookie", h.sessionCookie("", -1))
	return res, nil
}

// GetSession returns the caller's user, organization and role.
func (h *Handler) GetSession(ctx context.Context, _ *connect.Request[werbotv1.GetSessionRequest]) (*connect.Response[werbotv1.GetSessionResponse], error) {
	if _, err := principal.Require(ctx); err != nil {
		return nil, apperr.ToConnect(err)
	}
	info, err := h.svc.CurrentSession(ctx)
	if err != nil {
		return nil, apperr.ToConnect(err)
	}
	return connect.NewResponse(&werbotv1.GetSessionResponse{
		User:         userToProto(info.User),
		Organization: &werbotv1.Organization{Id: info.OrgID.String(), Name: info.OrgName},
		Role:         roleToProto(info.Role),
	}), nil
}
