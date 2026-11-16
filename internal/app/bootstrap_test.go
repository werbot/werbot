package app

import (
	"context"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/platform/db"
	"github.com/werbot/werbot/internal/platform/db/dbtest"
	"github.com/werbot/werbot/internal/platform/jobs"
)

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
		wg.Go(func() { _, errs[i] = bootstrap(bctx, url, nil) })
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
	go func() {
		_, err := bootstrap(ctx, url, nil)
		done <- err
	}()

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
	_, err := bootstrap(ctx, url, nil)
	assert.ErrorIs(t, err, context.DeadlineExceeded)
	assert.Less(t, time.Since(start), 5*time.Second)
	assert.False(t, gooseTableExists(t, pool), "migrations ran while the lock was held")
}
