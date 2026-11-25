package gateway

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	gatewayv1 "github.com/werbot/werbot/gen/gateway/v1"
)

func mfaConfirmedMsg(id string) *gatewayv1.ConnectResponse {
	return &gatewayv1.ConnectResponse{Msg: &gatewayv1.ConnectResponse_MfaConfirmed{MfaConfirmed: &gatewayv1.MFAConfirmed{ChallengeId: id}}}
}

// CreateMFAChallenge answers the challenge "c1"; its URL echoes the request fields.
func (*fakeTunnel) CreateMFAChallenge(_ context.Context, r *connect.Request[gatewayv1.CreateMFAChallengeRequest]) (*connect.Response[gatewayv1.CreateMFAChallengeResponse], error) {
	m := r.Msg
	return connect.NewResponse(&gatewayv1.CreateMFAChallengeResponse{ChallengeId: "c1", Code: "BCDF-GHJK", ExpiresAt: timestamppb.New(testApprovalExpiry),
		Url: fmt.Sprintf("%s %s %x %s %s %s", m.GetUserId(), m.GetResourceId(), m.GetSshSessionId(), m.GetClientIp(), m.GetBinding(), m.GetStrength())}), nil
}

// VerifyMFATOTP accepts "123456" for "c1"; "999999" is over the attempt limit.
func (*fakeTunnel) VerifyMFATOTP(_ context.Context, r *connect.Request[gatewayv1.VerifyMFATOTPRequest]) (*connect.Response[gatewayv1.VerifyMFATOTPResponse], error) {
	if r.Msg.GetCode() == "999999" {
		return nil, connect.NewError(connect.CodeResourceExhausted, nil)
	}
	return connect.NewResponse(&gatewayv1.VerifyMFATOTPResponse{Ok: r.Msg.GetChallengeId() == "c1" && r.Msg.GetCode() == "123456"}), nil
}

// GetMFAChallenge knows only "c1", which is confirmed.
func (*fakeTunnel) GetMFAChallenge(_ context.Context, r *connect.Request[gatewayv1.GetMFAChallengeRequest]) (*connect.Response[gatewayv1.GetMFAChallengeResponse], error) {
	if r.Msg.GetChallengeId() != "c1" {
		return nil, connect.NewError(connect.CodeNotFound, nil)
	}
	return connect.NewResponse(&gatewayv1.GetMFAChallengeResponse{Status: "confirmed"}), nil
}
