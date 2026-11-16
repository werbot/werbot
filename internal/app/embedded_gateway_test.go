package app

import (
	"context"
	"io"
	"net/http"
	"os"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
	"github.com/werbot/werbot/gen/werbot/v1/werbotv1connect"
	"github.com/werbot/werbot/internal/platform/db/dbtest"
)

func TestRunGatewayRequiresServerURL(t *testing.T) {
	err := Run(context.Background(), []string{"gateway", "--data-dir", t.TempDir(), "--ssh-addr", "127.0.0.1:0"}, env(nil), io.Discard)
	assert.ErrorContains(t, err, "WERBOT_GATEWAY_SERVER_URL")
}

func TestRunHelpIsNotAnError(t *testing.T) {
	assert.NoError(t, Run(context.Background(), []string{"gateway", "--help"}, env(nil), io.Discard))
	assert.NoError(t, Run(context.Background(), []string{"server", "--help"}, env(nil), io.Discard))
}

func TestRunServerRejectsUnknownFlag(t *testing.T) {
	err := Run(context.Background(), []string{"server", "--bogus"}, env(nil), io.Discard)
	assert.ErrorContains(t, err, "bogus")
}

// replica is one `werbot server --with-gateway` started by startReplica.
type replica struct {
	api  string // http://<HTTPAddr>
	stop func() // cancels Run and asserts it returns nil within 15s
}

// startReplica runs `werbot server --with-gateway` on free loopback ports with
// the embedded gateway state under dataDir; it does not wait for it (see healthy).
func startReplica(t *testing.T, dbURL, dataDir string) replica {
	t.Helper()
	httpAddr := freeAddr(t)
	getenv := env(map[string]string{
		"WERBOT_DATABASE_URL": dbURL, "WERBOT_HTTP_ADDR": httpAddr, "WERBOT_PUBLIC_URL": "http://" + httpAddr,
		"WERBOT_SECRET_KEY": testSecretKey, "WERBOT_GATEWAY_ADDR": freeAddr(t), "WERBOT_DATA_DIR": dataDir,
		"WERBOT_ENV": "development", "WERBOT_GATEWAY_SSH_ADDR": "127.0.0.1:0",
	})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- Run(ctx, []string{"server", "--with-gateway"}, getenv, io.Discard) }()
	r := replica{api: "http://" + httpAddr}
	stopped := false
	r.stop = func() {
		if stopped {
			return
		}
		stopped = true
		cancel()
		select {
		case err := <-done:
			assert.NoError(t, err)
		case <-time.After(15 * time.Second):
			t.Fatal("Run did not return within 15s")
		}
	}
	t.Cleanup(r.stop)
	return r
}

// healthy waits until the replica answers /healthz.
func (r replica) healthy(t *testing.T) replica {
	t.Helper()
	require.Eventually(t, func() bool {
		res, err := http.Get(r.api + "/healthz")
		if err != nil {
			return false
		}
		_ = res.Body.Close()
		return res.StatusCode == http.StatusOK
	}, 15*time.Second, 50*time.Millisecond)
	return r
}

// localPool waits until the pool "local" has want online instances and returns it.
func localPool(t *testing.T, gws werbotv1connect.GatewayServiceClient, want int, within time.Duration) *werbotv1.Gateway {
	t.Helper()
	var local *werbotv1.Gateway
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		res, err := gws.ListGateways(context.Background(), connect.NewRequest(&werbotv1.ListGatewaysRequest{}))
		require.NoError(c, err)
		require.Len(c, res.Msg.GetGateways(), 1)
		local = res.Msg.GetGateways()[0]
		require.Equal(c, "local", local.GetName())
		online := 0
		for _, in := range local.GetInstances() {
			if in.GetOnline() {
				online++
			}
		}
		require.Equal(c, want, online)
	}, within, 100*time.Millisecond)
	return local
}

func TestServerWithGatewayEnrollsLocal(t *testing.T) {
	pool, url := dbtest.New(t)
	dataDir := t.TempDir()
	ctx := context.Background()
	r := startReplica(t, url, dataDir).healthy(t)

	var pools int
	require.NoError(t, pool.QueryRow(ctx, "SELECT count(*) FROM gateway").Scan(&pools))
	assert.Zero(t, pools, "no pool before Setup")

	hc, _ := jarClient(t, http.DefaultTransport)
	_, err := werbotv1connect.NewAuthServiceClient(hc, r.api).Setup(ctx, connect.NewRequest(ownerSetup))
	require.NoError(t, err)
	local := localPool(t, werbotv1connect.NewGatewayServiceClient(hc, r.api), 1, 15*time.Second)
	require.Len(t, local.GetInstances(), 1)
	in := local.GetInstances()[0]
	host, err := os.Hostname()
	require.NoError(t, err)
	assert.Equal(t, host, in.GetHostname())
	assert.Contains(t, in.GetConnectors(), "ssh")
	r.stop()

	// A restart with the same data dir reuses the identity: no new instance.
	r = startReplica(t, url, dataDir).healthy(t)
	gws := werbotv1connect.NewGatewayServiceClient(hc, r.api)
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		res, err := gws.ListGateways(ctx, connect.NewRequest(&werbotv1.ListGatewaysRequest{}))
		require.NoError(c, err)
		require.Len(c, res.Msg.GetGateways(), 1)
		ins := res.Msg.GetGateways()[0].GetInstances()
		require.Len(c, ins, 1)
		assert.Equal(c, in.GetId(), ins[0].GetId())
		assert.True(c, ins[0].GetLastSeenAt().AsTime().After(in.GetLastSeenAt().AsTime()), "reconnected")
	}, 15*time.Second, 100*time.Millisecond)
}

func TestTwoReplicasJoinLocal(t *testing.T) {
	pool, url := dbtest.New(t)
	ctx := context.Background()
	// Both start at once: bootstrap, the CA and the pool "local" race.
	a, b := startReplica(t, url, t.TempDir()), startReplica(t, url, t.TempDir())
	a.healthy(t)
	b.healthy(t)

	hc, _ := jarClient(t, http.DefaultTransport)
	_, err := werbotv1connect.NewAuthServiceClient(hc, a.api).Setup(ctx, connect.NewRequest(ownerSetup))
	require.NoError(t, err)
	gws := werbotv1connect.NewGatewayServiceClient(hc, a.api)
	local := localPool(t, gws, 2, 20*time.Second)
	assert.Len(t, local.GetInstances(), 2)

	toks, err := gws.ListJoinTokens(ctx, connect.NewRequest(&werbotv1.ListJoinTokensRequest{GatewayId: local.GetId()}))
	require.NoError(t, err)
	require.Len(t, toks.Msg.GetTokens(), 2)
	for _, tok := range toks.Msg.GetTokens() {
		assert.Equal(t, int32(1), tok.GetUses())
		assert.Equal(t, int32(1), tok.GetMaxUses())
	}
	var keys int
	require.NoError(t, pool.QueryRow(ctx, "SELECT count(*) FROM ca_key").Scan(&keys))
	assert.Equal(t, 1, keys)
}
