package oauth

import (
	"context"

	"connectrpc.com/connect"
	"github.com/google/uuid"

	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
	"github.com/werbot/werbot/gen/werbot/v1/werbotv1connect"
	"github.com/werbot/werbot/internal/platform/apperr"
	"github.com/werbot/werbot/internal/platform/principal"
)

// ConsentHandler serves OAuthConsentService.
type ConsentHandler struct{ p *Provider }

var _ werbotv1connect.OAuthConsentServiceHandler = (*ConsentHandler)(nil)

// NewConsentHandler returns the OAuthConsentService handler of p.
func NewConsentHandler(p *Provider) *ConsentHandler { return &ConsentHandler{p: p} }

// subjectAndID returns the caller and the request id (validated as a UUID).
func subjectAndID(ctx context.Context, id string) (principal.Principal, uuid.UUID, error) {
	pr, err := principal.Require(ctx)
	if err != nil {
		return principal.Principal{}, uuid.Nil, err
	}
	uid, err := uuid.Parse(id)
	if err != nil {
		return principal.Principal{}, uuid.Nil, apperr.ErrInvalidArgument
	}
	return pr, uid, nil
}

// GetAuthRequest shows the pending sign-in id.
func (h *ConsentHandler) GetAuthRequest(ctx context.Context, req *connect.Request[werbotv1.GetAuthRequestRequest]) (*connect.Response[werbotv1.GetAuthRequestResponse], error) {
	pr, id, err := subjectAndID(ctx, req.Msg.GetId())
	if err != nil {
		return nil, apperr.ToConnect(err)
	}
	info, err := h.p.GetAuthRequest(ctx, pr, id)
	if err != nil {
		return nil, apperr.ToConnect(err)
	}
	res := &werbotv1.GetAuthRequestResponse{ClientId: info.ClientID, ClientHostname: info.Hostname, Scopes: info.Scopes}
	if info.IP.IsValid() {
		res.ClientIp = info.IP.String()
	}
	return connect.NewResponse(res), nil
}

// ApproveAuthRequest approves the sign-in id as the caller.
func (h *ConsentHandler) ApproveAuthRequest(ctx context.Context, req *connect.Request[werbotv1.ApproveAuthRequestRequest]) (*connect.Response[werbotv1.ApproveAuthRequestResponse], error) {
	pr, id, err := subjectAndID(ctx, req.Msg.GetId())
	if err != nil {
		return nil, apperr.ToConnect(err)
	}
	u, err := h.p.ApproveAuthRequest(ctx, pr, id)
	if err != nil {
		return nil, apperr.ToConnect(err)
	}
	return connect.NewResponse(&werbotv1.ApproveAuthRequestResponse{RedirectUrl: u}), nil
}

// DenyAuthRequest refuses the sign-in id.
func (h *ConsentHandler) DenyAuthRequest(ctx context.Context, req *connect.Request[werbotv1.DenyAuthRequestRequest]) (*connect.Response[werbotv1.DenyAuthRequestResponse], error) {
	pr, id, err := subjectAndID(ctx, req.Msg.GetId())
	if err != nil {
		return nil, apperr.ToConnect(err)
	}
	u, err := h.p.DenyAuthRequest(ctx, pr, id)
	if err != nil {
		return nil, apperr.ToConnect(err)
	}
	return connect.NewResponse(&werbotv1.DenyAuthRequestResponse{RedirectUrl: u}), nil
}

// LookupDeviceCode shows the pending device sign-in of the typed user code.
func (h *ConsentHandler) LookupDeviceCode(ctx context.Context, req *connect.Request[werbotv1.LookupDeviceCodeRequest]) (*connect.Response[werbotv1.LookupDeviceCodeResponse], error) {
	pr, err := principal.Require(ctx)
	if err != nil {
		return nil, apperr.ToConnect(err)
	}
	info, err := h.p.LookupDeviceCode(ctx, pr, req.Msg.GetUserCode())
	if err != nil {
		return nil, apperr.ToConnect(err)
	}
	res := &werbotv1.LookupDeviceCodeResponse{ClientId: info.ClientID, ClientHostname: info.Hostname, Scopes: info.Scopes}
	if info.IP.IsValid() {
		res.ClientIp = info.IP.String()
	}
	return connect.NewResponse(res), nil
}

// ApproveDeviceCode approves the device sign-in as the caller.
func (h *ConsentHandler) ApproveDeviceCode(ctx context.Context, req *connect.Request[werbotv1.ApproveDeviceCodeRequest]) (*connect.Response[werbotv1.ApproveDeviceCodeResponse], error) {
	pr, err := principal.Require(ctx)
	if err != nil {
		return nil, apperr.ToConnect(err)
	}
	if err := h.p.ApproveDeviceCode(ctx, pr, req.Msg.GetUserCode()); err != nil {
		return nil, apperr.ToConnect(err)
	}
	return connect.NewResponse(&werbotv1.ApproveDeviceCodeResponse{}), nil
}

// DenyDeviceCode refuses the device sign-in.
func (h *ConsentHandler) DenyDeviceCode(ctx context.Context, req *connect.Request[werbotv1.DenyDeviceCodeRequest]) (*connect.Response[werbotv1.DenyDeviceCodeResponse], error) {
	pr, err := principal.Require(ctx)
	if err != nil {
		return nil, apperr.ToConnect(err)
	}
	if err := h.p.DenyDeviceCode(ctx, pr, req.Msg.GetUserCode()); err != nil {
		return nil, apperr.ToConnect(err)
	}
	return connect.NewResponse(&werbotv1.DenyDeviceCodeResponse{}), nil
}
