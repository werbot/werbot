package db_test

import (
	"context"
	"errors"
	"net/url"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/platform/db"
	"github.com/werbot/werbot/internal/platform/db/dbtest"
	"github.com/werbot/werbot/migrations"
)

func withParams(t *testing.T, raw string, params map[string]string) string {
	t.Helper()
	u, err := url.Parse(raw)
	require.NoError(t, err)
	q := u.Query()
	for k, v := range params {
		q.Set(k, v)
	}
	u.RawQuery = q.Encode()
	return u.String()
}

func showStatementTimeout(t *testing.T, pool *pgxpool.Pool) string {
	t.Helper()
	var v string
	require.NoError(t, pool.QueryRow(context.Background(), "SHOW statement_timeout").Scan(&v))
	return v
}

func TestMigrateIsIdempotent(t *testing.T) {
	pool, _ := dbtest.New(t) // шаблон уже мигрирован
	sqlDB := stdlib.OpenDBFromPool(pool)
	t.Cleanup(func() { _ = sqlDB.Close() }) // до pool.Close из dbtest: иначе Close ждёт соединения sqlDB
	require.NoError(t, db.Migrate(context.Background(), sqlDB, migrations.FS))
	var n int
	require.NoError(t, pool.QueryRow(context.Background(), "SELECT count(*) FROM organization").Scan(&n))
	assert.Equal(t, 0, n)
}

func TestOpenUnreachable(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	start := time.Now()
	_, err := db.Open(ctx, "postgres://postgres:password@127.0.0.1:1/x?sslmode=disable")
	assert.Error(t, err)
	assert.Less(t, time.Since(start), 5*time.Second)
}

func TestOpenRuntimeParams(t *testing.T) {
	ctx := context.Background()
	_, dsn := dbtest.New(t)

	pool, err := db.Open(ctx, dsn)
	require.NoError(t, err)
	defer pool.Close()
	assert.Equal(t, "30s", showStatementTimeout(t, pool))

	custom, err := db.Open(ctx, withParams(t, dsn, map[string]string{"statement_timeout": "100ms", "pool_max_conns": "3"}))
	require.NoError(t, err)
	defer custom.Close()
	assert.Equal(t, "100ms", showStatementTimeout(t, custom))
	assert.Equal(t, int32(3), custom.Config().MaxConns)

	_, err = custom.Exec(ctx, "SELECT pg_sleep(1)")
	var pgErr *pgconn.PgError
	require.ErrorAs(t, err, &pgErr)
	assert.Equal(t, "57014", pgErr.Code)
}

func TestOpenBootstrap(t *testing.T) {
	ctx := context.Background()
	_, dsn := dbtest.New(t)

	pool, err := db.OpenBootstrap(ctx, withParams(t, dsn, map[string]string{"statement_timeout": "100ms", "pool_max_conns": "10"}))
	require.NoError(t, err)
	defer pool.Close()
	assert.Equal(t, "0", showStatementTimeout(t, pool))
	assert.Equal(t, int32(2), pool.Config().MaxConns)
	_, err = pool.Exec(ctx, "SELECT pg_sleep(0.3)")
	assert.NoError(t, err)
}

func TestWithLockSerializes(t *testing.T) {
	pool, _ := dbtest.New(t)
	ctx := context.Background()

	var active, maxActive atomic.Int32
	errs := make([]error, 4)
	var wg sync.WaitGroup
	for i := range errs {
		wg.Go(func() {
			errs[i] = db.WithLock(ctx, pool, db.BootstrapLock, func(context.Context) error {
				n := active.Add(1)
				for {
					m := maxActive.Load()
					if n <= m || maxActive.CompareAndSwap(m, n) {
						break
					}
				}
				time.Sleep(50 * time.Millisecond)
				active.Add(-1)
				return nil
			})
		})
	}
	wg.Wait()
	for _, err := range errs {
		assert.NoError(t, err)
	}
	assert.Equal(t, int32(1), maxActive.Load())
}

func TestWithLockReleases(t *testing.T) {
	pool, _ := dbtest.New(t)
	errBoom := errors.New("boom")

	assertReleased := func() {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		assert.NoError(t, db.WithLock(ctx, pool, db.BootstrapLock, func(context.Context) error { return nil }))
	}

	err := db.WithLock(context.Background(), pool, db.BootstrapLock, func(context.Context) error { return errBoom })
	assert.ErrorIs(t, err, errBoom)
	assertReleased()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	err = db.WithLock(ctx, pool, db.BootstrapLock, func(ctx context.Context) error {
		cancel()
		return ctx.Err()
	})
	assert.Error(t, err)
	assertReleased()
}

func TestWithLockWaitHonorsContext(t *testing.T) {
	pool, _ := dbtest.New(t)

	held := make(chan struct{})
	release := make(chan struct{})
	firstErr := make(chan error, 1)
	go func() {
		firstErr <- db.WithLock(context.Background(), pool, db.BootstrapLock, func(context.Context) error {
			close(held)
			<-release
			return nil
		})
	}()
	<-held

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	called := false
	start := time.Now()
	err := db.WithLock(ctx, pool, db.BootstrapLock, func(context.Context) error {
		called = true
		return nil
	})
	assert.Error(t, err)
	assert.Less(t, time.Since(start), 2*time.Second)
	assert.False(t, called)

	close(release)
	assert.NoError(t, <-firstErr)
}
