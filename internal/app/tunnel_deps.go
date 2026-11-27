package app

import (
	"context"
	"log/slog"
	"slices"
	"strings"

	"github.com/google/uuid"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/durationpb"

	gatewayv1 "github.com/werbot/werbot/gen/gateway/v1"
	"github.com/werbot/werbot/internal/agentic"
	"github.com/werbot/werbot/internal/identity"
	"github.com/werbot/werbot/internal/inventory"
	"github.com/werbot/werbot/internal/platform/metrics"
	"github.com/werbot/werbot/internal/platform/sshca"
	"github.com/werbot/werbot/internal/policy"
	"github.com/werbot/werbot/internal/session"
	"github.com/werbot/werbot/internal/tunnel"
)

// newTunnelDeps adapts the domain services to the tunnel handler.
func newTunnelDeps(s *services, reg *metrics.Registry, logger *slog.Logger) tunnel.Deps {
	return tunnel.Deps{
		Gateways:    tunnelGateways{s.Gateways},
		Users:       tunnelUsers{s.Identity, s.Agents, s.Sessions},
		Resources:   tunnelResources{s.Inventory},
		Credentials: tunnelCredentials{s.Credential, s.Inventory, s.Sessions, s.Access},
		Snapshots:   tunnelSnapshots{s: s, logger: logger},
		Sessions:    tunnelSessions{s.Sessions, s.Agents},
		Recordings:  s.Recordings,
		Health:      tunnelResources{s.Inventory},
		Pools:       tunnelGateways{s.Gateways},
		Approvals:   tunnelApprovals{s.Access},
		Agents:      tunnelAgents{s.Agents},
		Relay:       bridger{s.Relay},
		MFA:         tunnelMFA{s.Identity},
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
	return tunnel.Gateway{ID: in.GatewayID, InstanceID: in.ID, OrgID: in.OrgID, CertSerial: in.CertSerial}, nil
}

// tunnelUsers resolves SSH clients to members with their teams, and agent
// certificates to their sponsor with the run.
type tunnelUsers struct {
	ids      *identity.Service
	agents   *agentic.Service
	sessions *session.Service
}

func (u tunnelUsers) ByFingerprint(ctx context.Context, orgID uuid.UUID, fingerprint string) (tunnel.User, error) {
	return tunnelUser(u.ids.LookupSSHKey(ctx, orgID, fingerprint))
}

func (u tunnelUsers) ByLogin(ctx context.Context, orgID uuid.UUID, login string) (tunnel.User, error) {
	return tunnelUser(u.ids.LookupLogin(ctx, orgID, login))
}

func (u tunnelUsers) ByCert(ctx context.Context, orgID uuid.UUID, id sshca.Identity) (tunnel.User, error) {
	usr, err := tunnelUser(u.ids.AuthenticateCert(ctx, orgID, id))
	if err != nil || id.RunID == nil {
		return usr, err
	}
	usr.Agent, err = agentOf(ctx, u.agents, orgID, usr.ID, id)
	return usr, err
}

func tunnelUser(ref identity.UserRef, err error) (tunnel.User, error) {
	if err != nil {
		return tunnel.User{}, err
	}
	return tunnel.User{ID: ref.UserID, Login: ref.Login, TeamIDs: ref.TeamIDs}, nil
}

// tunnelResources adds tunnel-typed health reports to inventory.Service.
type tunnelResources struct{ *inventory.Service }

func (r tunnelResources) ReportHealth(ctx context.Context, orgID, instanceID uuid.UUID, hs []tunnel.ResourceHealth) error {
	out := make([]inventory.Health, len(hs))
	for i, h := range hs {
		out[i] = inventory.Health(h)
	}
	return r.Service.ReportHealth(ctx, orgID, instanceID, out)
}

// tunnelSessions adapts session.Service to the tunnel types; Start records
// sessions only for members of the organization, an agent's only when its run
// counts the action.
type tunnelSessions struct {
	*session.Service
	agents *agentic.Service
}

func (s tunnelSessions) Start(ctx context.Context, st tunnel.SessionStart) (uuid.UUID, error) {
	id, err := s.Service.Start(ctx, session.Start{OrgID: st.OrgID, UserID: st.UserID, ResourceID: st.ResourceID,
		GatewayID: st.GatewayID, InstanceID: st.InstanceID, ClientIP: st.ClientIP, Kind: st.Kind, Front: st.Front, Transport: st.Transport, Target: st.Target,
		Command: st.Command, Classes: st.Classes, Paths: st.Paths, AgentRunID: st.AgentRunID, ToolCallID: st.ToolCallID,
		DBName: st.DBName, Guard: actionGuard(s.agents, st)})
	if st.AgentRunID == nil {
		return id, err
	}
	return id, actionError(err)
}

func (s tunnelSessions) End(ctx context.Context, orgID, instanceID, id uuid.UUID, reason string, c *tunnel.Counters, r *tunnel.SessionResult) error {
	return s.Service.End(ctx, orgID, instanceID, id, reason, (*session.Counters)(c), (*session.Result)(r))
}

func (s tunnelSessions) UpdateCounters(ctx context.Context, orgID, instanceID uuid.UUID, cs []tunnel.Counters) error {
	out := make([]session.Counters, len(cs))
	for i, c := range cs {
		out[i] = session.Counters(c)
	}
	return s.Service.UpdateCounters(ctx, orgID, instanceID, out)
}

func (s tunnelSessions) Deny(ctx context.Context, orgID uuid.UUID, ds []tunnel.Denial) error {
	out := make([]session.Denial, len(ds))
	for i, d := range ds {
		out[i] = session.Denial(d)
	}
	return s.Service.Deny(ctx, orgID, out)
}

// tunnelSnapshots builds a pool's snapshot: the organization's compiled
// policy with its expiries and lock mode, its members with their teams, the blocklist and the user CA keys (Org), then the
// pool's resources by name (Pool), sorted, so equal data gives an equal version.
type tunnelSnapshots struct {
	s      *services
	logger *slog.Logger
}

func (t tunnelSnapshots) Org(ctx context.Context, orgID uuid.UUID) (*gatewayv1.Snapshot, error) {
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
		out.PathRules = pathRules(snap) // only a compiled snapshot: deny-all carries none
	}
	// Also in the deny-all snapshot: expiries, class rules, the lock and mask
	// modes and a membership change must reach the gateway.
	out.Expirations = snap.Expirations()
	out.ClassRules = classRules(snap.ClassRules)
	set, err := t.s.Identity.Settings(ctx, orgID)
	if err != nil {
		return nil, err
	}
	out.LockMode, out.MaskOutput = set.LockMode, set.MaskOutput
	members, err := t.s.Identity.OrgMembers(ctx, orgID)
	if err != nil {
		return nil, err
	}
	for _, m := range members { // sorted by user id, teams sorted
		pm := &gatewayv1.Member{UserId: m.UserID.String(), TeamIds: make([]string, len(m.TeamIDs))}
		for i, id := range m.TeamIDs {
			pm.TeamIds[i] = id.String()
		}
		out.Members = append(out.Members, pm)
	}
	bl, err := t.s.Policy.Blocklist(ctx)
	if err != nil {
		return nil, err
	}
	for _, p := range bl {
		out.Blocklist = append(out.Blocklist, p.String())
	}
	slices.Sort(out.Blocklist)
	// Creates the CA on the first build, so the first snapshot already trusts it.
	if out.UserCaKeys, err = t.s.Identity.UserCAPublicKeys(ctx, orgID); err != nil {
		return nil, err
	}
	return out, nil
}

// classRules are rs by pattern in byte order, not the database collation, so
// every server replica gives the same version.
func classRules(rs []policy.SnapshotClassRule) []*gatewayv1.ClassRule {
	out := make([]*gatewayv1.ClassRule, len(rs))
	for i, r := range rs {
		out[i] = &gatewayv1.ClassRule{Pattern: r.Pattern, Classes: r.Classes}
	}
	slices.SortFunc(out, func(a, b *gatewayv1.ClassRule) int { return strings.Compare(a.GetPattern(), b.GetPattern()) })
	return out
}

// pathRules are the path rules of snap for EvalPaths, by id in byte order.
func pathRules(snap policy.Snapshot) []*gatewayv1.PathRule {
	rs := snap.PathEvalRules()
	out := make([]*gatewayv1.PathRule, len(rs))
	for i, r := range rs {
		out[i] = &gatewayv1.PathRule{Id: r.ID, Paths: r.Globs, Access: string(r.Access), Effect: r.Effect}
	}
	return out
}

func (t tunnelSnapshots) Pool(ctx context.Context, org *gatewayv1.Snapshot, orgID, gatewayID uuid.UUID) (*gatewayv1.Snapshot, error) {
	rs, err := t.s.Inventory.ResourcesForGateway(ctx, orgID, gatewayID)
	if err != nil {
		return nil, err
	}
	out := proto.CloneOf(org)
	for _, r := range rs {
		if r.Config == nil { // type no longer registered: no gateway can serve it
			continue
		}
		cfg, err := protojson.Marshal(r.Config)
		if err != nil {
			return nil, err
		}
		res := &gatewayv1.Resource{Id: r.ID.String(), Name: r.Name, Type: r.Type, ConfigJson: cfg,
			AgentReadonlyStatus: r.AgentReadonlyStatus, ConfigVersion: r.UpdatedAt.UnixMicro()}
		if r.IdleTimeout != nil {
			res.IdleTimeout = durationpb.New(*r.IdleTimeout)
		}
		if r.MaxDuration != nil {
			res.MaxDuration = durationpb.New(*r.MaxDuration)
		}
		if r.HostKey != nil {
			res.HostKeySha256 = r.HostKey.FingerprintSHA256
		}
		out.Resources = append(out.Resources, res)
	}
	return out, nil
}
