package app

import (
	"context"
	"log/slog"
	"testing"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/wrapperspb"

	gatewayv1 "github.com/werbot/werbot/gen/gateway/v1"
	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
	"github.com/werbot/werbot/internal/platform/metrics"
	"github.com/werbot/werbot/internal/tunnel"
)

// buildSnapshot builds the snapshot of pool gw as the tunnel does: the
// organization part, then the pool's.
func buildSnapshot(ctx context.Context, b tunnel.Snapshots, org, gw uuid.UUID) (*gatewayv1.Snapshot, error) {
	base, err := b.Org(ctx, org)
	if err != nil {
		return nil, err
	}
	return b.Pool(ctx, base, org, gw)
}

func marshalSnapshot(t *testing.T, s *gatewayv1.Snapshot) []byte {
	t.Helper()
	b, err := proto.MarshalOptions{Deterministic: true}.Marshal(s)
	require.NoError(t, err)
	return b
}

// One organization part completed for two pools gives each pool the same
// bytes as a build of that pool alone, and the shared part stays unchanged.
func TestSnapshotOrgPartShared(t *testing.T) {
	s := newStage4(t, slog.New(slog.DiscardHandler))
	ctx, org, owner := s.org(t, "owner")
	gwA, _ := s.gatewayPool(ctx, t, owner)
	gwB, err := s.d.Gateways.Create(ctx, "edge-b")
	require.NoError(t, err)
	a, err := anypb.New(wrapperspb.String("b"))
	require.NoError(t, err)
	res, err := s.res.CreateResource(ctx, connect.NewRequest(&werbotv1.CreateResourceRequest{
		Name: "rb", Type: "echo", GatewayId: gwB.ID.String(), Config: a}))
	require.NoError(t, err)
	s.bind(ctx, t, &werbotv1.CreateBindingRequest{SubjectType: "user", SubjectId: owner.String(), ResourceId: res.Msg.GetResource().GetId()})

	b := newTunnelDeps(s.d, metrics.NewRegistry(), slog.New(slog.DiscardHandler)).Snapshots
	base, err := b.Org(ctx, org)
	require.NoError(t, err)
	before := marshalSnapshot(t, base)
	shared := map[uuid.UUID]*gatewayv1.Snapshot{}
	for _, gw := range []uuid.UUID{gwA, gwB.ID} {
		shared[gw], err = b.Pool(ctx, base, org, gw)
		require.NoError(t, err)
	}
	assert.Equal(t, before, marshalSnapshot(t, base), "Pool changed the shared organization part")
	assert.Len(t, shared[gwA].GetResources(), 3)
	assert.Len(t, shared[gwB.ID].GetResources(), 1)
	for gw, got := range shared {
		alone, err := buildSnapshot(ctx, b, org, gw)
		require.NoError(t, err)
		assert.Equal(t, marshalSnapshot(t, alone), marshalSnapshot(t, got), "pool %s", gw)
	}
}
