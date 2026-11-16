package app

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net"
	"net/http"
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
	"github.com/werbot/werbot/internal/platform/db/dbtest"
	"github.com/werbot/werbot/sdk"
)

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
func startServe(t *testing.T, cfg config.Config, pool *pgxpool.Pool, modules []sdk.Module) (stop func()) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- serve(ctx, cfg, slog.New(slog.DiscardHandler), pool, modules, fstest.MapFS{})
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
	}), io.Discard)
	assert.ErrorContains(t, err, "bootstrap")
	assert.Less(t, time.Since(start), 15*time.Second)
}

func TestRunMigrateTwice(t *testing.T) {
	_, url := dbtest.New(t)
	getenv := env(map[string]string{"WERBOT_DATABASE_URL": url})
	require.NoError(t, Run(context.Background(), []string{"migrate"}, getenv, io.Discard))
	require.NoError(t, Run(context.Background(), []string{"migrate"}, getenv, io.Discard))
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

func TestServeHealthVersionShutdown(t *testing.T) {
	pool, _ := dbtest.New(t)
	cfg := config.Config{HTTPAddr: freeAddr(t), WebEmbedded: true}
	stop := startServe(t, cfg, pool, nil)

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
	startServe(t, cfg, pool, nil)

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
	startServe(t, cfg, pool, modules)

	code, _ := get(t, "http://"+cfg.HTTPAddr+"/x/free")
	assert.Equal(t, http.StatusOK, code)
	code, _ = get(t, "http://"+cfg.HTTPAddr+"/x/paid")
	assert.Equal(t, http.StatusNotFound, code)
}
