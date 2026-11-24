package gateway

import (
	"context"

	"connectrpc.com/connect"

	gatewayv1 "github.com/werbot/werbot/gen/gateway/v1"
)

var _ mfaControl = (*tunnelClient)(nil)

// CreateMFAChallenge asks the user to confirm a login with a second factor;
// the confirmation comes as a hint on the stream or from GetMFAChallenge.
func (c *tunnelClient) CreateMFAChallenge(ctx context.Context, r challengeRequest) (mfaTicket, error) {
	ctx, cancel := context.WithTimeout(ctx, c.callTimeout)
	defer cancel()
	resp, err := c.conn.Load().rpc.CreateMFAChallenge(ctx, connect.NewRequest(&gatewayv1.CreateMFAChallengeRequest{
		UserId: r.UserID.String(), ResourceId: r.ResourceID.String(), SshSessionId: r.SSHSessionID, ClientIp: r.ClientIP.String(),
		Binding: r.Binding, Strength: r.Strength,
	}))
	if err != nil {
		return mfaTicket{}, err
	}
	m := resp.Msg
	return mfaTicket{ID: m.GetChallengeId(), Code: m.GetCode(), URL: m.GetUrl(), ExpiresAt: m.GetExpiresAt().AsTime()}, nil
}

// VerifyMFATOTP meets challenge id with a TOTP code; a wrong code is false,
// too many attempts ResourceExhausted.
func (c *tunnelClient) VerifyMFATOTP(ctx context.Context, id, code string) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, c.callTimeout)
	defer cancel()
	resp, err := c.conn.Load().rpc.VerifyMFATOTP(ctx, connect.NewRequest(&gatewayv1.VerifyMFATOTPRequest{ChallengeId: id, Code: code}))
	if err != nil {
		return false, err
	}
	return resp.Msg.GetOk(), nil
}

// GetMFAChallenge returns the status of challenge id: pending, confirmed, denied or expired.
func (c *tunnelClient) GetMFAChallenge(ctx context.Context, id string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, c.callTimeout)
	defer cancel()
	resp, err := c.conn.Load().rpc.GetMFAChallenge(ctx, connect.NewRequest(&gatewayv1.GetMFAChallengeRequest{ChallengeId: id}))
	if err != nil {
		return "", err
	}
	return resp.Msg.GetStatus(), nil
}

// SubscribeMFA wakes the returned channel when the stream brings
// mfa_confirmed for challenge id, until unsubscribe is called.
func (c *tunnelClient) SubscribeMFA(id string) (confirmed <-chan struct{}, unsubscribe func()) {
	return c.mfaWaits.subscribe(id)
}
