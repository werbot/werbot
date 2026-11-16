package app

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	gatewayv1 "github.com/werbot/werbot/gen/gateway/v1"
	"github.com/werbot/werbot/gen/gateway/v1/gatewayv1connect"
	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
	"github.com/werbot/werbot/gen/werbot/v1/werbotv1connect"
	"github.com/werbot/werbot/internal/gateway"
	"github.com/werbot/werbot/internal/inventory"
	"github.com/werbot/werbot/internal/platform/config"
	"github.com/werbot/werbot/internal/platform/db/dbtest"
	"github.com/werbot/werbot/internal/platform/metrics"
)

// fakeEmbedded replaces the embedded gateway with fn until the test ends.
func fakeEmbedded(t *testing.T, fn func(ctx context.Context) error) {
	t.Helper()
	embeddedGateway = func(ctx context.Context, _ config.Config, _ func(string) string, _ *pgxpool.Pool,
		_ *inventory.Gateways, _ *metrics.Registry, _ *slog.Logger) error {
		return fn(ctx)
	}
	t.Cleanup(func() { embeddedGateway = runEmbeddedGateway })
}

// serveWithGateway runs serve with WithGateway and waits for /healthz.
func serveWithGateway(t *testing.T, pool *pgxpool.Pool) (cfg config.Config, cancel context.CancelFunc, done <-chan error) {
	t.Helper()
	httpAddr := freeAddr(t)
	cfg = config.Config{HTTPAddr: httpAddr, PublicURL: "http://" + httpAddr, Env: "development", WebEmbedded: true}
	withTestDefaults(t, &cfg)
	o := testServeOpts(t, cfg, pool)
	o.WithGateway, o.Getenv = true, env(nil)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	errc := make(chan error, 1)
	go func() { errc <- serve(ctx, cfg, slog.New(slog.DiscardHandler), pool, nil, fstest.MapFS{}, o) }()
	require.Eventually(t, func() bool {
		res, err := http.Get("http://" + httpAddr + "/healthz")
		if err != nil {
			return false
		}
		_ = res.Body.Close()
		return res.StatusCode == http.StatusOK
	}, 5*time.Second, 20*time.Millisecond)
	return cfg, cancel, errc
}

func waitErr(t *testing.T, done <-chan error) error {
	t.Helper()
	select {
	case err := <-done:
		return err
	case <-time.After(15 * time.Second):
		t.Fatal("serve did not return within 15s")
		return nil
	}
}

// The embedded gateway drains after ctx ends and still sends EndSession
// through the tunnel: the tunnel plane must serve until it returned. A unary
// call on a fresh connection after cancel stands in for the drain's EndSession.
func TestServeKeepsTunnelUntilEmbeddedGatewayReturns(t *testing.T) {
	pool, _ := dbtest.New(t)
	clients := make(chan gatewayv1connect.TunnelServiceClient, 1)
	codes := make(chan connect.Code, 1)
	fakeEmbedded(t, func(ctx context.Context) error {
		<-ctx.Done()
		_, err := (<-clients).AuthenticateUser(context.Background(), connect.NewRequest(&gatewayv1.AuthenticateUserRequest{
			By: &gatewayv1.AuthenticateUserRequest_Login{Login: "nobody"}, ClientIp: "10.0.0.1"}))
		codes <- connect.CodeOf(err)
		return nil
	})
	cfg, cancel, done := serveWithGateway(t, pool)

	ctx, api := context.Background(), "http://"+cfg.HTTPAddr
	hc, _ := jarClient(t, http.DefaultTransport)
	_, err := werbotv1connect.NewAuthServiceClient(hc, api).Setup(ctx, connect.NewRequest(ownerSetup))
	require.NoError(t, err)
	gws := werbotv1connect.NewGatewayServiceClient(hc, api)
	created, err := gws.CreateGateway(ctx, connect.NewRequest(&werbotv1.CreateGatewayRequest{Name: "edge"}))
	require.NoError(t, err)
	tok, err := gws.CreateJoinToken(ctx, connect.NewRequest(&werbotv1.CreateJoinTokenRequest{GatewayId: created.Msg.GetGateway().GetId()}))
	require.NoError(t, err)
	clients <- enroll(t, werbotv1connect.NewGatewayEnrollServiceClient(http.DefaultClient, api), tok.Msg.GetToken()).client

	cancel()
	assert.Equal(t, connect.CodeNotFound, <-codes, "the tunnel answered after cancel")
	assert.NoError(t, waitErr(t, done))
}

func TestServeFailsWhenEmbeddedGatewayStops(t *testing.T) {
	pool, _ := dbtest.New(t)
	fakeEmbedded(t, func(context.Context) error { return nil })
	httpAddr := freeAddr(t)
	cfg := config.Config{HTTPAddr: httpAddr, PublicURL: "http://" + httpAddr, WebEmbedded: true}
	withTestDefaults(t, &cfg)
	o := testServeOpts(t, cfg, pool)
	o.WithGateway, o.Getenv = true, env(nil)
	err := serve(context.Background(), cfg, slog.New(slog.DiscardHandler), pool, nil, fstest.MapFS{}, o)
	assert.EqualError(t, err, "embedded gateway stopped")
}

func TestFirstOrganizationRetriesDBErrors(t *testing.T) {
	pool, _ := dbtest.New(t)
	ctx := context.Background()
	var org uuid.UUID
	require.NoError(t, pool.QueryRow(ctx, "INSERT INTO organization (name) VALUES ('Acme') RETURNING id").Scan(&org))
	_, err := pool.Exec(ctx, "ALTER TABLE organization RENAME TO organization_away")
	require.NoError(t, err)
	logs := &syncBuffer{}
	got := make(chan uuid.UUID, 1)
	go func() { got <- firstOrganization(ctx, pool, slog.New(slog.NewTextHandler(logs, nil))) }()
	require.Eventually(t, func() bool { return strings.Contains(logs.String(), "organization lookup failed") },
		5*time.Second, 20*time.Millisecond)
	_, err = pool.Exec(ctx, "ALTER TABLE organization_away RENAME TO organization")
	require.NoError(t, err)
	select {
	case id := <-got:
		assert.Equal(t, org, id)
	case <-time.After(5 * time.Second):
		t.Fatal("firstOrganization did not retry")
	}
}

// embeddedSetup is an organization and the server config of an embedded
// gateway whose server is not started yet.
func embeddedSetup(t *testing.T) (*pgxpool.Pool, config.Config, *inventory.Gateways) {
	t.Helper()
	pool, _ := dbtest.New(t)
	httpAddr := freeAddr(t)
	cfg := config.Config{HTTPAddr: httpAddr, PublicURL: "http://" + httpAddr, DataDir: t.TempDir(), WebEmbedded: true}
	withTestDefaults(t, &cfg)
	o := testServeOpts(t, cfg, pool)
	_, err := pool.Exec(context.Background(), "INSERT INTO organization (name) VALUES ('Acme')")
	require.NoError(t, err)
	return pool, cfg, inventory.NewGateways(pool, o.CA, o.Box)
}

func TestEmbeddedGatewayRetriesEnrollment(t *testing.T) {
	pool, cfg, gws := embeddedSetup(t)
	logs := &syncBuffer{}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		done <- runEmbeddedGateway(ctx, cfg, env(map[string]string{"WERBOT_GATEWAY_SSH_ADDR": "127.0.0.1:0"}), pool, gws,
			metrics.NewRegistry(), slog.New(slog.NewTextHandler(logs, nil)))
	}()
	require.Eventually(t, func() bool { return strings.Contains(logs.String(), "enrollment failed, retrying") },
		5*time.Second, 20*time.Millisecond, "no server yet")

	startServe(t, cfg, pool, nil, fstest.MapFS{}, slog.New(slog.DiscardHandler))
	require.Eventually(t, func() bool { return gateway.Enrolled(filepath.Join(cfg.DataDir, "gateway")) },
		10*time.Second, 20*time.Millisecond)
	var tokens int
	require.NoError(t, pool.QueryRow(context.Background(), "SELECT count(*) FROM gateway_join_token").Scan(&tokens))
	assert.Equal(t, 1, tokens, "failed attempts reused one token")
	cancel()
	select {
	case err := <-done:
		assert.NoError(t, err)
	case <-time.After(15 * time.Second):
		t.Fatal("embedded gateway did not return within 15s")
	}
}

// runEmbeddedWithoutServer starts the embedded gateway against a server that is
// not up yet and waits for its first failed attempt (one join token minted).
func runEmbeddedWithoutServer(t *testing.T, cfg config.Config, pool *pgxpool.Pool, gws *inventory.Gateways) {
	t.Helper()
	old := orgPollInterval
	orgPollInterval = 50 * time.Millisecond
	logs := &syncBuffer{}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- runEmbeddedGateway(ctx, cfg, env(map[string]string{"WERBOT_GATEWAY_SSH_ADDR": "127.0.0.1:0"}), pool, gws,
			metrics.NewRegistry(), slog.New(slog.NewTextHandler(logs, nil)))
	}()
	t.Cleanup(func() {
		cancel()
		select {
		case <-done:
		case <-time.After(15 * time.Second):
			t.Error("embedded gateway did not return within 15s")
		}
		orgPollInterval = old
	})
	require.Eventually(t, func() bool { return strings.Contains(logs.String(), "enrollment failed, retrying") },
		5*time.Second, 10*time.Millisecond, "no server yet")
}

func countTokens(t *testing.T, pool *pgxpool.Pool) (n int) {
	t.Helper()
	require.NoError(t, pool.QueryRow(context.Background(), "SELECT count(*) FROM gateway_join_token").Scan(&n))
	return n
}

func TestEmbeddedGatewayRemintsRejectedToken(t *testing.T) {
	pool, cfg, gws := embeddedSetup(t)
	runEmbeddedWithoutServer(t, cfg, pool, gws)
	require.Equal(t, 1, countTokens(t, pool))
	_, err := pool.Exec(context.Background(), "UPDATE gateway_join_token SET expires_at = now() - interval '1 second'")
	require.NoError(t, err)

	startServe(t, cfg, pool, nil, fstest.MapFS{}, slog.New(slog.DiscardHandler))
	require.Eventually(t, func() bool { return gateway.Enrolled(filepath.Join(cfg.DataDir, "gateway")) },
		10*time.Second, 20*time.Millisecond, "the rejected token is replaced")
	assert.Equal(t, 2, countTokens(t, pool))
}

func TestEmbeddedGatewayRemintsAgedToken(t *testing.T) {
	pool, cfg, gws := embeddedSetup(t)
	old := tokenRefresh
	tokenRefresh = 100 * time.Millisecond
	t.Cleanup(func() { tokenRefresh = old })
	runEmbeddedWithoutServer(t, cfg, pool, gws)
	require.Eventually(t, func() bool { return countTokens(t, pool) >= 2 }, 5*time.Second, 20*time.Millisecond,
		"a token older than tokenRefresh is replaced while the server stays down")
}

func TestEmbeddedGatewayReenrollsOnExpiredCert(t *testing.T) {
	pool, cfg, gws := embeddedSetup(t)
	old, oldRun := orgPollInterval, runGateway
	orgPollInterval = 20 * time.Millisecond
	t.Cleanup(func() { orgPollInterval, runGateway = old, oldRun })
	var tokens []string // the join token of each attempt
	secondRun := make(chan struct{})
	runGateway = func(ctx context.Context, c gateway.Config, _ *slog.Logger) error {
		tokens = append(tokens, c.JoinToken)
		if len(tokens) == 1 {
			return gateway.ErrCertExpired
		}
		close(secondRun)
		<-ctx.Done()
		return nil
	}
	logs := &syncBuffer{}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		done <- runEmbeddedGateway(ctx, cfg, env(nil), pool, gws, metrics.NewRegistry(), slog.New(slog.NewTextHandler(logs, nil)))
	}()
	select {
	case <-secondRun:
	case <-time.After(10 * time.Second):
		t.Fatal("no second attempt after an expired certificate")
	}
	cancel()
	require.NoError(t, <-done)

	require.Len(t, tokens, 2)
	assert.NotEmpty(t, tokens[0])
	assert.NotEqual(t, tokens[0], tokens[1], "the held token is dropped")
	assert.Equal(t, 2, countTokens(t, pool))
	assert.Contains(t, logs.String(), "certificate expired, enrolling again")
	assert.NotContains(t, logs.String(), "enrollment failed")
	assert.NotContains(t, logs.String(), tokens[0])
	assert.NotContains(t, logs.String(), tokens[1])
}

func TestEmbeddedGatewayListenErrorIsFatal(t *testing.T) {
	pool, cfg, gws := embeddedSetup(t)
	busy, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer func() { _ = busy.Close() }()
	err = runEmbeddedGateway(context.Background(), cfg, env(map[string]string{"WERBOT_GATEWAY_SSH_ADDR": busy.Addr().String()}),
		pool, gws, metrics.NewRegistry(), slog.New(slog.DiscardHandler))
	assert.ErrorContains(t, err, "gateway ssh listen")
}
