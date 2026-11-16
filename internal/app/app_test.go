package app

import (
	"bytes"
	"context"
	"io"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"os"
	"regexp"
	"sync"
	"testing"
	"testing/fstest"
	"time"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
	"github.com/werbot/werbot/gen/werbot/v1/werbotv1connect"
	"github.com/werbot/werbot/internal/platform/config"
	"github.com/werbot/werbot/internal/platform/db"
	"github.com/werbot/werbot/internal/platform/db/dbtest"
	"github.com/werbot/werbot/internal/platform/jobs"
	"github.com/werbot/werbot/sdk"
)

// Valid stage 3 values: Load requires both.
const (
	testPublicURL = "http://localhost:8080"
	testSecretKey = "AQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQE=" //nolint:gosec // test key, not a credential
)

// TestMain points the feed jobs at a closed port: serve runs them on start
// and they must neither reach the network nor hold River's Stop.
func TestMain(m *testing.M) {
	feedsConfig.BlocklistURL = "http://127.0.0.1:1/"
	feedsConfig.GeoIPURL = "http://127.0.0.1:1/%s"
	os.Exit(m.Run())
}

// withSecretKey fills the key serve requires when a test built cfg by hand.
func withSecretKey(cfg *config.Config) {
	if len(cfg.SecretKey) == 0 {
		cfg.SecretKey = bytes.Repeat([]byte{1}, 32)
	}
}

func env(kv map[string]string) func(string) string {
	return func(k string) string { return kv[k] }
}

func freeAddr(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := l.Addr().String()
	require.NoError(t, l.Close())
	return addr
}

func get(t *testing.T, url string) (int, string) {
	t.Helper()
	res, err := http.Get(url) //nolint:gosec // loopback test server
	require.NoError(t, err)
	defer func() { _ = res.Body.Close() }()
	body, err := io.ReadAll(res.Body)
	require.NoError(t, err)
	return res.StatusCode, string(body)
}

// startServe runs serve in a goroutine, waits for /healthz and returns a stop
// func that cancels ctx and asserts serve returns nil within 10s.
func startServe(t *testing.T, cfg config.Config, pool *pgxpool.Pool, modules []sdk.Module, ui fs.FS, logger *slog.Logger) (stop func()) {
	t.Helper()
	withSecretKey(&cfg)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- serve(ctx, cfg, logger, pool, modules, ui)
	}()
	require.Eventually(t, func() bool {
		res, err := http.Get("http://" + cfg.HTTPAddr + "/healthz")
		if err != nil {
			return false
		}
		_ = res.Body.Close()
		return res.StatusCode == http.StatusOK
	}, 5*time.Second, 20*time.Millisecond)
	stopped := false
	stop = func() {
		if stopped {
			return
		}
		stopped = true
		cancel()
		select {
		case err := <-done:
			assert.NoError(t, err)
		case <-time.After(10 * time.Second):
			t.Fatal("serve did not return within 10s")
		}
	}
	t.Cleanup(stop)
	return stop
}

func TestRunVersion(t *testing.T) {
	var out bytes.Buffer
	require.NoError(t, Run(context.Background(), []string{"version"}, env(nil), &out))
	assert.Equal(t, "werbot dev (commit none, built unknown)\n", out.String())
}

func TestRunUnknownCommand(t *testing.T) {
	err := Run(context.Background(), []string{"bogus"}, env(nil), io.Discard)
	assert.ErrorContains(t, err, "usage: werbot")
	err = Run(context.Background(), nil, env(nil), io.Discard)
	assert.ErrorContains(t, err, "usage: werbot")
}

func TestRunServerMissingDatabaseURL(t *testing.T) {
	assert.NotPanics(t, func() {
		err := Run(context.Background(), []string{"server"}, env(nil), io.Discard)
		assert.ErrorContains(t, err, "WERBOT_DATABASE_URL")
	})
}

func TestRunServerUnreachableDB(t *testing.T) {
	start := time.Now()
	err := Run(context.Background(), []string{"server"}, env(map[string]string{
		"WERBOT_DATABASE_URL": "postgres://postgres@127.0.0.1:1/x?sslmode=disable",
		"WERBOT_HTTP_ADDR":    freeAddr(t),
		"WERBOT_PUBLIC_URL":   testPublicURL,
		"WERBOT_SECRET_KEY":   testSecretKey,
	}), io.Discard)
	assert.ErrorContains(t, err, "bootstrap")
	assert.Less(t, time.Since(start), 15*time.Second)
}

func TestRunMigrateTwice(t *testing.T) {
	pool, url := dbtest.New(t)
	getenv := env(map[string]string{"WERBOT_DATABASE_URL": url, "WERBOT_PUBLIC_URL": testPublicURL, "WERBOT_SECRET_KEY": testSecretKey})
	require.NoError(t, Run(context.Background(), []string{"migrate"}, getenv, io.Discard))
	require.NoError(t, Run(context.Background(), []string{"migrate"}, getenv, io.Discard))
	var ok bool
	require.NoError(t, pool.QueryRow(context.Background(), "SELECT to_regclass('river_job') IS NOT NULL").Scan(&ok))
	assert.True(t, ok)
}

func TestBootstrapConcurrentRiverMigrate(t *testing.T) {
	pool, url := dbtest.New(t)
	ctx := context.Background()
	_, err := pool.Exec(ctx, "DROP SCHEMA public CASCADE; CREATE SCHEMA public")
	require.NoError(t, err)

	getenv := env(map[string]string{"WERBOT_DATABASE_URL": url, "WERBOT_PUBLIC_URL": testPublicURL, "WERBOT_SECRET_KEY": testSecretKey})
	bctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	errs := make([]error, 3)
	var wg sync.WaitGroup
	for i := range errs {
		wg.Go(func() { errs[i] = Run(bctx, []string{"migrate"}, getenv, io.Discard) })
	}
	wg.Wait()
	for _, err := range errs {
		assert.NoError(t, err)
	}

	var v int
	require.NoError(t, pool.QueryRow(ctx, "SELECT max(version) FROM river_migration").Scan(&v))
	assert.Equal(t, jobs.SchemaVersion(), v)
}

func TestBootstrapConcurrent(t *testing.T) {
	pool, url := dbtest.New(t)
	ctx := context.Background()
	_, err := pool.Exec(ctx, "DROP SCHEMA public CASCADE; CREATE SCHEMA public")
	require.NoError(t, err)

	bctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	errs := make([]error, 4)
	var wg sync.WaitGroup
	for i := range errs {
		wg.Go(func() { errs[i] = bootstrap(bctx, url) })
	}
	wg.Wait()
	for _, err := range errs {
		assert.NoError(t, err)
	}

	var n int
	require.NoError(t, pool.QueryRow(ctx, "SELECT count(*) FROM goose_db_version WHERE version_id = 1").Scan(&n))
	assert.Equal(t, 1, n)
	require.NoError(t, pool.QueryRow(ctx, "SELECT count(*) FROM organization").Scan(&n))
	assert.Equal(t, 0, n)
}

// emptySchemaLocked returns a test DB with an empty public schema and a
// connection that holds db.BootstrapLock until the returned unlock is called.
func emptySchemaLocked(t *testing.T) (pool *pgxpool.Pool, url string, unlock func()) {
	t.Helper()
	pool, url = dbtest.New(t)
	ctx := context.Background()
	_, err := pool.Exec(ctx, "DROP SCHEMA public CASCADE; CREATE SCHEMA public")
	require.NoError(t, err)
	conn, err := pool.Acquire(ctx)
	require.NoError(t, err)
	t.Cleanup(conn.Release)
	_, err = conn.Exec(ctx, "SELECT pg_advisory_lock($1)", db.BootstrapLock)
	require.NoError(t, err)
	return pool, url, func() {
		_, err := conn.Exec(ctx, "SELECT pg_advisory_unlock($1)", db.BootstrapLock)
		require.NoError(t, err)
	}
}

func gooseTableExists(t *testing.T, pool *pgxpool.Pool) bool {
	t.Helper()
	var ok bool
	require.NoError(t, pool.QueryRow(context.Background(), "SELECT to_regclass('goose_db_version') IS NOT NULL").Scan(&ok))
	return ok
}

func TestBootstrapWaitsForLock(t *testing.T) {
	pool, url, unlock := emptySchemaLocked(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- bootstrap(ctx, url) }()

	time.Sleep(300 * time.Millisecond) // give bootstrap time to reach the lock
	select {
	case err := <-done:
		t.Fatalf("bootstrap returned while the lock was held: %v", err)
	default:
	}
	assert.False(t, gooseTableExists(t, pool), "migrations ran while the lock was held")

	unlock()
	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(10 * time.Second):
		t.Fatal("bootstrap did not finish after the lock was released")
	}
	var n int
	require.NoError(t, pool.QueryRow(ctx, "SELECT count(*) FROM goose_db_version WHERE version_id = 1").Scan(&n))
	assert.Equal(t, 1, n)
}

func TestBootstrapLockWaitCanceled(t *testing.T) {
	pool, url, _ := emptySchemaLocked(t)
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	start := time.Now()
	err := bootstrap(ctx, url)
	assert.ErrorIs(t, err, context.DeadlineExceeded)
	assert.Less(t, time.Since(start), 5*time.Second)
	assert.False(t, gooseTableExists(t, pool), "migrations ran while the lock was held")
}

func TestServeHealthVersionShutdown(t *testing.T) {
	pool, _ := dbtest.New(t)
	cfg := config.Config{HTTPAddr: freeAddr(t), WebEmbedded: true}
	stop := startServe(t, cfg, pool, nil, fstest.MapFS{}, slog.New(slog.DiscardHandler))

	code, body := get(t, "http://"+cfg.HTTPAddr+"/readyz")
	assert.Equal(t, http.StatusOK, code)
	assert.Equal(t, "ok", body)

	_, err := werbotv1connect.NewSystemServiceClient(http.DefaultClient, "http://"+cfg.HTTPAddr).
		GetVersion(context.Background(), connect.NewRequest(&werbotv1.GetVersionRequest{}))
	require.NoError(t, err)

	stop()
}

func TestServeMetrics(t *testing.T) {
	pool, _ := dbtest.New(t)
	cfg := config.Config{HTTPAddr: freeAddr(t), MetricsAddr: freeAddr(t), WebEmbedded: true}
	startServe(t, cfg, pool, nil, fstest.MapFS{}, slog.New(slog.DiscardHandler))

	var (
		code       int
		ctype, out string
	)
	require.Eventually(t, func() bool {
		res, err := http.Get("http://" + cfg.MetricsAddr + "/metrics")
		if err != nil {
			return false
		}
		defer func() { _ = res.Body.Close() }()
		b, err := io.ReadAll(res.Body)
		code, ctype, out = res.StatusCode, res.Header.Get("Content-Type"), string(b)
		return err == nil
	}, 5*time.Second, 20*time.Millisecond)
	assert.Equal(t, http.StatusOK, code)
	assert.Equal(t, "text/plain; version=0.0.4", ctype)
	assert.Contains(t, out, "# TYPE werbot_db_pool_acquired_conns gauge")
	assert.Regexp(t, regexp.MustCompile(`(?m)^werbot_db_pool_total_conns [1-9][0-9]*$`), out)

	code, _ = get(t, "http://"+cfg.HTTPAddr+"/metrics")
	assert.Equal(t, http.StatusNotFound, code)
}

func TestServeSkipsUnentitledModule(t *testing.T) {
	pool, _ := dbtest.New(t)
	ok := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	modules := []sdk.Module{
		{Name: "free", Init: func(_ context.Context, h sdk.Host) error { h.Handle("/x/free", ok); return nil }},
		{Name: "paid", Feature: "saml", Init: func(_ context.Context, h sdk.Host) error { h.Handle("/x/paid", ok); return nil }},
	}
	cfg := config.Config{HTTPAddr: freeAddr(t), WebEmbedded: true}
	startServe(t, cfg, pool, modules, fstest.MapFS{}, slog.New(slog.DiscardHandler))

	code, _ := get(t, "http://"+cfg.HTTPAddr+"/x/free")
	assert.Equal(t, http.StatusOK, code)
	code, _ = get(t, "http://"+cfg.HTTPAddr+"/x/paid")
	assert.Equal(t, http.StatusNotFound, code)
}

func getHTML(t *testing.T, url string) (int, string) {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, url, nil)
	require.NoError(t, err)
	req.Header.Set("Accept", "text/html")
	res, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { _ = res.Body.Close() }()
	body, err := io.ReadAll(res.Body)
	require.NoError(t, err)
	return res.StatusCode, string(body)
}

func TestServeWebNotEmbedded(t *testing.T) {
	pool, _ := dbtest.New(t)
	ui := fstest.MapFS{"index.html": {Data: []byte("<html>app</html>")}}

	cfg := config.Config{HTTPAddr: freeAddr(t), WebEmbedded: true}
	stop := startServe(t, cfg, pool, nil, ui, slog.New(slog.DiscardHandler))
	code, body := getHTML(t, "http://"+cfg.HTTPAddr+"/")
	assert.Equal(t, http.StatusOK, code)
	assert.Equal(t, "<html>app</html>", body)
	stop()

	cfg = config.Config{HTTPAddr: freeAddr(t), WebEmbedded: false}
	startServe(t, cfg, pool, nil, ui, slog.New(slog.DiscardHandler))
	code, _ = getHTML(t, "http://"+cfg.HTTPAddr+"/")
	assert.Equal(t, http.StatusNotFound, code)
	code, _ = get(t, "http://"+cfg.HTTPAddr+"/healthz")
	assert.Equal(t, http.StatusOK, code)
	_, err := werbotv1connect.NewSystemServiceClient(http.DefaultClient, "http://"+cfg.HTTPAddr).
		GetVersion(context.Background(), connect.NewRequest(&werbotv1.GetVersionRequest{}))
	require.NoError(t, err)
}

func TestServeAddrInUse(t *testing.T) {
	pool, _ := dbtest.New(t)
	busy, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer func() { _ = busy.Close() }()
	addr := busy.Addr().String()

	var logs bytes.Buffer
	cfg := config.Config{HTTPAddr: freeAddr(t), MetricsAddr: addr, WebEmbedded: true}
	withSecretKey(&cfg)
	done := make(chan error, 1)
	go func() {
		done <- serve(context.Background(), cfg, slog.New(slog.NewTextHandler(&logs, nil)), pool, nil, fstest.MapFS{})
	}()
	select {
	case err := <-done:
		assert.ErrorContains(t, err, addr)
	case <-time.After(5 * time.Second):
		t.Fatal("serve did not return on a busy address")
	}
	assert.NotContains(t, logs.String(), "server started")
}
