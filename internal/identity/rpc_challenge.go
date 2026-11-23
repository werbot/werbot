package identity

import (
	"context"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
	"github.com/werbot/werbot/internal/platform/apperr"
	"github.com/werbot/werbot/internal/platform/principal"
)

// GetChallenge returns the caller's pending MFA challenge for the confirmation page.
func (h *Handler) GetChallenge(ctx context.Context, req *connect.Request[werbotv1.GetChallengeRequest]) (*connect.Response[werbotv1.GetChallengeResponse], error) {
	p, err := principal.Require(ctx)
	if err != nil {
		return nil, apperr.ToConnect(err)
	}
	id, err := parseID(req.Msg.GetId())
	if err != nil {
		return nil, apperr.ToConnect(err)
	}
	c, err := h.svc.GetMFAChallenge(ctx, p, id)
	if err != nil {
		return nil, apperr.ToConnect(err)
	}
	res := &werbotv1.GetChallengeResponse{Code: c.Code, Host: c.Host, Resource: c.Resource, Strength: c.Strength,
		ExpiresAt: timestamppb.New(c.ExpiresAt), WebauthnOptions: c.WebAuthnOptions}
	if c.ClientIP.IsValid() {
		res.ClientIp = c.ClientIP.String()
	}
	return connect.NewResponse(res), nil
}

// ConfirmChallenge confirms the caller's MFA challenge with a passkey assertion.
func (h *Handler) ConfirmChallenge(ctx context.Context, req *connect.Request[werbotv1.ConfirmChallengeRequest]) (*connect.Response[werbotv1.ConfirmChallengeResponse], error) {
	p, err := principal.Require(ctx)
	if err != nil {
		return nil, apperr.ToConnect(err)
	}
	id, err := parseID(req.Msg.GetId())
	if err != nil {
		return nil, apperr.ToConnect(err)
	}
	if err := h.svc.ConfirmMFAChallenge(ctx, p, id, req.Msg.GetWebauthnAssertion()); err != nil {
		return nil, apperr.ToConnect(err)
	}
	return connect.NewResponse(&werbotv1.ConfirmChallengeResponse{}), nil
}
