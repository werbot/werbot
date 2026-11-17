package tunnel

import (
	"context"
	"errors"
	"net/netip"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	gossh "golang.org/x/crypto/ssh"

	gatewayv1 "github.com/werbot/werbot/gen/gateway/v1"
	"github.com/werbot/werbot/internal/platform/apperr"
)

// fail maps err to a Connect error and logs causes the client cannot see.
func (h *Handler) fail(ctx context.Context, op string, err error) error {
	ce := apperr.ToConnect(err)
	if code := connect.CodeOf(ce); code == connect.CodeInternal || code == connect.CodeUnavailable {
		h.log.ErrorContext(ctx, "tunnel: "+op, "error", err)
	}
	return ce
}

func invalid(field string) error {
	return connect.NewError(connect.CodeInvalidArgument, errors.New("invalid "+field))
}

func parseIP(s string) (netip.Addr, error) {
	ip, err := netip.ParseAddr(s)
	if err != nil {
		return netip.Addr{}, invalid("client_ip")
	}
	return ip.Unmap(), nil
}

func parseID(s, field string) (uuid.UUID, error) {
	id, err := uuid.Parse(s)
	if err != nil {
		return uuid.Nil, invalid(field)
	}
	return id, nil
}

// AuthenticateUser resolves an SSH client in the caller's organization.
func (h *Handler) AuthenticateUser(ctx context.Context, req *connect.Request[gatewayv1.AuthenticateUserRequest]) (*connect.Response[gatewayv1.AuthenticateUserResponse], error) {
	g, err := caller(ctx)
	if err != nil {
		return nil, err
	}
	ip, err := parseIP(req.Msg.GetClientIp())
	if err != nil {
		return nil, err
	}
	var u User
	switch by := req.Msg.GetBy().(type) {
	case *gatewayv1.AuthenticateUserRequest_FingerprintSha256:
		u, err = h.d.Users.ByFingerprint(ctx, g.OrgID, by.FingerprintSha256)
	case *gatewayv1.AuthenticateUserRequest_Login:
		u, err = h.d.Users.ByLogin(ctx, g.OrgID, by.Login)
	default:
		return nil, invalid("by")
	}
	if err != nil {
		return nil, h.fail(ctx, "authenticate user", err)
	}
	teams := make([]string, len(u.TeamIDs))
	for i, id := range u.TeamIDs {
		teams[i] = id.String()
	}
	return connect.NewResponse(&gatewayv1.AuthenticateUserResponse{
		UserId: u.ID.String(), Login: u.Login, TeamIds: teams, Country: h.d.Country(ip)}), nil
}

// StartSession reveals the credential first, so a failed reveal never leaves
// an open session row, then records the session with its kind and target.
func (h *Handler) StartSession(ctx context.Context, req *connect.Request[gatewayv1.StartSessionRequest]) (*connect.Response[gatewayv1.StartSessionResponse], error) {
	g, err := caller(ctx)
	if err != nil {
		return nil, err
	}
	userID, err := parseID(req.Msg.GetUserId(), "user_id")
	if err != nil {
		return nil, err
	}
	resourceID, err := parseID(req.Msg.GetResourceId(), "resource_id")
	if err != nil {
		return nil, err
	}
	ip, err := parseIP(req.Msg.GetClientIp())
	if err != nil {
		return nil, err
	}
	if err := h.ownResource(ctx, g, resourceID); err != nil {
		return nil, err
	}
	secret, err := h.d.Credentials.Reveal(ctx, g.OrgID, resourceID)
	if err != nil {
		return nil, h.fail(ctx, "reveal credential", err)
	}
	id, err := h.d.Sessions.Start(ctx, SessionStart{OrgID: g.OrgID, GatewayID: g.ID, InstanceID: g.InstanceID,
		UserID: userID, ResourceID: resourceID, ClientIP: ip, Kind: req.Msg.GetKind(), Target: req.Msg.GetTarget()})
	if err != nil {
		return nil, h.fail(ctx, "start session", err)
	}
	return connect.NewResponse(&gatewayv1.StartSessionResponse{SessionId: id.String(),
		Credential: &gatewayv1.Credential{Kind: secret.Kind, Value: secret.Value}}), nil
}

// ownResource denies resources outside the caller's pool.
func (h *Handler) ownResource(ctx context.Context, g Gateway, resourceID uuid.UUID) error {
	ok, err := h.d.Resources.OwnedBy(ctx, g.OrgID, g.ID, resourceID)
	if err != nil {
		return h.fail(ctx, "resource owner", err)
	}
	if !ok {
		return connect.NewError(connect.CodePermissionDenied, errors.New("resource is not served by this gateway"))
	}
	return nil
}

// EndSession closes a session of the calling instance with its final counters, if sent.
func (h *Handler) EndSession(ctx context.Context, req *connect.Request[gatewayv1.EndSessionRequest]) (*connect.Response[gatewayv1.EndSessionResponse], error) {
	g, err := caller(ctx)
	if err != nil {
		return nil, err
	}
	id, err := parseID(req.Msg.GetSessionId(), "session_id")
	if err != nil {
		return nil, err
	}
	var c *Counters
	if m := req.Msg.GetCounters(); m != nil {
		v := countersOf(id, m)
		c = &v
	}
	if err := h.d.Sessions.End(ctx, g.OrgID, g.InstanceID, id, req.Msg.GetReason(), c); err != nil {
		return nil, h.fail(ctx, "end session", err)
	}
	return connect.NewResponse(&gatewayv1.EndSessionResponse{}), nil
}

// PinHostKey records the target host key seen on first connect.
func (h *Handler) PinHostKey(ctx context.Context, req *connect.Request[gatewayv1.PinHostKeyRequest]) (*connect.Response[gatewayv1.PinHostKeyResponse], error) {
	g, err := caller(ctx)
	if err != nil {
		return nil, err
	}
	resourceID, err := parseID(req.Msg.GetResourceId(), "resource_id")
	if err != nil {
		return nil, err
	}
	key, err := gossh.ParsePublicKey(req.Msg.GetPublicKey())
	if err != nil {
		return nil, invalid("public_key")
	}
	if key.Type() != req.Msg.GetAlg() {
		return nil, invalid("alg")
	}
	if err := h.ownResource(ctx, g, resourceID); err != nil {
		return nil, err
	}
	if err := h.d.Resources.PinHostKey(ctx, g.OrgID, resourceID, key); err != nil {
		return nil, h.fail(ctx, "pin host key", err)
	}
	return connect.NewResponse(&gatewayv1.PinHostKeyResponse{}), nil
}

// RenewCertificate reissues the caller's certificate; the instance comes only
// from the verified client certificate.
func (h *Handler) RenewCertificate(ctx context.Context, req *connect.Request[gatewayv1.RenewCertificateRequest]) (*connect.Response[gatewayv1.RenewCertificateResponse], error) {
	g, err := caller(ctx)
	if err != nil {
		return nil, err
	}
	certPEM, err := h.d.Gateways.Renew(ctx, g.InstanceID, req.Msg.GetCsrDer())
	if err != nil {
		return nil, h.fail(ctx, "renew certificate", err)
	}
	return connect.NewResponse(&gatewayv1.RenewCertificateResponse{CertPem: certPEM}), nil
}
