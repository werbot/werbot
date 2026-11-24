package app

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"net/netip"
	"slices"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/pquerna/otp/totp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/wrapperspb"

	gatewayv1 "github.com/werbot/werbot/gen/gateway/v1"
	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
	"github.com/werbot/werbot/gen/werbot/v1/werbotv1connect"
	"github.com/werbot/werbot/internal/identity"
	"github.com/werbot/werbot/internal/platform/apperr"
	"github.com/werbot/werbot/internal/platform/config"
	"github.com/werbot/werbot/internal/platform/db/dbtest"
	"github.com/werbot/werbot/internal/platform/secretbox"
	"github.com/werbot/werbot/internal/tunnel"
)

var mfaKeyBinding = "key:SHA256:" + strings.Repeat("A", 43)

// The MFA adapter keeps each instance to its own challenges, answers a wrong
// code as false and a missing verification as the maximum age.
func TestTunnelMFA(t *testing.T) {
	pool, _ := dbtest.New(t)
	f := dbtest.Seed(t, pool)
	ctx := context.Background()
	box, err := secretbox.New(bytes.Repeat([]byte{3}, 32), nil)
	require.NoError(t, err)
	m := tunnelMFA{identity.New(pool, nil, identity.Config{WebURL: "https://werbot.test", Box: box})}
	secret := f.TOTP(t, pool, box, f.MemberID)

	r := tunnel.MFAChallengeRequest{OrgID: f.OrgID, InstanceID: f.InstanceID, UserID: f.MemberID, ResourceID: f.ResourceID,
		SSHSessionID: []byte("s"), ClientIP: netip.MustParseAddr("198.51.100.4"), Binding: mfaKeyBinding, Strength: "any"}
	id, code, url, exp, err := m.Create(ctx, r)
	require.NoError(t, err)
	assert.Regexp(t, `^[A-Z]{4}-[A-Z]{4}$`, code)
	assert.Equal(t, "https://werbot.test/mfa/"+id.String(), url)
	assert.WithinDuration(t, time.Now().Add(identity.ChallengeTTL), exp, 5*time.Second)

	_, err = m.Status(ctx, f.OrgID, f.OtherInstanceID, id)
	require.ErrorIs(t, err, apperr.ErrNotFound, "another instance's challenge")
	now, err := totp.GenerateCode(secret, time.Now())
	require.NoError(t, err)
	ok, err := m.VerifyTOTP(ctx, f.OrgID, f.InstanceID, id, wrongTOTP(t, secret))
	require.NoError(t, err)
	assert.False(t, ok, "a wrong code is false, not an error")
	ok, err = m.VerifyTOTP(ctx, f.OrgID, f.InstanceID, id, now)
	require.NoError(t, err)
	assert.True(t, ok)
	s, err := m.Status(ctx, f.OrgID, f.InstanceID, id)
	require.NoError(t, err)
	assert.Equal(t, "confirmed", s)
	ids, err := m.Confirmed(ctx, f.OrgID, f.InstanceID)
	require.NoError(t, err)
	assert.Equal(t, []uuid.UUID{id}, ids)

	age, method, err := m.Age(ctx, f.OrgID, f.MemberID, mfaKeyBinding)
	require.NoError(t, err)
	assert.Equal(t, "totp", method)
	assert.Less(t, age, int64(5))
	age, method, err = m.Age(ctx, f.OrgID, f.OtherMemberID, mfaKeyBinding)
	require.NoError(t, err)
	assert.Equal(t, int64(math.MaxInt64), age)
	assert.Empty(t, method)
}

// wrongTOTP is a code of no step within a minute of now.
func wrongTOTP(t *testing.T, secret string) string {
	t.Helper()
	var window []string
	for _, d := range []time.Duration{-time.Minute, -30 * time.Second, 0, 30 * time.Second, time.Minute} {
		c, err := totp.GenerateCode(secret, time.Now().Add(d))
		require.NoError(t, err)
		window = append(window, c)
	}
	for i := 0; ; i++ {
		if c := fmt.Sprintf("%06d", i); !slices.Contains(window, c) {
			return c
		}
	}
}

// A challenge confirmed with a TOTP code reaches the gateway stream:
// VerifyMFATOTP -> NOTIFY werbot_mfa_confirmed -> listenNotify -> Hub. The
// challenge's user reads it through MFAChallengeService.
func TestMFANotifyReachesStream(t *testing.T) {
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
	// The factor comes after the admin set-up, which would otherwise need a step-up.
	enr, err := werbotv1connect.NewMFAServiceClient(hc, api).EnrollTOTP(ctx, connect.NewRequest(&werbotv1.EnrollTOTPRequest{}))
	require.NoError(t, err)
	secret := enr.Msg.GetSecret()
	confirm, err := totp.GenerateCode(secret, time.Now().Add(-30*time.Second))
	require.NoError(t, err)
	_, err = werbotv1connect.NewMFAServiceClient(hc, api).ConfirmTOTP(ctx, connect.NewRequest(&werbotv1.ConfirmTOTPRequest{Code: confirm}))
	require.NoError(t, err)
	var owner uuid.UUID
	require.NoError(t, pool.QueryRow(ctx, "SELECT user_id FROM membership WHERE role = 'owner'").Scan(&owner))

	sctx, cancel := context.WithCancel(ctx)
	t.Cleanup(cancel)
	st := gw.client.Connect(sctx)
	require.NoError(t, st.Send(&gatewayv1.ConnectRequest{Msg: &gatewayv1.ConnectRequest_Heartbeat{Heartbeat: &gatewayv1.Heartbeat{Version: "test"}}}))
	first, err := st.Receive()
	require.NoError(t, err)
	require.NotNil(t, first.GetSnapshot())
	confirmed := make(chan string, 1)
	go func() {
		for {
			m, err := st.Receive()
			if err != nil {
				return
			}
			if c := m.GetMfaConfirmed(); c != nil {
				confirmed <- c.GetChallengeId()
			}
		}
	}()

	ch, err := gw.client.CreateMFAChallenge(ctx, connect.NewRequest(&gatewayv1.CreateMFAChallengeRequest{UserId: owner.String(),
		ResourceId: res.Msg.GetResource().GetId(), SshSessionId: []byte("s"), ClientIp: "198.51.100.4", Binding: mfaKeyBinding, Strength: "any"}))
	require.NoError(t, err)
	id := ch.Msg.GetChallengeId()
	got, err := werbotv1connect.NewMFAChallengeServiceClient(hc, api).GetChallenge(ctx, connect.NewRequest(&werbotv1.GetChallengeRequest{Id: id}))
	require.NoError(t, err)
	assert.Equal(t, []string{ch.Msg.GetCode(), "gw", "198.51.100.4", "web", "any"},
		[]string{got.Msg.GetCode(), got.Msg.GetHost(), got.Msg.GetClientIp(), got.Msg.GetResource(), got.Msg.GetStrength()})

	code, err := totp.GenerateCode(secret, time.Now().Add(30*time.Second))
	require.NoError(t, err)
	v, err := gw.client.VerifyMFATOTP(ctx, connect.NewRequest(&gatewayv1.VerifyMFATOTPRequest{ChallengeId: id, Code: code}))
	require.NoError(t, err)
	require.True(t, v.Msg.GetOk())
	select {
	case c := <-confirmed:
		assert.Equal(t, id, c)
	case <-time.After(2 * time.Second):
		t.Fatal("no mfa_confirmed within 2s")
	}
	s, err := gw.client.GetMFAChallenge(ctx, connect.NewRequest(&gatewayv1.GetMFAChallengeRequest{ChallengeId: id}))
	require.NoError(t, err)
	assert.Equal(t, "confirmed", s.Msg.GetStatus())
}
