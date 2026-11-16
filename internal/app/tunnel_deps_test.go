package app

import (
	"context"
	"log/slog"
	"net/netip"
	"slices"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/wrapperspb"

	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
	"github.com/werbot/werbot/internal/platform/apperr"
	"github.com/werbot/werbot/internal/platform/metrics"
	"github.com/werbot/werbot/internal/tunnel"
)

// gatewayPool creates a gateway pool in the caller's organization and three echo
// resources with a user binding; the first pool takes every resource.
func (s stage4) gatewayPool(ctx context.Context, t *testing.T, user uuid.UUID) (gatewayID uuid.UUID, resourceIDs []string) {
	t.Helper()
	g, err := s.d.Gateways.Create(ctx, "edge")
	require.NoError(t, err)
	for range 3 {
		r, err := s.echo(ctx, t, wrapperspb.String("x"), map[string]string{"env": "prod"})
		require.NoError(t, err)
		resourceIDs = append(resourceIDs, r.GetId())
		s.bind(ctx, t, &werbotv1.CreateBindingRequest{SubjectType: "user", SubjectId: user.String(), ResourceId: r.GetId()})
	}
	return g.ID, resourceIDs
}

func TestSnapshotBuildDeterministic(t *testing.T) {
	s := newStage4(t, slog.New(slog.DiscardHandler))
	ctx, org, owner := s.org(t, "owner")
	gw, ids := s.gatewayPool(ctx, t, owner)
	require.NoError(t, s.d.Policy.ReplaceBlocklist(ctx, "test", []netip.Prefix{
		netip.MustParsePrefix("9.9.9.0/24"), netip.MustParsePrefix("45.0.0.0/24"), netip.MustParsePrefix("203.0.113.0/24")}))

	b := newTunnelDeps(s.d, metrics.NewRegistry(), slog.New(slog.DiscardHandler)).Snapshots
	first, err := b.Build(context.Background(), org, gw)
	require.NoError(t, err)
	second, err := b.Build(context.Background(), org, gw)
	require.NoError(t, err)
	assert.True(t, proto.Equal(first, second), "two builds of the same data differ")

	assert.Contains(t, first.GetCedarPolicies(), ids[0])
	assert.NotEmpty(t, first.GetEntitiesJson())
	require.Len(t, first.GetResources(), 3)
	names := make([]string, 0, 3)
	for _, r := range first.GetResources() {
		names = append(names, r.GetName())
		assert.Equal(t, "echo", r.GetType())
		assert.JSONEq(t, `"x"`, string(r.GetConfigJson()))
	}
	assert.True(t, slices.IsSorted(names), "resources by name: %v", names)
	assert.True(t, slices.IsSorted(first.GetBlocklist()), "blocklist sorted: %v", first.GetBlocklist())
	assert.Len(t, first.GetBlocklist(), 3)
}

// A policy that does not compile must not leave the gateway on an older,
// possibly more permissive snapshot: Build sends deny-all instead.
func TestSnapshotBuildDenyAllOnCompileError(t *testing.T) {
	s := newStage4(t, slog.New(slog.DiscardHandler))
	ctx, org, owner := s.org(t, "owner")
	gw, _ := s.gatewayPool(ctx, t, owner)
	_, err := s.pool.Exec(ctx, `ALTER TABLE access_binding DROP CONSTRAINT access_binding_subject_type_check`)
	require.NoError(t, err)
	_, err = s.pool.Exec(ctx, `UPDATE access_binding SET subject_type = 'robot' WHERE org_id = $1`, org)
	require.NoError(t, err)

	snap, err := newTunnelDeps(s.d, metrics.NewRegistry(), slog.New(slog.DiscardHandler)).Snapshots.Build(context.Background(), org, gw)
	require.NoError(t, err)
	assert.Empty(t, snap.GetCedarPolicies())
	assert.Empty(t, snap.GetEntitiesJson())
	assert.Len(t, snap.GetResources(), 3)
}

// The gateway names the session's user; only a member of the pool's
// organization may own a session.
func TestSessionStartRequiresMember(t *testing.T) {
	s := newStage4(t, slog.New(slog.DiscardHandler))
	ctx, org, owner := s.org(t, "owner")
	gw, ids := s.gatewayPool(ctx, t, owner)
	_, otherOrg, stranger := s.org(t, "owner")
	require.NotEqual(t, org, otherOrg)
	var instance uuid.UUID
	require.NoError(t, s.pool.QueryRow(ctx, `INSERT INTO gateway_instance (gateway_id, org_id, hostname, cert_serial, cert_expires_at)
		VALUES ($1, $2, 'h', 'ab', now() + interval '1 hour') RETURNING id`, gw, org).Scan(&instance))
	sessions := newTunnelDeps(s.d, metrics.NewRegistry(), slog.New(slog.DiscardHandler)).Sessions
	start := tunnel.SessionStart{OrgID: org, GatewayID: gw, InstanceID: instance, ResourceID: uuid.MustParse(ids[0]),
		ClientIP: netip.MustParseAddr("10.0.0.1")}

	for name, user := range map[string]uuid.UUID{"other org": stranger, "unknown": uuid.New()} {
		st := start
		st.UserID = user
		_, err := sessions.Start(context.Background(), st)
		require.ErrorIs(t, err, apperr.ErrNotFound, name)
	}
	var n int
	require.NoError(t, s.pool.QueryRow(ctx, `SELECT count(*) FROM session`).Scan(&n))
	assert.Zero(t, n, "no session row for a non-member")

	start.UserID = owner
	id, err := sessions.Start(context.Background(), start)
	require.NoError(t, err)
	var user uuid.UUID
	require.NoError(t, s.pool.QueryRow(ctx, `SELECT user_id FROM session WHERE id = $1`, id).Scan(&user))
	assert.Equal(t, owner, user)
}

func TestTunnelURL(t *testing.T) {
	for _, c := range []struct{ public, addr, want string }{
		{"https://w.example.com", ":8443", "https://w.example.com:8443"},
		{"http://127.0.0.1:8080", "127.0.0.1:9443", "https://127.0.0.1:9443"},
		{"https://[::1]:8080", ":8443", "https://[::1]:8443"},
	} {
		got, err := tunnelURL(c.public, c.addr)
		require.NoError(t, err, c.public)
		assert.Equal(t, c.want, got)
	}
	_, err := tunnelURL("https://w.example.com", "8443")
	assert.ErrorContains(t, err, "WERBOT_GATEWAY_ADDR")
}
