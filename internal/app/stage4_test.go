package app

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"net/netip"
	"slices"
	"testing"
	"testing/fstest"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/wrapperspb"

	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
	"github.com/werbot/werbot/gen/werbot/v1/werbotv1connect"
	"github.com/werbot/werbot/internal/access"
	"github.com/werbot/werbot/internal/inventory"
	"github.com/werbot/werbot/internal/platform/config"
	"github.com/werbot/werbot/internal/platform/db/dbtest"
	"github.com/werbot/werbot/internal/platform/principal"
	"github.com/werbot/werbot/internal/platform/secretbox"
	"github.com/werbot/werbot/internal/policy"
)

type noTeams struct{}

func (noTeams) UserTeams(context.Context, uuid.UUID, uuid.UUID) ([]uuid.UUID, error) { return nil, nil }

// stage4 is newDomains on a test database with its three Connect handlers.
type stage4 struct {
	d    domains
	pool *pgxpool.Pool
	res  *inventory.Handler
	acc  *access.Handler
	pol  *policy.Handler
}

func newStage4(t *testing.T, logger *slog.Logger) stage4 {
	t.Helper()
	pool, _ := dbtest.New(t)
	return stage4Of(t, pool, logger)
}

func stage4Of(t *testing.T, pool *pgxpool.Pool, logger *slog.Logger) stage4 {
	t.Helper()
	box, err := secretbox.New(bytes.Repeat([]byte{1}, 32), nil)
	require.NoError(t, err)
	d := newDomains(context.Background(), pool, box, noTeams{}, logger)
	return stage4{d: d, pool: pool, res: inventory.NewHandler(d.inv, d.cred), acc: access.NewHandler(d.acc), pol: policy.NewHandler(d.pol)}
}

// org creates an organization with one user of role and returns the user's ctx.
func (s stage4) org(t *testing.T, role string) (ctx context.Context, org, user uuid.UUID) {
	t.Helper()
	org = dbtest.Org(t, s.pool)
	user = dbtest.User(t, s.pool, org, role)
	return principal.With(context.Background(), principal.Principal{UserID: user, OrgID: org, Role: role}), org, user
}

// echo creates a resource of the test type "echo".
func (s stage4) echo(ctx context.Context, t *testing.T, cfg *wrapperspb.StringValue, labels map[string]string) (*werbotv1.Resource, error) {
	t.Helper()
	a, err := anypb.New(cfg)
	require.NoError(t, err)
	res, err := s.res.CreateResource(ctx, connect.NewRequest(&werbotv1.CreateResourceRequest{
		Name: "r" + uuid.NewString()[:8], Type: "echo", Config: a, Labels: labels}))
	if err != nil {
		return nil, err
	}
	return res.Msg.GetResource(), nil
}

func (s stage4) bind(ctx context.Context, t *testing.T, req *werbotv1.CreateBindingRequest) string {
	t.Helper()
	res, err := s.acc.CreateBinding(ctx, connect.NewRequest(req))
	require.NoError(t, err)
	return res.Msg.GetBinding().GetId()
}

func (s stage4) allowed(ctx context.Context, t *testing.T, user uuid.UUID, resourceID string) bool {
	t.Helper()
	res, err := s.pol.CheckAccess(ctx, connect.NewRequest(&werbotv1.CheckAccessRequest{
		UserId: user.String(), ResourceId: resourceID, Ip: "10.0.0.1"}))
	require.NoError(t, err)
	return res.Msg.GetAllow()
}

func TestEchoTypeNeedsNoMigrationOrDomainChanges(t *testing.T) {
	s := newStage4(t, slog.New(slog.DiscardHandler))
	ctx, org, owner := s.org(t, "owner")

	types, err := s.res.ListResourceTypes(ctx, connect.NewRequest(&werbotv1.ListResourceTypesRequest{}))
	require.NoError(t, err)
	i := slices.IndexFunc(types.Msg.GetTypes(), func(rt *werbotv1.ResourceType) bool { return rt.GetName() == "echo" })
	require.GreaterOrEqual(t, i, 0, "echo listed")
	echo := types.Msg.GetTypes()[i]
	assert.Equal(t, []string{"echo.connect"}, echo.GetActions())
	assert.Equal(t, []string{"stream"}, echo.GetKinds())
	assert.Equal(t, "applications", echo.GetCategory())
	assert.Equal(t, "L1", echo.GetLevel())

	r, err := s.echo(ctx, t, wrapperspb.String("hello"), nil)
	require.NoError(t, err)
	got, err := s.res.GetResource(ctx, connect.NewRequest(&werbotv1.GetResourceRequest{Id: r.GetId()}))
	require.NoError(t, err)
	var cfg wrapperspb.StringValue
	require.NoError(t, got.Msg.GetResource().GetConfig().UnmarshalTo(&cfg))
	assert.Equal(t, "hello", cfg.GetValue())
	assert.Equal(t, "applications", got.Msg.GetResource().GetCategory())

	_, err = s.echo(ctx, t, wrapperspb.String(""), nil)
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))

	assert.Contains(t, policy.Schema(), `action "echo.connect"`)

	bid := s.bind(ctx, t, &werbotv1.CreateBindingRequest{SubjectType: "user", SubjectId: owner.String(), ResourceId: r.GetId()})
	snap, err := s.d.pol.Snapshot(ctx, org)
	require.NoError(t, err)
	ps, text, err := policy.Compile(snap)
	require.NoError(t, err)
	assert.Contains(t, text, `Werbot::Resource::"`+r.GetId()+`"`)
	dec := ps.Authorize(policy.Request{UserID: owner, ResourceID: uuid.MustParse(r.GetId()), Action: "echo.connect",
		IP: netip.MustParseAddr("10.0.0.1"), Time: time.Now()})
	assert.True(t, dec.Allow, dec.Reasons)

	assert.True(t, s.allowed(ctx, t, owner, r.GetId()))
	_, err = s.acc.DeleteBinding(ctx, connect.NewRequest(&werbotv1.DeleteBindingRequest{Id: bid}))
	require.NoError(t, err)
	assert.False(t, s.allowed(ctx, t, owner, r.GetId()))
}

// The gateway builds snapshots without a user principal (stage 5).
func TestSnapshotWithoutPrincipal(t *testing.T) {
	s := newStage4(t, slog.New(slog.DiscardHandler))
	ctx, org, owner := s.org(t, "owner")
	r, err := s.echo(ctx, t, wrapperspb.String("x"), nil)
	require.NoError(t, err)
	s.bind(ctx, t, &werbotv1.CreateBindingRequest{SubjectType: "user", SubjectId: owner.String(), ResourceId: r.GetId()})

	snap, err := s.d.pol.Snapshot(context.Background(), org)
	require.NoError(t, err)
	require.Len(t, snap.Bindings, 1)
	assert.Equal(t, owner, snap.Bindings[0].SubjectID)
}

// Every replica loads the stored GeoIP database at start: without it country
// rules deny everything until the next geoip.refresh.
func TestNewDomainsReloadsGeoIP(t *testing.T) {
	pool, _ := dbtest.New(t)
	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, nil))

	stage4Of(t, pool, logger)
	assert.NotContains(t, logs.String(), "geoip reload failed", "no row is not an error")

	_, err := pool.Exec(context.Background(), `INSERT INTO geoip_db (id, month, data) VALUES (1, '2026-11-01', '\x00')`)
	require.NoError(t, err)
	stage4Of(t, pool, logger)
	assert.Contains(t, logs.String(), "geoip reload failed")
}

func TestServeRegistersStage4Services(t *testing.T) {
	pool, _ := dbtest.New(t)
	cfg := config.Config{HTTPAddr: freeAddr(t), WebEmbedded: true}
	startServe(t, cfg, pool, nil, fstest.MapFS{}, slog.New(slog.DiscardHandler))
	base, ctx := "http://"+cfg.HTTPAddr, context.Background()
	id := uuid.NewString()

	_, err := werbotv1connect.NewResourceServiceClient(http.DefaultClient, base).
		ListResourceTypes(ctx, connect.NewRequest(&werbotv1.ListResourceTypesRequest{}))
	assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err), "ResourceService")
	_, err = werbotv1connect.NewAccessServiceClient(http.DefaultClient, base).
		ListBindings(ctx, connect.NewRequest(&werbotv1.ListBindingsRequest{}))
	assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err), "AccessService")
	_, err = werbotv1connect.NewPolicyServiceClient(http.DefaultClient, base).
		CheckAccess(ctx, connect.NewRequest(&werbotv1.CheckAccessRequest{UserId: id, ResourceId: id, Ip: "10.0.0.1"}))
	assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err), "PolicyService")
}
