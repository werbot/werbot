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

// Login is public: it sets the session cookie, or, for a user with a second
// factor, answers the mfa_token and the methods for CompleteMFA.
func (h *Handler) Login(ctx context.Context, req *connect.Request[werbotv1.LoginRequest]) (*connect.Response[werbotv1.LoginResponse], error) {
	r, err := h.svc.Login(ctx, req.Msg.GetEmail(), req.Msg.GetPassword(), req.Header().Get("User-Agent"))
	if err != nil {
		return nil, apperr.ToConnect(err)
	}
	res := connect.NewResponse(&werbotv1.LoginResponse{
		MfaRequired: r.MFAToken != "", MfaToken: r.MFAToken, Methods: r.Methods, WebauthnOptions: r.WebAuthnOptions,
	})
	if r.Token != "" {
		res.Header().Add("Set-Cookie", h.sessionCookie(r.Token, sessionMaxAge))
	}
	return res, nil
}

// CompleteMFA is public: the mfa_token is the credential. It sets the session cookie.
func (h *Handler) CompleteMFA(ctx context.Context, req *connect.Request[werbotv1.CompleteMFARequest]) (*connect.Response[werbotv1.CompleteMFAResponse], error) {
	m := req.Msg
	proof := MFAProof{TOTP: m.GetTotpCode(), RecoveryCode: m.GetRecoveryCode(), Assertion: m.GetWebauthnAssertion()}
	token, err := h.svc.CompleteMFA(ctx, m.GetMfaToken(), proof, req.Header().Get("User-Agent"))
	if err != nil {
		return nil, apperr.ToConnect(err)
	}
	res := connect.NewResponse(&werbotv1.CompleteMFAResponse{})
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
		User:                  userToProto(info.User),
		Organization:          &werbotv1.Organization{Id: info.OrgID.String(), Name: info.OrgName},
		Role:                  roleToProto(info.Role),
		MfaEnrollmentRequired: EnrollmentRequired(ctx),
	}), nil
}

// RequestPasswordReset is public and answers the same for known and unknown emails.
func (h *Handler) RequestPasswordReset(ctx context.Context, req *connect.Request[werbotv1.RequestPasswordResetRequest]) (*connect.Response[werbotv1.RequestPasswordResetResponse], error) {
	if err := h.svc.RequestPasswordReset(ctx, req.Msg.GetEmail()); err != nil {
		return nil, apperr.ToConnect(err)
	}
	return connect.NewResponse(&werbotv1.RequestPasswordResetResponse{}), nil
}

// ResetPassword is public: the one-time token is the credential.
func (h *Handler) ResetPassword(ctx context.Context, req *connect.Request[werbotv1.ResetPasswordRequest]) (*connect.Response[werbotv1.ResetPasswordResponse], error) {
	if err := h.svc.ResetPassword(ctx, req.Msg.GetToken(), req.Msg.GetNewPassword()); err != nil {
		return nil, apperr.ToConnect(err)
	}
	return connect.NewResponse(&werbotv1.ResetPasswordResponse{}), nil
}

// BeginStepUp starts a step-up of the caller's session or CLI token.
func (h *Handler) BeginStepUp(ctx context.Context, _ *connect.Request[werbotv1.BeginStepUpRequest]) (*connect.Response[werbotv1.BeginStepUpResponse], error) {
	p, err := principal.Require(ctx)
	if err != nil {
		return nil, apperr.ToConnect(err)
	}
	su, err := h.svc.BeginStepUp(ctx, p)
	if err != nil {
		return nil, apperr.ToConnect(err)
	}
	return connect.NewResponse(&werbotv1.BeginStepUpResponse{
		Methods: su.Methods, WebauthnOptions: su.WebAuthnOptions, Url: su.URL, Code: su.Code,
	}), nil
}

// CompleteStepUp records a fresh second factor of the caller's session or CLI token.
func (h *Handler) CompleteStepUp(ctx context.Context, req *connect.Request[werbotv1.CompleteStepUpRequest]) (*connect.Response[werbotv1.CompleteStepUpResponse], error) {
	p, err := principal.Require(ctx)
	if err != nil {
		return nil, apperr.ToConnect(err)
	}
	proof := MFAProof{TOTP: req.Msg.GetTotpCode(), Assertion: req.Msg.GetWebauthnAssertion()}
	if err := h.svc.CompleteStepUp(ctx, p, proof); err != nil {
		return nil, apperr.ToConnect(err)
	}
	return connect.NewResponse(&werbotv1.CompleteStepUpResponse{}), nil
}

// ListIdentityProviders is public: the login page offers the enabled IdPs.
func (h *Handler) ListIdentityProviders(ctx context.Context, _ *connect.Request[werbotv1.AuthServiceListIdentityProvidersRequest]) (*connect.Response[werbotv1.AuthServiceListIdentityProvidersResponse], error) {
	refs, err := h.svc.IdentityProviders(ctx)
	if err != nil {
		return nil, apperr.ToConnect(err)
	}
	res := &werbotv1.AuthServiceListIdentityProvidersResponse{}
	for _, r := range refs {
		res.Providers = append(res.Providers, &werbotv1.IdentityProviderRef{Id: r.ID.String(), Name: r.Name})
	}
	return connect.NewResponse(res), nil
}
