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

// ConsentHandler serves OAuthConsentService; the device methods answer
// CodeUnimplemented through the embedded handler until the device flow lands.
type ConsentHandler struct {
	werbotv1connect.UnimplementedOAuthConsentServiceHandler
	p *Provider
}

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
