package inventory

import (
	"context"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
	"github.com/werbot/werbot/gen/werbot/v1/werbotv1connect"
	"github.com/werbot/werbot/internal/platform/apperr"
	"github.com/werbot/werbot/internal/platform/clientip"
)

// GatewayRPC serves GatewayService and GatewayEnrollService.
type GatewayRPC struct {
	werbotv1connect.UnimplementedGatewayServiceHandler
	werbotv1connect.UnimplementedGatewayEnrollServiceHandler
	g         *Gateways
	tunnelURL string
}

// NewGatewayRPC returns the handler; tunnelURL is handed to enrolled gateways.
func NewGatewayRPC(g *Gateways, tunnelURL string) *GatewayRPC {
	return &GatewayRPC{g: g, tunnelURL: tunnelURL}
}

func tsPtr(t *time.Time) *timestamppb.Timestamp {
	if t == nil {
		return nil
	}
	return timestamppb.New(*t)
}

func gatewayToProto(g Gateway) *werbotv1.Gateway {
	out := &werbotv1.Gateway{Id: g.ID.String(), Name: g.Name, Online: g.Online, Connectors: g.Connectors,
		CreatedAt: timestamppb.New(g.CreatedAt), Instances: make([]*werbotv1.GatewayInstance, len(g.Instances))}
	for i, in := range g.Instances {
		pi := &werbotv1.GatewayInstance{Id: in.ID.String(), Hostname: in.Hostname, Version: in.Version, Connectors: in.Connectors,
			Online: in.Online, LastSeenAt: tsPtr(in.LastSeenAt), CertExpiresAt: timestamppb.New(in.CertExpiresAt),
			CreatedAt: timestamppb.New(in.CreatedAt)}
		if in.ClientIP.IsValid() {
			pi.ClientIp = in.ClientIP.String()
		}
		out.Instances[i] = pi
	}
	return out
}

func joinTokenToProto(t JoinToken) *werbotv1.JoinToken {
	out := &werbotv1.JoinToken{Id: t.ID.String(), MaxUses: int32(t.MaxUses), Uses: int32(t.Uses), //nolint:gosec // both come from int32 columns
		ExpiresAt: timestamppb.New(t.ExpiresAt), RevokedAt: tsPtr(t.RevokedAt), CreatedAt: timestamppb.New(t.CreatedAt)}
	if t.CreatedBy != nil {
		out.CreatedBy = t.CreatedBy.String()
	}
	return out
}

// CreateGateway adds a pool.
func (h *GatewayRPC) CreateGateway(ctx context.Context, req *connect.Request[werbotv1.CreateGatewayRequest]) (*connect.Response[werbotv1.CreateGatewayResponse], error) {
	g, err := h.g.Create(ctx, req.Msg.GetName())
	if err != nil {
		return nil, apperr.ToConnect(err)
	}
	return connect.NewResponse(&werbotv1.CreateGatewayResponse{Gateway: gatewayToProto(g)}), nil
}

// ListGateways lists the pools with their instances.
func (h *GatewayRPC) ListGateways(ctx context.Context, _ *connect.Request[werbotv1.ListGatewaysRequest]) (*connect.Response[werbotv1.ListGatewaysResponse], error) {
	gs, err := h.g.List(ctx)
	if err != nil {
		return nil, apperr.ToConnect(err)
	}
	res := &werbotv1.ListGatewaysResponse{Gateways: make([]*werbotv1.Gateway, len(gs))}
	for i, g := range gs {
		res.Gateways[i] = gatewayToProto(g)
	}
	return connect.NewResponse(res), nil
}

// DeleteGateway removes a pool that serves no resources.
func (h *GatewayRPC) DeleteGateway(ctx context.Context, req *connect.Request[werbotv1.DeleteGatewayRequest]) (*connect.Response[werbotv1.DeleteGatewayResponse], error) {
	id, err := parseID(req.Msg.GetId())
	if err == nil {
		err = h.g.Delete(ctx, id)
	}
	if err != nil {
		return nil, apperr.ToConnect(err)
	}
	return connect.NewResponse(&werbotv1.DeleteGatewayResponse{}), nil
}

// CreateJoinToken issues a join token; unset expires_in and zero max_uses take the defaults.
func (h *GatewayRPC) CreateJoinToken(ctx context.Context, req *connect.Request[werbotv1.CreateJoinTokenRequest]) (*connect.Response[werbotv1.CreateJoinTokenResponse], error) {
	id, err := parseID(req.Msg.GetGatewayId())
	if err != nil {
		return nil, apperr.ToConnect(err)
	}
	tokenID, token, exp, err := h.g.CreateJoinToken(ctx, id, req.Msg.GetExpiresIn().AsDuration(), int(req.Msg.GetMaxUses()))
	if err != nil {
		return nil, apperr.ToConnect(err)
	}
	return connect.NewResponse(&werbotv1.CreateJoinTokenResponse{Id: tokenID.String(), Token: token, ExpiresAt: timestamppb.New(exp)}), nil
}

// ListJoinTokens lists the tokens of a pool.
func (h *GatewayRPC) ListJoinTokens(ctx context.Context, req *connect.Request[werbotv1.ListJoinTokensRequest]) (*connect.Response[werbotv1.ListJoinTokensResponse], error) {
	id, err := parseID(req.Msg.GetGatewayId())
	if err != nil {
		return nil, apperr.ToConnect(err)
	}
	ts, err := h.g.ListJoinTokens(ctx, id)
	if err != nil {
		return nil, apperr.ToConnect(err)
	}
	res := &werbotv1.ListJoinTokensResponse{Tokens: make([]*werbotv1.JoinToken, len(ts))}
	for i, t := range ts {
		res.Tokens[i] = joinTokenToProto(t)
	}
	return connect.NewResponse(res), nil
}

// RevokeJoinToken stops a token from enrolling.
func (h *GatewayRPC) RevokeJoinToken(ctx context.Context, req *connect.Request[werbotv1.RevokeJoinTokenRequest]) (*connect.Response[werbotv1.RevokeJoinTokenResponse], error) {
	id, err := parseID(req.Msg.GetId())
	if err == nil {
		err = h.g.RevokeJoinToken(ctx, id)
	}
	if err != nil {
		return nil, apperr.ToConnect(err)
	}
	return connect.NewResponse(&werbotv1.RevokeJoinTokenResponse{}), nil
}

// DeleteGatewayInstance removes one instance.
func (h *GatewayRPC) DeleteGatewayInstance(ctx context.Context, req *connect.Request[werbotv1.DeleteGatewayInstanceRequest]) (*connect.Response[werbotv1.DeleteGatewayInstanceResponse], error) {
	id, err := parseID(req.Msg.GetId())
	if err == nil {
		err = h.g.DeleteInstance(ctx, id)
	}
	if err != nil {
		return nil, apperr.ToConnect(err)
	}
	return connect.NewResponse(&werbotv1.DeleteGatewayInstanceResponse{}), nil
}

// Enroll registers a gateway instance; the join token is the only credential.
func (h *GatewayRPC) Enroll(ctx context.Context, req *connect.Request[werbotv1.EnrollRequest]) (*connect.Response[werbotv1.EnrollResponse], error) {
	m := req.Msg
	en, err := h.g.Enroll(ctx, EnrollRequest{JoinToken: m.GetJoinToken(), CSRDER: m.GetCsrDer(), Hostname: m.GetHostname(),
		Version: m.GetVersion(), Connectors: m.GetConnectors(), IP: clientip.From(ctx)})
	if err != nil {
		return nil, apperr.ToConnect(err)
	}
	return connect.NewResponse(&werbotv1.EnrollResponse{GatewayId: en.GatewayID.String(), InstanceId: en.InstanceID.String(),
		CertPem: string(en.CertPEM), CaPem: string(en.CAPEM), TunnelUrl: h.tunnelURL, HostKeyPem: string(en.HostKeyPEM)}), nil
}
