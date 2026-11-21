package app

import (
	"context"
	"log/slog"
	"net/http"
	"net/netip"
	"slices"
	"strings"
	"testing"
	"testing/fstest"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"

	gatewayv1 "github.com/werbot/werbot/gen/gateway/v1"
	sshv1 "github.com/werbot/werbot/gen/werbot/connector/ssh/v1"
	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
	"github.com/werbot/werbot/gen/werbot/v1/werbotv1connect"
	"github.com/werbot/werbot/internal/platform/config"
	"github.com/werbot/werbot/internal/platform/db/dbtest"
	"github.com/werbot/werbot/internal/platform/metrics"
	"github.com/werbot/werbot/internal/platform/principal"
	"github.com/werbot/werbot/internal/policy"
	"github.com/werbot/werbot/internal/tunnel"
)

// The organization part carries the path rules by id with access and effect
// (their globs for EvalPaths) and the path and terminal rules in the Cedar
// text; a new path rule changes the version, an unchanged build does not.
func TestSnapshotCarriesPathRules(t *testing.T) {
	s := newStage4(t, slog.New(slog.DiscardHandler))
	f := dbtest.Seed(t, s.pool)
	ctx := principal.With(context.Background(), f.Principal(f.OwnerID))
	build := func() *gatewayv1.Snapshot {
		snap, err := buildSnapshot(ctx, newTunnelDeps(s.d, metrics.NewRegistry(), slog.New(slog.DiscardHandler)).Snapshots, f.OrgID, f.GatewayID)
		require.NoError(t, err)
		return snap
	}
	target := policy.Selector{ResourceIDs: []uuid.UUID{f.ResourceID}}
	var ids []string
	for _, in := range []policy.PathRuleInput{
		{Name: "app", Paths: []string{"~/app/**"}, Access: "any", Effect: "only"},
		{Name: "secrets", Paths: []string{"/etc/shadow", "/root/**"}, Access: "read", Effect: "deny"},
	} {
		in.Subjects, in.Selector = policy.Subjects{AllUsers: true}, target
		r, err := s.d.Policy.CreatePathRule(ctx, f.OrgID, in)
		require.NoError(t, err)
		ids = append(ids, r.ID.String())
	}
	term, err := s.d.Policy.CreateTerminalRule(ctx, f.OrgID, policy.TerminalRuleInput{Name: "console",
		Subjects: policy.Subjects{AllUsers: true}, Selector: target, Mode: "console"})
	require.NoError(t, err)

	first := build()
	want := []*gatewayv1.PathRule{
		{Id: ids[0], Paths: []string{"~/app/**"}, Access: "any", Effect: "only"},
		{Id: ids[1], Paths: []string{"/etc/shadow", "/root/**"}, Access: "read", Effect: "deny"},
	}
	slices.SortFunc(want, func(a, b *gatewayv1.PathRule) int { return strings.Compare(a.GetId(), b.GetId()) })
	require.Len(t, first.GetPathRules(), 2)
	for i := range want {
		assert.True(t, proto.Equal(want[i], first.GetPathRules()[i]), "path rule %d: %v", i, first.GetPathRules()[i])
	}
	for _, id := range []string{`@id("path:` + ids[0] + `")`, `@id("path:` + ids[1] + `")`, `@id("terminal:` + term.ID.String() + `")`} {
		assert.Contains(t, first.GetCedarPolicies(), id)
	}
	assert.True(t, proto.Equal(first, build()), "unchanged data, same snapshot and version")

	_, err = s.d.Policy.CreatePathRule(ctx, f.OrgID, policy.PathRuleInput{Name: "logs", Subjects: policy.Subjects{AllUsers: true},
		Selector: target, Paths: []string{"/var/log/**"}, Access: "write", Effect: "approval"})
	require.NoError(t, err)
	second := build()
	assert.Len(t, second.GetPathRules(), 3)
	assert.False(t, proto.Equal(first, second), "a path rule changes the version")
}

// A stored path rule that does not compile sends the deny-all snapshot, and
// that snapshot carries no path rules (Ruling G19).
func TestDenyAllSnapshotHasNoPathRules(t *testing.T) {
	s := newStage4(t, slog.New(slog.DiscardHandler))
	f := dbtest.Seed(t, s.pool)
	ctx := principal.With(context.Background(), f.Principal(f.OwnerID))
	r, err := s.d.Policy.CreatePathRule(ctx, f.OrgID, policy.PathRuleInput{Name: "app", Subjects: policy.Subjects{AllUsers: true},
		Selector: policy.Selector{ResourceIDs: []uuid.UUID{f.ResourceID}}, Paths: []string{"~/app/**"}, Access: "any", Effect: "only"})
	require.NoError(t, err)
	_, err = s.pool.Exec(ctx, "UPDATE path_rule SET paths = ARRAY['app/**'] WHERE id = $1", r.ID) // a relative glob
	require.NoError(t, err)
	snap, err := buildSnapshot(ctx, newTunnelDeps(s.d, metrics.NewRegistry(), slog.New(slog.DiscardHandler)).Snapshots, f.OrgID, f.GatewayID)
	require.NoError(t, err)
	assert.Empty(t, snap.GetCedarPolicies(), "deny-all")
	assert.Empty(t, snap.GetPathRules())
}

// The sessions adapter stores the start paths and a console result, and
// passes a console denial with its session id, classes, paths and reasons.
func TestTunnelSessionsPathsResultDenial(t *testing.T) {
	s := newStage4(t, slog.New(slog.DiscardHandler))
	f := dbtest.Seed(t, s.pool)
	ctx := context.Background()
	sessions := newTunnelDeps(s.d, metrics.NewRegistry(), slog.New(slog.DiscardHandler)).Sessions
	id, err := sessions.Start(ctx, tunnel.SessionStart{OrgID: f.OrgID, GatewayID: f.GatewayID, InstanceID: f.InstanceID,
		UserID: f.MemberID, ResourceID: f.ResourceID, ClientIP: netip.MustParseAddr("10.0.0.1"), Kind: "terminal",
		Paths: []string{"/srv"}})
	require.NoError(t, err)
	var paths []string
	require.NoError(t, s.pool.QueryRow(ctx, "SELECT paths FROM session WHERE id = $1", id).Scan(&paths))
	assert.Equal(t, []string{"/srv"}, paths)

	require.NoError(t, sessions.Deny(ctx, f.OrgID, []tunnel.Denial{{SessionID: &id, Resource: "db", Kind: "terminal",
		Target: "cat /etc/hosts", Reason: "denied", Classes: []string{"read"}, Paths: []string{"/etc/hosts"}, Reasons: []string{"path:r1"}}}))
	var data string
	require.NoError(t, s.pool.QueryRow(ctx, `SELECT data::text FROM audit_event WHERE org_id = $1 AND action = 'session.command_denied'
		AND target_id = $2`, f.OrgID, id.String()).Scan(&data))
	assert.JSONEq(t, `{"command":"cat /etc/hosts","classes":["read"],"paths":["/etc/hosts"],"reasons":["path:r1"],"resource":"db","reason":"denied"}`, data)

	require.NoError(t, sessions.End(ctx, f.OrgID, f.InstanceID, id, "closed", nil,
		&tunnel.SessionResult{Command: "cp a b", Classes: []string{"read", "write"}, Paths: []string{"/srv/a", "/srv/b"}}))
	var command string
	var classes []string
	require.NoError(t, s.pool.QueryRow(ctx, "SELECT command, classes, paths FROM session WHERE id = $1", id).Scan(&command, &classes, &paths))
	assert.Equal(t, [3]any{"cp a b", []string{"read", "write"}, []string{"/srv/a", "/srv/b"}}, [3]any{command, classes, paths})
}

// Creating a path rule re-snapshots the connected gateway instance (a new
// version carrying the rule) within 2s, through the table trigger and the Hub.
func TestPathRuleChangeReachesGateway(t *testing.T) {
	pool, _ := dbtest.New(t)
	httpAddr := freeAddr(t)
	cfg := config.Config{HTTPAddr: httpAddr, MetricsAddr: freeAddr(t), GatewayAddr: freeAddr(t), Env: "development",
		PublicURL: "http://" + httpAddr, WebEmbedded: true}
	startServe(t, cfg, pool, nil, fstest.MapFS{}, slog.New(slog.DiscardHandler))
	ctx, api := context.Background(), "http://"+httpAddr
	hc, _ := jarClient(t, http.DefaultTransport)
	_, err := werbotv1connect.NewAuthServiceClient(hc, api).Setup(ctx, connect.NewRequest(ownerSetup))
	require.NoError(t, err)
	gws := werbotv1connect.NewGatewayServiceClient(hc, api)
	created, err := gws.CreateGateway(ctx, connect.NewRequest(&werbotv1.CreateGatewayRequest{Name: "edge"}))
	require.NoError(t, err)
	gwID := created.Msg.GetGateway().GetId()
	tok, err := gws.CreateJoinToken(ctx, connect.NewRequest(&werbotv1.CreateJoinTokenRequest{GatewayId: gwID}))
	require.NoError(t, err)
	gw := enroll(t, werbotv1connect.NewGatewayEnrollServiceClient(http.DefaultClient, api), tok.Msg.GetToken())
	sshCfg, err := anypb.New(&sshv1.Config{Host: "db.internal", Login: "root"})
	require.NoError(t, err)
	res, err := werbotv1connect.NewResourceServiceClient(hc, api).CreateResource(ctx, connect.NewRequest(&werbotv1.CreateResourceRequest{
		Name: "db", Type: "ssh", GatewayId: gwID, Config: sshCfg}))
	require.NoError(t, err)

	first, snaps := recvSnapshots(t, gw.client)
	require.NotEmpty(t, first.GetVersion())
	rule, err := werbotv1connect.NewPolicyServiceClient(hc, api).CreatePathRule(ctx, connect.NewRequest(&werbotv1.CreatePathRuleRequest{
		Name: "app", Subjects: &werbotv1.Subjects{AllUsers: true}, Paths: []string{"~/app/**"}, Access: "any", Effect: "only",
		ResourceSelector: &werbotv1.ResourceSelector{ResourceIds: []string{res.Msg.GetResource().GetId()}}}))
	require.NoError(t, err)
	ruled := awaitSnapshot(t, snaps, func(s *gatewayv1.Snapshot) bool { return len(s.GetPathRules()) == 1 })
	assert.Equal(t, rule.Msg.GetRule().GetId(), ruled.GetPathRules()[0].GetId())
	assert.NotEqual(t, first.GetVersion(), ruled.GetVersion())
}
