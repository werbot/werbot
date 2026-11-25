package app

import (
	"context"
	"log/slog"
	"net/http"
	"net/netip"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"

	gatewayv1 "github.com/werbot/werbot/gen/gateway/v1"
	"github.com/werbot/werbot/gen/gateway/v1/gatewayv1connect"
	sshv1 "github.com/werbot/werbot/gen/werbot/connector/ssh/v1"
	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
	"github.com/werbot/werbot/gen/werbot/v1/werbotv1connect"
	"github.com/werbot/werbot/internal/inventory"
	"github.com/werbot/werbot/internal/platform/config"
	"github.com/werbot/werbot/internal/platform/db/dbtest"
	"github.com/werbot/werbot/internal/platform/metrics"
	"github.com/werbot/werbot/internal/platform/principal"
	"github.com/werbot/werbot/internal/policy"
	"github.com/werbot/werbot/internal/tunnel"
)

const reviewWritesCedar = `@obligation("approval")
forbid (principal, action == Werbot::Action::"ssh.exec", resource)
when { context.classes.contains("write") }
unless { context.approval_granted };`

// The organization part carries the command rules and custom policies in the
// Cedar text, the class rules by pattern (byte order, as on every replica) and
// the mask mode; the deny-all snapshot keeps the class rules and the mask mode.
func TestSnapshotCarriesStage11(t *testing.T) {
	s := newStage4(t, slog.New(slog.DiscardHandler))
	f := dbtest.Seed(t, s.pool)
	ctx := principal.With(context.Background(), f.Principal(f.OwnerID))
	r, err := s.d.Inventory.Create(ctx, f.OrgID, inventory.CreateInput{Name: "db", Type: "ssh", GatewayID: &f.GatewayID,
		Config: &sshv1.Config{Host: "db.internal", Login: "root"}})
	require.NoError(t, err)
	build := func() *gatewayv1.Snapshot {
		snap, err := buildSnapshot(ctx, newTunnelDeps(s.d, metrics.NewRegistry(), slog.New(slog.DiscardHandler)).Snapshots, f.OrgID, f.GatewayID)
		require.NoError(t, err)
		return snap
	}
	assert.Equal(t, "agents", build().GetMaskOutput(), "default mode")

	rule, err := s.d.Policy.CreateCommandRule(ctx, f.OrgID, policy.CommandRuleInput{Name: "no rm", Effect: "deny",
		Subjects: policy.Subjects{AllUsers: true}, Selector: policy.Selector{ResourceIDs: []uuid.UUID{r.ID}}, Classes: []string{"destructive"}})
	require.NoError(t, err)
	custom, err := s.d.Policy.CreateCustomPolicy(ctx, f.OrgID, policy.CustomPolicyInput{Name: "review", Cedar: reviewWritesCedar, Enabled: true})
	require.NoError(t, err)
	for _, p := range []string{"ab", "a-z"} { // the database collation orders these the other way
		_, err = s.d.Policy.CreateClassRule(ctx, f.OrgID, p, []string{"write"})
		require.NoError(t, err)
	}
	_, err = s.pool.Exec(ctx, "UPDATE org_setting SET mask_output = 'all' WHERE org_id = $1", f.OrgID)
	require.NoError(t, err)

	first := build()
	assert.Contains(t, first.GetCedarPolicies(), `@id("command:`+rule.ID.String()+`")`)
	assert.Contains(t, first.GetCedarPolicies(), `@id("custom:`+custom.ID.String()+`:0")`)
	assert.Equal(t, []*gatewayv1.ClassRule{{Pattern: "a-z", Classes: []string{"write"}}, {Pattern: "ab", Classes: []string{"write"}}},
		first.GetClassRules(), "ascending pattern")
	assert.Equal(t, "all", first.GetMaskOutput())
	assert.True(t, proto.Equal(first, build()), "unchanged data, same snapshot and version")

	_, err = s.d.Policy.CreateClassRule(ctx, f.OrgID, "deploy*", []string{"destructive"})
	require.NoError(t, err)
	second := build()
	assert.False(t, proto.Equal(first, second), "a class rule changes the version")
	assert.Len(t, second.GetClassRules(), 3)

	_, err = s.pool.Exec(ctx, "UPDATE custom_policy SET cedar = 'permit (' WHERE id = $1", custom.ID)
	require.NoError(t, err)
	denyAll := build()
	assert.Empty(t, denyAll.GetCedarPolicies())
	assert.True(t, proto.Equal(&gatewayv1.Snapshot{ClassRules: second.GetClassRules(), MaskOutput: "all"},
		&gatewayv1.Snapshot{ClassRules: denyAll.GetClassRules(), MaskOutput: denyAll.GetMaskOutput()}))
}

// The sessions adapter records an exec session's command and classes.
func TestTunnelSessionsStartCommand(t *testing.T) {
	s := newStage4(t, slog.New(slog.DiscardHandler))
	f := dbtest.Seed(t, s.pool)
	ctx := context.Background()
	sessions := newTunnelDeps(s.d, metrics.NewRegistry(), slog.New(slog.DiscardHandler)).Sessions
	id, err := sessions.Start(ctx, tunnel.SessionStart{OrgID: f.OrgID, GatewayID: f.GatewayID, InstanceID: f.InstanceID,
		UserID: f.MemberID, ResourceID: f.ResourceID, ClientIP: netip.MustParseAddr("10.0.0.1"), Kind: "exec",
		Command: "ls -la", Classes: []string{"read"}})
	require.NoError(t, err)
	var command string
	var classes []string
	require.NoError(t, s.pool.QueryRow(ctx, "SELECT command, classes FROM session WHERE id = $1", id).Scan(&command, &classes))
	assert.Equal(t, "ls -la", command)
	assert.Equal(t, []string{"read"}, classes)
}

// recvSnapshots returns the snapshots a gateway stream receives after its first one.
func recvSnapshots(t *testing.T, c gatewayv1connect.TunnelServiceClient) (*gatewayv1.Snapshot, <-chan *gatewayv1.Snapshot) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	st := c.Connect(ctx)
	require.NoError(t, st.Send(&gatewayv1.ConnectRequest{Msg: &gatewayv1.ConnectRequest_Heartbeat{Heartbeat: &gatewayv1.Heartbeat{Version: "test"}}}))
	first, err := st.Receive()
	require.NoError(t, err)
	out := make(chan *gatewayv1.Snapshot, 16)
	go func() {
		defer close(out)
		for {
			m, err := st.Receive()
			if err != nil {
				return
			}
			if snap := m.GetSnapshot(); snap != nil {
				out <- snap
			}
		}
	}()
	return first.GetSnapshot(), out
}

// awaitSnapshot waits up to 2s for a snapshot matching ok.
func awaitSnapshot(t *testing.T, ch <-chan *gatewayv1.Snapshot, ok func(*gatewayv1.Snapshot) bool) *gatewayv1.Snapshot {
	t.Helper()
	deadline := time.After(2 * time.Second)
	for {
		select {
		case snap, open := <-ch:
			require.True(t, open, "stream ended")
			if ok(snap) {
				return snap
			}
		case <-deadline:
			t.Fatal("no matching snapshot within 2s")
			return nil
		}
	}
}

// Creating a command rule or a class rule re-snapshots the connected gateway
// instance (a new version) within 2s, through the table triggers and the Hub.
func TestRuleChangeReachesGateway(t *testing.T) {
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
	pol := werbotv1connect.NewPolicyServiceClient(hc, api)
	rule, err := pol.CreateCommandRule(ctx, connect.NewRequest(&werbotv1.CreateCommandRuleRequest{Name: "no rm", Effect: "deny",
		Subjects: &werbotv1.Subjects{AllUsers: true}, Classes: []string{"destructive"},
		ResourceSelector: &werbotv1.ResourceSelector{ResourceIds: []string{res.Msg.GetResource().GetId()}}}))
	require.NoError(t, err)
	ruled := awaitSnapshot(t, snaps, func(s *gatewayv1.Snapshot) bool {
		return strings.Contains(s.GetCedarPolicies(), `@id("command:`+rule.Msg.GetRule().GetId()+`")`)
	})
	assert.NotEqual(t, first.GetVersion(), ruled.GetVersion())

	_, err = pol.CreateClassRule(ctx, connect.NewRequest(&werbotv1.CreateClassRuleRequest{Pattern: "deploy*", Classes: []string{"destructive"}}))
	require.NoError(t, err)
	classed := awaitSnapshot(t, snaps, func(s *gatewayv1.Snapshot) bool { return len(s.GetClassRules()) == 1 })
	assert.NotEqual(t, ruled.GetVersion(), classed.GetVersion())
	assert.Equal(t, "deploy*", classed.GetClassRules()[0].GetPattern())
}
