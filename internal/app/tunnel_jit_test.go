package app

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"testing"
	"testing/fstest"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/wrapperspb"

	gatewayv1 "github.com/werbot/werbot/gen/gateway/v1"
	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
	"github.com/werbot/werbot/gen/werbot/v1/werbotv1connect"
	"github.com/werbot/werbot/internal/access"
	"github.com/werbot/werbot/internal/platform/apperr"
	"github.com/werbot/werbot/internal/platform/config"
	"github.com/werbot/werbot/internal/platform/db/dbtest"
	"github.com/werbot/werbot/internal/platform/metrics"
	"github.com/werbot/werbot/internal/platform/principal"
	"github.com/werbot/werbot/internal/tunnel"
)

// The organization part of a snapshot carries the grant and lock expiries
// and the lock mode; a lock mode change gives a different snapshot.
func TestSnapshotExpirationsAndLockMode(t *testing.T) {
	s := newStage4(t, slog.New(slog.DiscardHandler))
	f := dbtest.Seed(t, s.pool)
	ctx := context.Background()
	f.Grant(t, s.pool, f.MemberID, f.ResourceID, 2*time.Hour)
	l := f.Lock(t, s.pool, "user", &f.OtherMemberID)
	_, err := s.pool.Exec(ctx, "UPDATE lock SET expires_at = now() + interval '1 hour' WHERE id = $1", l)
	require.NoError(t, err)
	f.Lock(t, s.pool, "resource", &f.ResourceID) // no expiry: no entry

	b := newTunnelDeps(s.d, metrics.NewRegistry(), slog.New(slog.DiscardHandler)).Snapshots
	first, err := b.Org(ctx, f.OrgID)
	require.NoError(t, err)
	pol, err := s.d.Policy.Snapshot(ctx, f.OrgID)
	require.NoError(t, err)
	assert.Len(t, first.GetExpirations(), 2)
	assert.Equal(t, pol.Expirations(), first.GetExpirations(), "ascending: the lock before the grant")
	assert.Equal(t, "best_effort", first.GetLockMode())

	_, err = s.pool.Exec(ctx, "UPDATE org_setting SET lock_mode = 'strict' WHERE org_id = $1", f.OrgID)
	require.NoError(t, err)
	second, err := b.Org(ctx, f.OrgID)
	require.NoError(t, err)
	assert.Equal(t, "strict", second.GetLockMode())
	assert.False(t, bytes.Equal(marshalSnapshot(t, first), marshalSnapshot(t, second)), "the version hashes this encoding")
}

// The approvals adapter keeps each gateway instance to its own command approvals.
func TestTunnelApprovals(t *testing.T) {
	s := newStage4(t, slog.New(slog.DiscardHandler))
	f := dbtest.Seed(t, s.pool)
	ctx := context.Background()
	a := tunnelApprovals{s.d.Access}
	in := tunnel.ApprovalRequest{OrgID: f.OrgID, InstanceID: f.InstanceID, UserID: f.MemberID, ResourceID: f.ResourceID, Command: "ls"}
	got, err := a.Request(ctx, in)
	require.NoError(t, err)
	assert.Equal(t, access.StatusPending, got.Status)
	assert.Equal(t, s.d.Access.RequestURL(got.ID), got.URL)
	assert.WithinDuration(t, time.Now().Add(5*time.Minute), got.ExpiresAt, time.Minute)

	_, err = a.Get(ctx, f.OrgID, f.OtherInstanceID, got.ID)
	require.ErrorIs(t, err, apperr.ErrNotFound, "another instance's approval")
	mine, err := a.Get(ctx, f.OrgID, f.InstanceID, got.ID)
	require.NoError(t, err)
	assert.Equal(t, access.StatusPending, mine.Status)
	in.UserID = uuid.New()
	_, err = a.Request(ctx, in)
	require.ErrorIs(t, err, apperr.ErrNotFound, "user not found")

	_, err = s.d.Access.Decide(ctx, f.Principal(f.OwnerID), got.ID, false, "")
	require.NoError(t, err)
	since := time.Now().Add(-tunnel.ApprovalResendWindow)
	decided, err := a.DecidedSince(ctx, f.OrgID, f.InstanceID, since)
	require.NoError(t, err)
	require.Len(t, decided, 1)
	assert.Equal(t, tunnel.Approval{ID: got.ID, Status: access.StatusDenied, ExpiresAt: got.ExpiresAt, URL: got.URL}, decided[0])
	other, err := a.DecidedSince(ctx, f.OrgID, f.OtherInstanceID, since)
	require.NoError(t, err)
	assert.Empty(t, other)
}

// fixedSettings are the default organization settings, for an access.Service of its own.
type fixedSettings struct{}

func (fixedSettings) AccessSettings(context.Context, uuid.UUID) (access.Settings, error) {
	return access.Settings{GrantDefaultTTL: 8 * time.Hour, GrantMaxTTL: 14 * 24 * time.Hour,
		RequestPendingTTL: time.Hour, CommandApprovalTTL: 5 * time.Minute}, nil
}

// A command decided by another replica's access.Service reaches the gateway
// stream: Decide -> NOTIFY werbot_approval_decided -> listenNotify -> Hub.
func TestApprovalNotifyReachesStream(t *testing.T) {
	pool, _ := dbtest.New(t)
	httpAddr := freeAddr(t)
	cfg := config.Config{HTTPAddr: httpAddr, GatewayAddr: freeAddr(t), Env: "development", PublicURL: "http://" + httpAddr, WebEmbedded: true}
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
	cfgAny, err := anypb.New(wrapperspb.String("x"))
	require.NoError(t, err)
	res, err := werbotv1connect.NewResourceServiceClient(hc, api).CreateResource(ctx, connect.NewRequest(&werbotv1.CreateResourceRequest{
		Name: "web", Type: "echo", GatewayId: gwID, Config: cfgAny}))
	require.NoError(t, err)
	var org, owner uuid.UUID
	require.NoError(t, pool.QueryRow(ctx, "SELECT org_id, user_id FROM membership WHERE role = 'owner'").Scan(&org, &owner))
	member := dbtest.User(t, pool, org, principal.RoleMember)

	sctx, cancel := context.WithCancel(ctx)
	t.Cleanup(cancel)
	st := gw.client.Connect(sctx)
	require.NoError(t, st.Send(&gatewayv1.ConnectRequest{Msg: &gatewayv1.ConnectRequest_Heartbeat{Heartbeat: &gatewayv1.Heartbeat{Version: "test"}}}))
	first, err := st.Receive()
	require.NoError(t, err)
	require.NotNil(t, first.GetSnapshot())
	decided := make(chan *gatewayv1.ApprovalDecided, 1)
	go func() {
		for {
			m, err := st.Receive()
			if err != nil {
				return
			}
			if d := m.GetApprovalDecided(); d != nil {
				decided <- d
			}
		}
	}()

	req, err := gw.client.RequestApproval(ctx, connect.NewRequest(&gatewayv1.RequestApprovalRequest{
		UserId: member.String(), ResourceId: res.Msg.GetResource().GetId(), Command: "rm -rf /tmp/x"}))
	require.NoError(t, err)
	id, err := uuid.Parse(req.Msg.GetApprovalId())
	require.NoError(t, err)
	other := access.New(pool, nil, fixedSettings{}, access.Config{})
	go func() {
		_, err := other.Decide(ctx, principal.Principal{UserID: owner, OrgID: org, Role: principal.RoleOwner}, id, true, "")
		assert.NoError(t, err)
	}()
	select {
	case d := <-decided:
		assert.Equal(t, id.String(), d.GetApprovalId())
		assert.Equal(t, access.StatusApproved, d.GetStatus())
	case <-time.After(2 * time.Second):
		t.Fatal("no approval_decided within 2s")
	}
	got, err := gw.client.GetApproval(ctx, connect.NewRequest(&gatewayv1.GetApprovalRequest{ApprovalId: id.String()}))
	require.NoError(t, err)
	assert.Equal(t, access.StatusApproved, got.Msg.GetStatus())
}
