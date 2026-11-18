package app

import (
	"context"
	"log/slog"
	"net/http"
	"testing"
	"testing/fstest"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	gatewayv1 "github.com/werbot/werbot/gen/gateway/v1"
	"github.com/werbot/werbot/gen/gateway/v1/gatewayv1connect"
	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
	"github.com/werbot/werbot/gen/werbot/v1/werbotv1connect"
	"github.com/werbot/werbot/internal/platform/config"
	"github.com/werbot/werbot/internal/platform/db/dbtest"
)

// A cached mTLS instance check ends with the instance: deleting it, or its
// pool, notifies werbot_org_changed and the next request goes to the database.
func TestTunnelAuthCacheDroppedOnDelete(t *testing.T) {
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
	enrollSvc := werbotv1connect.NewGatewayEnrollServiceClient(http.DefaultClient, api)
	newPool := func(name string) (string, enrolled) {
		created, err := gws.CreateGateway(ctx, connect.NewRequest(&werbotv1.CreateGatewayRequest{Name: name}))
		require.NoError(t, err)
		id := created.Msg.GetGateway().GetId()
		tok, err := gws.CreateJoinToken(ctx, connect.NewRequest(&werbotv1.CreateJoinTokenRequest{GatewayId: id}))
		require.NoError(t, err)
		return id, enroll(t, enrollSvc, tok.Msg.GetToken())
	}
	auth := func(c gatewayv1connect.TunnelServiceClient) connect.Code {
		_, err := c.AuthenticateUser(ctx, connect.NewRequest(&gatewayv1.AuthenticateUserRequest{
			By: &gatewayv1.AuthenticateUserRequest_Login{Login: "nobody"}, ClientIp: "10.0.0.1"}))
		return connect.CodeOf(err)
	}
	// cached waits until the check of e is answered from the cache: with the
	// serial changed behind the server's back (no notification fires for
	// cert_serial alone), the database would refuse it.
	cached := func(e enrolled) {
		t.Helper()
		var serial string
		require.NoError(t, pool.QueryRow(ctx, "SELECT cert_serial FROM gateway_instance WHERE id = $1", e.res.GetInstanceId()).Scan(&serial))
		setSerial := func(s string) {
			_, err := pool.Exec(ctx, "UPDATE gateway_instance SET cert_serial = $2 WHERE id = $1", e.res.GetInstanceId(), s)
			require.NoError(t, err)
		}
		require.Eventually(t, func() bool {
			auth(e.client)
			setSerial("0")
			defer setSerial(serial)
			return auth(e.client) == connect.CodeNotFound
		}, 5*time.Second, 20*time.Millisecond)
	}
	refused := func(e enrolled, what string) {
		t.Helper()
		assert.Eventually(t, func() bool { return auth(e.client) == connect.CodeUnauthenticated }, 5*time.Second, 20*time.Millisecond, what)
	}

	_, a := newPool("edge")
	cached(a)
	_, err = gws.DeleteGatewayInstance(ctx, connect.NewRequest(&werbotv1.DeleteGatewayInstanceRequest{Id: a.res.GetInstanceId()}))
	require.NoError(t, err)
	refused(a, "instance deleted")

	id, b := newPool("other")
	cached(b)
	_, err = gws.DeleteGateway(ctx, connect.NewRequest(&werbotv1.DeleteGatewayRequest{Id: id}))
	require.NoError(t, err)
	refused(b, "pool deleted")
}
