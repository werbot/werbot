package app

import (
	"context"
	"log/slog"
	"math/rand/v2"
	"net/netip"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/emptypb"

	connectorv1 "github.com/werbot/werbot/gen/werbot/connector/v1"
	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
	"github.com/werbot/werbot/internal/connector"
	"github.com/werbot/werbot/internal/gateway"
	"github.com/werbot/werbot/internal/platform/db/dbtest"
	"github.com/werbot/werbot/internal/platform/metrics"
	"github.com/werbot/werbot/internal/policy"
	"github.com/werbot/werbot/internal/tunnel"
)

// noconnect is a resource type without a "<type>.connect" action: neither
// side may let a gateway session reach it.
func init() {
	connector.Register(connector.Type{Name: "noconnect", Category: connector.CategoryApplications, Level: "L1",
		NewConfig:   func() proto.Message { return &emptypb.Empty{} },
		Validate:    func(proto.Message) error { return nil },
		Credentials: []connector.CredentialKind{connector.CredentialPassword}, Actions: []string{"noconnect.exec"},
		Kinds:    []connector.Kind{connector.KindStream},
		Endpoint: func(proto.Message) connector.Endpoint { return connector.Endpoint{AnyTarget: true, NoHealth: true} }})
}

// TestPolicyParity decides the same requests on the server (policy.PolicySet)
// and on the gateway (CompilePolicy of the snapshot the tunnel sends) from
// one database: both decisions must be equal and as expected.
func TestPolicyParity(t *testing.T) {
	s := newStage4(t, slog.New(slog.DiscardHandler))
	ctx, org, user := s.org(t, "owner")
	p := parity{s: s, org: org, user: user, gw: dbtest.Gateway(t, s.pool, org, "gw"),
		snaps: newTunnelDeps(s.d, metrics.NewRegistry(), slog.New(slog.DiscardHandler)).Snapshots}
	team := dbtest.Team(t, s.pool, org, user)
	for _, r := range []struct{ name, typ string }{{"srv", "ssh"}, {"odd", "noconnect"}} {
		id := dbtest.Resource(t, s.pool, org, r.name, nil)
		_, err := s.pool.Exec(ctx, "UPDATE resource SET gateway_id = $2, type = $3 WHERE id = $1", id, p.gw, r.typ)
		require.NoError(t, err)
		s.bind(ctx, t, &werbotv1.CreateBindingRequest{SubjectType: "team", SubjectId: team.String(), ResourceId: id.String()})
		if r.name == "srv" {
			p.srv = id
		}
	}

	v4, v6 := netip.MustParseAddr("10.1.2.3"), netip.MustParseAddr("::ffff:10.1.2.3")
	cases := []struct {
		name     string
		rules    policy.ResourceRules // of "srv"
		blocked  string               // a blocklist CIDR
		resource string
		ip       netip.Addr
		country  string
		time     time.Time // zero: monday10
		allow    bool
	}{
		{name: "allow", resource: "srv", ip: v4, allow: true},
		{name: "allow mapped", resource: "srv", ip: v6, allow: true},
		{name: "cidr deny", rules: networks("10.0.0.0/8", policy.EffectDeny), resource: "srv", ip: v4},
		{name: "cidr deny mapped", rules: networks("10.0.0.0/8", policy.EffectDeny), resource: "srv", ip: v6},
		{name: "cidr allow mapped", rules: networks("10.0.0.0/8", policy.EffectAllow), resource: "srv", ip: v6, allow: true},
		{name: "cidr allow other", rules: networks("192.168.0.0/16", policy.EffectAllow), resource: "srv", ip: v6},
		{name: "country NL", rules: nlOnly, resource: "srv", ip: v4, country: "NL", allow: true},
		{name: "country US", rules: nlOnly, resource: "srv", ip: v4, country: "US"},
		{name: "country unknown", rules: nlOnly, resource: "srv", ip: v4},
		{name: "schedule inside", rules: monday10Only, resource: "srv", ip: v4, allow: true},
		{name: "schedule inside other zone", rules: monday10Only, resource: "srv", ip: v4,
			time: monday10.In(time.FixedZone("UTC+5", 5*3600)), allow: true},
		{name: "schedule outside", rules: monday10Only, resource: "srv", ip: v4, time: monday10.Add(time.Hour)},
		{name: "blocklist", blocked: "10.1.0.0/16", resource: "srv", ip: v4},
		{name: "blocklist mapped", blocked: "10.1.0.0/16", resource: "srv", ip: v6},
		{name: "blocklist other", blocked: "10.2.0.0/16", resource: "srv", ip: v4, allow: true},
		{name: "invalid ip", resource: "srv"},
		{name: "type without connect action", resource: "odd", ip: v4},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rules := tc.rules
			rules.ResourceID = p.srv
			require.NoError(t, s.d.Policy.SetRules(ctx, org, rules))
			_, err := s.pool.Exec(ctx, "DELETE FROM ip_blocklist")
			require.NoError(t, err)
			if tc.blocked != "" {
				_, err = s.pool.Exec(ctx, "INSERT INTO ip_blocklist (cidr, source) VALUES ($1, 'test')", tc.blocked)
				require.NoError(t, err)
			}
			when := tc.time
			if when.IsZero() {
				when = monday10
			}
			server, gw := p.decide(ctx, t, tc.resource, tc.ip, tc.country, when, 0, uuid.Nil)
			assert.Equal(t, tc.allow, server, "server")
			assert.Equal(t, tc.allow, gw, "gateway")
		})
	}

	// A binding with ports on a second ssh resource: 200 requests (the range
	// edges, then random ports, half of them near the ranges) decide the same
	// on both sides and as the ranges say; port 0 (terminal, exec) passes.
	t.Run("ports", func(t *testing.T) {
		require.NoError(t, s.d.Policy.SetRules(ctx, org, policy.ResourceRules{ResourceID: p.srv}))
		_, err := s.pool.Exec(ctx, "DELETE FROM ip_blocklist")
		require.NoError(t, err)
		id := dbtest.Resource(t, s.pool, org, "pg", nil)
		_, err = s.pool.Exec(ctx, "UPDATE resource SET gateway_id = $2, type = 'ssh' WHERE id = $1", id, p.gw)
		require.NoError(t, err)
		s.bind(ctx, t, &werbotv1.CreateBindingRequest{SubjectType: "user", SubjectId: user.String(), ResourceId: id.String(),
			Ports: []*connectorv1.PortRange{{Port: 8000, EndPort: 8100}, {Port: 5432}}})
		rng := rand.New(rand.NewPCG(6, 2)) //nolint:gosec // reproducible test ports
		edges := []uint16{0, 1, 5431, 5432, 5433, 7999, 8000, 8100, 8101, 65535}
		for i := range 200 {
			port := uint16(rng.Uint32()) //nolint:gosec // any port
			switch {
			case i < len(edges):
				port = edges[i]
			case i%2 == 0:
				port = 5400 + uint16(rng.Uint32()%2800) // 5400–8199
			}
			want := port == 0 || port == 5432 || (port >= 8000 && port <= 8100)
			server, gw := p.decide(ctx, t, "pg", v4, "", monday10, port, uuid.Nil)
			require.Equal(t, want, server, "server, port %d", port)
			require.Equal(t, want, gw, "gateway, port %d", port)
		}
	})

	// A stored rule that does not compile denies on both sides: the server
	// (CheckAccess too) and the deny-all snapshot of the tunnel. Compiled,
	// the rule (deny "nl") would let a client from US in.
	t.Run("broken policy", func(t *testing.T) {
		require.NoError(t, s.d.Policy.SetRules(ctx, org, policy.ResourceRules{ResourceID: p.srv}))
		_, err := s.pool.Exec(ctx, "ALTER TABLE resource_country_rule DROP CONSTRAINT resource_country_rule_country_check")
		require.NoError(t, err)
		_, err = s.pool.Exec(ctx, "INSERT INTO resource_country_rule (org_id, resource_id, country, effect) VALUES ($1, $2, 'nl', 'deny')",
			org, p.srv)
		require.NoError(t, err)
		d, err := s.d.Policy.CheckAccess(ctx, org, user, p.srv, v4, monday10, 0)
		require.NoError(t, err)
		assert.Equal(t, []string{"error:compile"}, d.Reasons)
		server, gw := p.decide(ctx, t, "srv", v4, "US", monday10, 0, uuid.Nil)
		assert.False(t, server, "server")
		assert.False(t, gw, "gateway")
		// a snapshot the gateway cannot compile leaves it without a policy (not ready): deny
		allow, reasons := (*gateway.Policy)(nil).Authorize(gateway.AccessRequest{IP: v4})
		assert.False(t, allow)
		assert.Equal(t, []string{"error:compile"}, reasons)
	})
}

var (
	monday10     = time.Date(2026, 11, 16, 10, 30, 0, 0, time.UTC)
	nlOnly       = policy.ResourceRules{Countries: []policy.CountryRule{{Country: "NL", Effect: policy.EffectAllow}}}
	monday10Only = policy.ResourceRules{Schedule: []policy.Slot{{Weekday: 1, Hour: 10}}}
)

func networks(cidr string, e policy.Effect) policy.ResourceRules {
	return policy.ResourceRules{Networks: []policy.NetworkRule{{CIDR: netip.MustParsePrefix(cidr), Effect: e}}}
}

// parity is one organization with the pool gw on the services of stage4.
type parity struct {
	s             stage4
	snaps         tunnel.Snapshots
	org, user, gw uuid.UUID
	srv           uuid.UUID // the ssh resource
}

// decide asks the server PolicySet (with the action "<type>.connect", as the
// gateway asks) and the gateway Policy of the pool snapshot whether user may
// connect to the resource name on port from the CLI session cli. For an ssh resource,
// no country and no CLI session it also checks that CheckAccess agrees with the server.
func (p parity) decide(ctx context.Context, t *testing.T, name string, ip netip.Addr, country string, when time.Time, port uint16, cli uuid.UUID) (server, gw bool) {
	t.Helper()
	teams, err := p.s.d.Identity.UserTeams(ctx, p.org, p.user)
	require.NoError(t, err)

	snap, err := p.s.d.Policy.Snapshot(ctx, p.org)
	require.NoError(t, err)
	i := slices.IndexFunc(snap.Resources, func(r policy.SnapshotResource) bool { return r.Name == name })
	require.GreaterOrEqual(t, i, 0)
	res := snap.Resources[i]
	ps, _, _ := policy.Compile(snap) // nil on error: Authorize denies, as CheckAccess does
	bl, err := p.s.d.Policy.Blocklist(ctx)
	require.NoError(t, err)
	blocked := ip.IsValid() && slices.ContainsFunc(bl, func(b netip.Prefix) bool { return b.Contains(ip.Unmap()) })
	server = ps.Authorize(policy.Request{UserID: p.user, TeamIDs: teams, ResourceID: res.ID, Action: res.Type + ".connect",
		IP: ip, Country: country, Time: when, IPBlocked: blocked, Port: port, CLISessionID: cli}).Allow
	if country == "" && cli == uuid.Nil && res.Type == "ssh" {
		d, err := p.s.d.Policy.CheckAccess(ctx, p.org, p.user, res.ID, ip, when, port)
		require.NoError(t, err)
		require.Equal(t, server, d.Allow, "CheckAccess")
	}

	gsnap, err := buildSnapshot(ctx, p.snaps, p.org, p.gw)
	require.NoError(t, err)
	pol, err := gateway.CompilePolicy(gsnap)
	require.NoError(t, err)
	r, ok := pol.Resource(name)
	require.True(t, ok, "resource in the snapshot")
	gw, _ = pol.Authorize(gateway.AccessRequest{User: gateway.User{ID: p.user, TeamIDs: teams, Country: country},
		Resource: r, IP: ip, Time: when, Port: port, CLISessionID: cli})
	return server, gw
}
