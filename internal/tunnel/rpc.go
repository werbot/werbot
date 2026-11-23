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
	"github.com/werbot/werbot/internal/platform/sshca"
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

// AuthenticateUser resolves an SSH client, or the user of a web ticket, in
// the caller's organization; a web ticket's country is the browser's.
func (h *Handler) AuthenticateUser(ctx context.Context, req *connect.Request[gatewayv1.AuthenticateUserRequest]) (*connect.Response[gatewayv1.AuthenticateUserResponse], error) {
	g, err := caller(ctx)
	if err != nil {
		return nil, err
	}
	ip, err := parseIP(req.Msg.GetClientIp())
	if err != nil {
		return nil, err
	}
	u, wt, err := h.userBy(ctx, g, req.Msg)
	if err != nil {
		return nil, h.fail(ctx, "authenticate user", err)
	}
	if wt != nil {
		ip = wt.ClientIP
	}
	teams := make([]string, len(u.TeamIDs))
	for i, id := range u.TeamIDs {
		teams[i] = id.String()
	}
	res := &gatewayv1.AuthenticateUserResponse{UserId: u.ID.String(), Login: u.Login, TeamIds: teams,
		Country: h.d.Country(ip), Agent: agentInfo(u), Web: webTicketOf(wt)}
	if err := h.setMFAAge(ctx, g, req.Msg, u, wt, res); err != nil {
		return nil, h.fail(ctx, "mfa age", err)
	}
	return connect.NewResponse(res), nil
}

// userBy resolves the identity m names; the web ticket is set only for one.
func (h *Handler) userBy(ctx context.Context, g Gateway, m *gatewayv1.AuthenticateUserRequest) (User, *WebTicket, error) {
	switch by := m.GetBy().(type) {
	case *gatewayv1.AuthenticateUserRequest_FingerprintSha256:
		u, err := h.d.Users.ByFingerprint(ctx, g.OrgID, by.FingerprintSha256)
		return u, nil, err
	case *gatewayv1.AuthenticateUserRequest_Login:
		u, err := h.d.Users.ByLogin(ctx, g.OrgID, by.Login)
		return u, nil, err
	case *gatewayv1.AuthenticateUserRequest_Cert:
		u, err := h.byCert(ctx, g.OrgID, by.Cert)
		return u, nil, err
	case *gatewayv1.AuthenticateUserRequest_WebTicket:
		return h.userByWebTicket(ctx, g, by.WebTicket)
	}
	return User{}, nil, invalid("by")
}

// byCert resolves a certificate the gateway verified, with its agent run when
// it has one; the organization is always the caller's, never one the gateway names.
func (h *Handler) byCert(ctx context.Context, orgID uuid.UUID, c *gatewayv1.CertIdentity) (User, error) {
	id := sshca.Identity{OrgID: orgID, Login: c.GetLogin()}
	var err error
	if id.UserID, err = parseID(c.GetUserId(), "user_id"); err != nil {
		return User{}, err
	}
	if id.CLISessionID, err = parseID(c.GetCliSessionId(), "cli_session_id"); err != nil {
		return User{}, err
	}
	if err = parseAgent(c, &id); err != nil {
		return User{}, err
	}
	return h.d.Users.ByCert(ctx, orgID, id)
}

// StartSession reveals the credential first, so a failed reveal never leaves
// an open session row, then records the session with its kind and target; an
// agent's session counts an action of its run (Sessions.Start refuses it).
func (h *Handler) StartSession(ctx context.Context, req *connect.Request[gatewayv1.StartSessionRequest]) (*connect.Response[gatewayv1.StartSessionResponse], error) {
	g, err := caller(ctx)
	if err != nil {
		return nil, err
	}
	s, err := sessionStartOf(g, req.Msg)
	if err != nil {
		return nil, err
	}
	typ, err := h.ownResource(ctx, g, s.ResourceID)
	if err != nil {
		return nil, err
	}
	secret, err := h.d.Credentials.Reveal(ctx, g.OrgID, s.ResourceID, typ)
	if err != nil {
		return nil, h.fail(ctx, "reveal credential", err)
	}
	id, err := h.d.Sessions.Start(ctx, s)
	if err != nil {
		return nil, h.fail(ctx, "start session", err)
	}
	return connect.NewResponse(&gatewayv1.StartSessionResponse{SessionId: id.String(),
		Credential: &gatewayv1.Credential{Kind: secret.Kind, Value: secret.Value}}), nil
}

// sessionStartOf is the session m asks g to record; an invalid id or IP is InvalidArgument.
func sessionStartOf(g Gateway, m *gatewayv1.StartSessionRequest) (s SessionStart, err error) {
	s = SessionStart{OrgID: g.OrgID, GatewayID: g.ID, InstanceID: g.InstanceID, Kind: m.GetKind(), Target: m.GetTarget(),
		Front: m.GetFront(), Command: m.GetCommand(), Classes: m.GetClasses(), Paths: m.GetPaths()}
	if s.UserID, err = parseID(m.GetUserId(), "user_id"); err != nil {
		return s, err
	}
	if s.ResourceID, err = parseID(m.GetResourceId(), "resource_id"); err != nil {
		return s, err
	}
	if s.ClientIP, err = parseIP(m.GetClientIp()); err != nil {
		return s, err
	}
	if s.AgentRunID, err = optionalID(m.GetAgentRunId(), "agent_run_id"); err != nil {
		return s, err
	}
	s.ToolCallID, err = optionalID(m.GetToolCallId(), "tool_call_id")
	return s, err
}

// ownResource denies resources outside the caller's pool and returns the type of one inside it.
func (h *Handler) ownResource(ctx context.Context, g Gateway, resourceID uuid.UUID) (string, error) {
	typ, ok, err := h.d.Resources.OwnedBy(ctx, g.OrgID, g.ID, resourceID)
	if err != nil {
		return "", h.fail(ctx, "resource owner", err)
	}
	if !ok {
		return "", connect.NewError(connect.CodePermissionDenied, errors.New("resource is not served by this gateway"))
	}
	return typ, nil
}

// EndSession closes a session of the calling instance with its final counters
// and its result, if sent: a console's last command, an exec's exit code and output hash.
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
		if cid, err := uuid.Parse(m.GetSessionId()); err != nil || cid != id {
			return nil, invalid("counters.session_id")
		}
		v := countersOf(id, m)
		c = &v
	}
	if err := h.d.Sessions.End(ctx, g.OrgID, g.InstanceID, id, req.Msg.GetReason(), c, resultOf(req.Msg)); err != nil {
		return nil, h.fail(ctx, "end session", err)
	}
	return connect.NewResponse(&gatewayv1.EndSessionResponse{}), nil
}

// resultOf is the result of m; nil without a command, an exit code and an output hash.
func resultOf(m *gatewayv1.EndSessionRequest) *SessionResult {
	if m.GetCommand() == "" && m.ExitCode == nil && len(m.GetOutputSha256()) == 0 {
		return nil
	}
	return &SessionResult{Command: m.GetCommand(), Classes: m.GetClasses(), Paths: m.GetPaths(),
		ExitCode: m.ExitCode, OutputSHA256: m.GetOutputSha256()}
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
	if _, err := h.ownResource(ctx, g, resourceID); err != nil {
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
