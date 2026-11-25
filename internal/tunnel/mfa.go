package tunnel

import (
	"context"
	"errors"
	"math"
	"net/netip"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/timestamppb"

	gatewayv1 "github.com/werbot/werbot/gen/gateway/v1"
)

// NoMFA is the MFA age of a request without a second factor for its binding.
const NoMFA = math.MaxInt64

// MFAChallengeRequest asks UserID to confirm an SSH login of instance
// InstanceID to ResourceID; Binding is the verified key or certificate.
type MFAChallengeRequest struct {
	OrgID, InstanceID, UserID, ResourceID uuid.UUID
	SSHSessionID                          []byte
	ClientIP                              netip.Addr
	Binding, Strength                     string
}

// MFA stores the MFA challenges of gateway instances and the second factors
// of their bindings.
type MFA interface {
	Create(ctx context.Context, r MFAChallengeRequest) (id uuid.UUID, code, url string, expiresAt time.Time, err error)
	// VerifyTOTP: a wrong code is false; another instance's challenge ErrNotFound.
	VerifyTOTP(ctx context.Context, orgID, instanceID, id uuid.UUID, code string) (bool, error)
	Status(ctx context.Context, orgID, instanceID, id uuid.UUID) (string, error) // another instance's: ErrNotFound
	Confirmed(ctx context.Context, orgID, instanceID uuid.UUID) ([]uuid.UUID, error)
	// Age is the age in seconds and the method of userID's last second factor for binding; none: NoMFA, "".
	Age(ctx context.Context, orgID, userID uuid.UUID, binding string) (seconds int64, method string, err error)
}

var errMFAOff = connect.NewError(connect.CodeUnimplemented, errors.New("mfa challenges are not served"))

// CreateMFAChallenge asks the user to confirm an SSH login to a resource of
// the caller's pool; the confirmation arrives as mfa_confirmed on Connect.
func (h *Handler) CreateMFAChallenge(ctx context.Context, req *connect.Request[gatewayv1.CreateMFAChallengeRequest]) (*connect.Response[gatewayv1.CreateMFAChallengeResponse], error) {
	g, err := caller(ctx)
	if err != nil {
		return nil, err
	}
	if h.d.MFA == nil {
		return nil, errMFAOff
	}
	m := req.Msg
	r := MFAChallengeRequest{OrgID: g.OrgID, InstanceID: g.InstanceID, SSHSessionID: m.GetSshSessionId(), Binding: m.GetBinding(), Strength: m.GetStrength()}
	if r.UserID, err = parseID(m.GetUserId(), "user_id"); err != nil {
		return nil, err
	}
	if r.ResourceID, err = parseID(m.GetResourceId(), "resource_id"); err != nil {
		return nil, err
	}
	if r.ClientIP, err = parseIP(m.GetClientIp()); err != nil {
		return nil, err
	}
	if _, err := h.ownResource(ctx, g, r.ResourceID); err != nil {
		return nil, err
	}
	id, code, url, exp, err := h.d.MFA.Create(ctx, r)
	if err != nil {
		return nil, h.fail(ctx, "create mfa challenge", err)
	}
	return connect.NewResponse(&gatewayv1.CreateMFAChallengeResponse{ChallengeId: id.String(), Code: code, Url: url,
		ExpiresAt: timestamppb.New(exp)}), nil
}

// VerifyMFATOTP confirms a challenge of the calling instance with a TOTP code.
func (h *Handler) VerifyMFATOTP(ctx context.Context, req *connect.Request[gatewayv1.VerifyMFATOTPRequest]) (*connect.Response[gatewayv1.VerifyMFATOTPResponse], error) {
	g, id, err := h.mfaCaller(ctx, req.Msg.GetChallengeId())
	if err != nil {
		return nil, err
	}
	ok, err := h.d.MFA.VerifyTOTP(ctx, g.OrgID, g.InstanceID, id, req.Msg.GetCode())
	if err != nil {
		return nil, h.fail(ctx, "verify mfa totp", err)
	}
	return connect.NewResponse(&gatewayv1.VerifyMFATOTPResponse{Ok: ok}), nil
}

// GetMFAChallenge returns the status of a challenge of the calling instance.
func (h *Handler) GetMFAChallenge(ctx context.Context, req *connect.Request[gatewayv1.GetMFAChallengeRequest]) (*connect.Response[gatewayv1.GetMFAChallengeResponse], error) {
	g, id, err := h.mfaCaller(ctx, req.Msg.GetChallengeId())
	if err != nil {
		return nil, err
	}
	status, err := h.d.MFA.Status(ctx, g.OrgID, g.InstanceID, id)
	if err != nil {
		return nil, h.fail(ctx, "get mfa challenge", err)
	}
	return connect.NewResponse(&gatewayv1.GetMFAChallengeResponse{Status: status}), nil
}

// mfaCaller is the caller of a challenge RPC and the challenge id it names.
func (h *Handler) mfaCaller(ctx context.Context, challengeID string) (Gateway, uuid.UUID, error) {
	g, err := caller(ctx)
	if err != nil {
		return Gateway{}, uuid.Nil, err
	}
	if h.d.MFA == nil {
		return Gateway{}, uuid.Nil, errMFAOff
	}
	id, err := parseID(challengeID, "challenge_id")
	return g, id, err
}

// setMFAAge sets the age and method of u's last second factor for the
// binding of m on res: key:<fingerprint>, cert:<certificate fingerprint> or
// web:<the ticket's cookie session>. A login, an agent and a request without
// a binding have none (NoMFA); without MFA the fields stay unset.
func (h *Handler) setMFAAge(ctx context.Context, g Gateway, m *gatewayv1.AuthenticateUserRequest, u User, wt *WebTicket, res *gatewayv1.AuthenticateUserResponse) error {
	if h.d.MFA == nil {
		return nil
	}
	res.MfaAgeSeconds = NoMFA
	binding := mfaBinding(m, wt)
	if binding == "" || u.Agent != nil {
		return nil
	}
	var err error
	res.MfaAgeSeconds, res.MfaMethod, err = h.d.MFA.Age(ctx, g.OrgID, u.ID, binding)
	return err
}

// mfaBinding is what a second factor of the request m is recorded for; empty: nothing.
func mfaBinding(m *gatewayv1.AuthenticateUserRequest, wt *WebTicket) string {
	switch by := m.GetBy().(type) {
	case *gatewayv1.AuthenticateUserRequest_FingerprintSha256:
		return "key:" + by.FingerprintSha256
	case *gatewayv1.AuthenticateUserRequest_Cert:
		if fp := by.Cert.GetCertFingerprint(); fp != "" {
			return "cert:" + fp
		}
	case *gatewayv1.AuthenticateUserRequest_WebTicket:
		if wt != nil && wt.UserSessionID != nil {
			return "web:" + wt.UserSessionID.String()
		}
	}
	return ""
}

// MFAConfirmed queues mfa_confirmed for payload "<instance_id>:<challenge_id>"
// on that instance's streams on this replica; it never reads the database.
// A full queue closes the stream with "overflow": the reconnected gateway
// gets the confirmation resent.
func (h *Hub) MFAConfirmed(payload string) {
	inst, id, ok := h.parsePayload("mfa_confirmed", payload)
	if ok && h.enqueue(inst, ctlMsg{kind: mfaConfirmed, id: id}, true) {
		h.logger().Warn("tunnel: mfa confirmation overflow", "instance_id", inst)
	}
}

// resendMFAConfirmed sends mfa_confirmed for every confirmed, unexpired
// challenge of g; a failed read only logs, the stream lives.
func (h *Handler) resendMFAConfirmed(ctx context.Context, st *connectStream, g Gateway) error {
	if h.d.MFA == nil {
		return nil
	}
	ids, err := h.d.MFA.Confirmed(ctx, g.OrgID, g.InstanceID)
	if err != nil {
		h.log.Warn("tunnel: resend mfa confirmations", "instance_id", g.InstanceID, "error", err)
		return nil
	}
	for _, id := range ids {
		if err := sendMFAConfirmed(st, id); err != nil {
			return err
		}
	}
	return nil
}

func sendMFAConfirmed(st *connectStream, id uuid.UUID) error {
	return st.Send(&gatewayv1.ConnectResponse{Msg: &gatewayv1.ConnectResponse_MfaConfirmed{
		MfaConfirmed: &gatewayv1.MFAConfirmed{ChallengeId: id.String()}}})
}
