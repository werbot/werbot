package app

import (
	"context"
	"log/slog"
	"slices"

	"github.com/google/uuid"
	"google.golang.org/protobuf/encoding/protojson"

	gatewayv1 "github.com/werbot/werbot/gen/gateway/v1"
	"github.com/werbot/werbot/internal/credential"
	"github.com/werbot/werbot/internal/identity"
	"github.com/werbot/werbot/internal/inventory"
	"github.com/werbot/werbot/internal/platform/metrics"
	"github.com/werbot/werbot/internal/policy"
	"github.com/werbot/werbot/internal/session"
	"github.com/werbot/werbot/internal/tunnel"
)

// newTunnelDeps adapts the domain services to the tunnel handler.
func newTunnelDeps(s *services, reg *metrics.Registry, logger *slog.Logger) tunnel.Deps {
	return tunnel.Deps{
		Gateways:    tunnelGateways{s.Gateways},
		Users:       tunnelUsers{s.Identity},
		Resources:   tunnelResources{s.Inventory},
		Credentials: tunnelCredentials{s.Credential},
		Snapshots:   tunnelSnapshots{s: s, logger: logger},
		Sessions:    tunnelSessions{Service: s.Sessions, ids: s.Identity},
		Country:     s.Policy.Country,
		Metrics:     reg,
		Logger:      logger,
	}
}

// tunnelGateways is inventory.Gateways with Authenticate returning the tunnel identity.
type tunnelGateways struct{ *inventory.Gateways }

func (g tunnelGateways) Authenticate(ctx context.Context, gatewayID, instanceID uuid.UUID, certSerial string) (tunnel.Gateway, error) {
	in, err := g.Gateways.Authenticate(ctx, gatewayID, instanceID, certSerial)
	if err != nil {
		return tunnel.Gateway{}, err
	}
	return tunnel.Gateway{ID: in.GatewayID, InstanceID: in.ID, OrgID: in.OrgID}, nil
}

// tunnelUsers resolves SSH clients to members with their teams.
type tunnelUsers struct{ ids *identity.Service }

func (u tunnelUsers) ByFingerprint(ctx context.Context, orgID uuid.UUID, fingerprint string) (tunnel.User, error) {
	ref, err := u.ids.LookupSSHKey(ctx, orgID, fingerprint)
	return u.user(ctx, orgID, ref, err)
}

func (u tunnelUsers) ByLogin(ctx context.Context, orgID uuid.UUID, login string) (tunnel.User, error) {
	ref, err := u.ids.LookupLogin(ctx, orgID, login)
	return u.user(ctx, orgID, ref, err)
}

func (u tunnelUsers) user(ctx context.Context, orgID uuid.UUID, ref identity.UserRef, err error) (tunnel.User, error) {
	if err != nil {
		return tunnel.User{}, err
	}
	teams, err := u.ids.UserTeams(ctx, orgID, ref.UserID)
	if err != nil {
		return tunnel.User{}, err
	}
	return tunnel.User{ID: ref.UserID, Login: ref.Login, TeamIDs: teams}, nil
}

// tunnelResources adds the pool ownership check to inventory.Service.
type tunnelResources struct{ *inventory.Service }

// OwnedBy reports whether resourceID is served by the pool gatewayID.
// ponytail: scans the pool's resources per call (O(n)); add a by-id query if pools grow large.
func (r tunnelResources) OwnedBy(ctx context.Context, orgID, gatewayID, resourceID uuid.UUID) (bool, error) {
	rs, err := r.ResourcesForGateway(ctx, orgID, gatewayID)
	if err != nil {
		return false, err
	}
	return slices.ContainsFunc(rs, func(res inventory.Resource) bool { return res.ID == resourceID }), nil
}

type tunnelCredentials struct{ c *credential.Service }

func (c tunnelCredentials) Reveal(ctx context.Context, orgID, resourceID uuid.UUID) (tunnel.Secret, error) {
	sec, err := c.c.Reveal(ctx, orgID, resourceID)
	if err != nil {
		return tunnel.Secret{}, err
	}
	return tunnel.Secret{Kind: string(sec.Kind), Value: sec.Value}, nil
}

// tunnelSessions records sessions only for members of the organization: the
// user id comes from the gateway, which the server does not trust with it.
type tunnelSessions struct {
	*session.Service
	ids *identity.Service
}

func (s tunnelSessions) Start(ctx context.Context, st tunnel.SessionStart) (uuid.UUID, error) {
	if _, err := s.ids.LookupUser(ctx, st.OrgID, st.UserID); err != nil {
		return uuid.Nil, err
	}
	return s.Service.Start(ctx, session.Start{OrgID: st.OrgID, UserID: st.UserID, ResourceID: st.ResourceID,
		GatewayID: st.GatewayID, InstanceID: st.InstanceID, ClientIP: st.ClientIP})
}

// tunnelSnapshots builds a pool's snapshot: the organization's compiled
// policy, the pool's resources by name and the blocklist, sorted, so equal
// data gives an equal version.
type tunnelSnapshots struct {
	s      *services
	logger *slog.Logger
}

func (t tunnelSnapshots) Build(ctx context.Context, orgID, gatewayID uuid.UUID) (*gatewayv1.Snapshot, error) {
	snap, err := t.s.Policy.Snapshot(ctx, orgID)
	if err != nil {
		return nil, err
	}
	out := &gatewayv1.Snapshot{}
	// A policy that does not compile denies everything (empty cedar_policies),
	// as CheckAccess does; a stale snapshot could keep revoked access.
	if ps, text, err := policy.Compile(snap); err != nil {
		t.logger.ErrorContext(ctx, "tunnel: policy does not compile, sending deny-all", "org_id", orgID, "error", err)
	} else {
		if out.EntitiesJson, err = ps.EntitiesJSON(); err != nil {
			return nil, err
		}
		out.CedarPolicies = text
	}
	rs, err := t.s.Inventory.ResourcesForGateway(ctx, orgID, gatewayID)
	if err != nil {
		return nil, err
	}
	for _, r := range rs {
		if r.Config == nil { // type no longer registered: no gateway can serve it
			continue
		}
		cfg, err := protojson.Marshal(r.Config)
		if err != nil {
			return nil, err
		}
		out.Resources = append(out.Resources, &gatewayv1.Resource{Id: r.ID.String(), Name: r.Name, Type: r.Type, ConfigJson: cfg})
	}
	bl, err := t.s.Policy.Blocklist(ctx)
	if err != nil {
		return nil, err
	}
	for _, p := range bl {
		out.Blocklist = append(out.Blocklist, p.String())
	}
	slices.Sort(out.Blocklist)
	return out, nil
}
