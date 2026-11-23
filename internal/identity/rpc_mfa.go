package identity

import (
	"context"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
	"github.com/werbot/werbot/internal/platform/apperr"
	"github.com/werbot/werbot/internal/platform/principal"
)

func factorToProto(f Factor) *werbotv1.Factor {
	pf := &werbotv1.Factor{Id: f.ID, Kind: f.Kind, Name: f.Name, Remaining: int32(f.Remaining)} //nolint:gosec // at most RecoveryCodeCount
	if !f.CreatedAt.IsZero() {
		pf.CreatedAt = timestamppb.New(f.CreatedAt)
	}
	if f.LastUsedAt != nil {
		pf.LastUsedAt = timestamppb.New(*f.LastUsedAt)
	}
	return pf
}

// ListFactors returns the caller's second factors.
func (h *Handler) ListFactors(ctx context.Context, _ *connect.Request[werbotv1.ListFactorsRequest]) (*connect.Response[werbotv1.ListFactorsResponse], error) {
	p, err := principal.Require(ctx)
	if err != nil {
		return nil, apperr.ToConnect(err)
	}
	fs, err := h.svc.ListFactors(ctx, p)
	if err != nil {
		return nil, apperr.ToConnect(err)
	}
	res := &werbotv1.ListFactorsResponse{}
	for _, f := range fs {
		res.Factors = append(res.Factors, factorToProto(f))
	}
	return connect.NewResponse(res), nil
}

// EnrollTOTP starts the caller's TOTP enrollment; the response is the only
// place the secret appears.
func (h *Handler) EnrollTOTP(ctx context.Context, _ *connect.Request[werbotv1.EnrollTOTPRequest]) (*connect.Response[werbotv1.EnrollTOTPResponse], error) {
	p, err := principal.Require(ctx)
	if err != nil {
		return nil, apperr.ToConnect(err)
	}
	e, err := h.svc.EnrollTOTP(ctx, p)
	if err != nil {
		return nil, apperr.ToConnect(err)
	}
	return connect.NewResponse(&werbotv1.EnrollTOTPResponse{
		OtpauthUrl: e.URL, Secret: e.Secret, QrSize: int32(e.QRSize), QrPath: e.QRPath, //nolint:gosec // a QR code has at most 177 modules
	}), nil
}

// ConfirmTOTP confirms the caller's TOTP enrollment with a current code.
func (h *Handler) ConfirmTOTP(ctx context.Context, req *connect.Request[werbotv1.ConfirmTOTPRequest]) (*connect.Response[werbotv1.ConfirmTOTPResponse], error) {
	p, err := principal.Require(ctx)
	if err != nil {
		return nil, apperr.ToConnect(err)
	}
	if err := h.svc.ConfirmTOTP(ctx, p, req.Msg.GetCode()); err != nil {
		return nil, apperr.ToConnect(err)
	}
	return connect.NewResponse(&werbotv1.ConfirmTOTPResponse{}), nil
}

// GenerateRecoveryCodes replaces the caller's recovery codes; the response
// is the only place the codes appear.
func (h *Handler) GenerateRecoveryCodes(ctx context.Context, _ *connect.Request[werbotv1.GenerateRecoveryCodesRequest]) (*connect.Response[werbotv1.GenerateRecoveryCodesResponse], error) {
	p, err := principal.Require(ctx)
	if err != nil {
		return nil, apperr.ToConnect(err)
	}
	codes, err := h.svc.GenerateRecoveryCodes(ctx, p)
	if err != nil {
		return nil, apperr.ToConnect(err)
	}
	return connect.NewResponse(&werbotv1.GenerateRecoveryCodesResponse{Codes: codes}), nil
}

// DeleteFactor removes one of the caller's factors.
func (h *Handler) DeleteFactor(ctx context.Context, req *connect.Request[werbotv1.DeleteFactorRequest]) (*connect.Response[werbotv1.DeleteFactorResponse], error) {
	p, err := principal.Require(ctx)
	if err != nil {
		return nil, apperr.ToConnect(err)
	}
	if err := h.svc.DeleteFactor(ctx, p, req.Msg.GetId()); err != nil {
		return nil, apperr.ToConnect(err)
	}
	return connect.NewResponse(&werbotv1.DeleteFactorResponse{}), nil
}
