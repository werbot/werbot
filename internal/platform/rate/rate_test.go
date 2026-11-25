package rate_test

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/platform/db"
	"github.com/werbot/werbot/internal/platform/db/dbtest"
	"github.com/werbot/werbot/internal/platform/jobs"
	"github.com/werbot/werbot/internal/platform/rate"
)

// Postgres now() sets the window, so these tests run on the real clock, not
// in a synctest bubble; rollover moves the stored window instead of waiting.

func TestRateAllow(t *testing.T) {
	pool, _ := dbtest.New(t)
	ctx := context.Background()
	for i := range 3 {
		ok, err := rate.Allow(ctx, pool, "login:ip:10.0.0.1", 3, time.Minute)
		require.NoError(t, err)
		assert.True(t, ok, "call %d", i+1)
	}
	ok, err := rate.Allow(ctx, pool, "login:ip:10.0.0.1", 3, time.Minute)
	require.NoError(t, err)
	assert.False(t, ok, "the 4th call in the window is refused")
	ok, err = rate.Allow(ctx, pool, "login:ip:10.0.0.2", 3, time.Minute)
	require.NoError(t, err)
	assert.True(t, ok, "another key has its own count")
}

func TestRateWindowRollover(t *testing.T) {
	pool, _ := dbtest.New(t)
	ctx := context.Background()
	ok, err := rate.Allow(ctx, pool, "k", 1, time.Minute)
	require.NoError(t, err)
	require.True(t, ok)
	ok, err = rate.Allow(ctx, pool, "k", 1, time.Minute)
	require.NoError(t, err)
	require.False(t, ok)

	_, err = pool.Exec(ctx, "UPDATE rate_limit SET window_start = window_start - interval '1 minute'")
	require.NoError(t, err)
	ok, err = rate.Allow(ctx, pool, "k", 1, time.Minute)
	require.NoError(t, err)
	assert.True(t, ok, "the next window starts a new count")
}

func TestRateConcurrent(t *testing.T) {
	pool, _ := dbtest.New(t)
	var allowed atomic.Int32
	var wg sync.WaitGroup
	for range 50 {
		wg.Go(func() {
			ok, err := rate.Allow(context.Background(), pool, "k", 10, time.Minute)
			assert.NoError(t, err)
			if ok {
				allowed.Add(1)
			}
		})
	}
	wg.Wait()
	assert.Equal(t, int32(10), allowed.Load())
}

// TestRateSharedAcrossReplicas: two replicas (pools) on one database share
// one counter row per key and window.
func TestRateSharedAcrossReplicas(t *testing.T) {
	poolA, url := dbtest.New(t)
	ctx := context.Background()
	poolB, err := db.Open(ctx, url)
	require.NoError(t, err)
	t.Cleanup(poolB.Close)

	for _, pool := range []*pgxpool.Pool{poolA, poolB, poolA} {
		ok, err := rate.Allow(ctx, pool, "k", 3, time.Minute)
		require.NoError(t, err)
		assert.True(t, ok)
	}
	ok, err := rate.Allow(ctx, poolB, "k", 3, time.Minute)
	require.NoError(t, err)
	assert.False(t, ok, "replica B sees the calls of replica A")

	var rows, count int
	require.NoError(t, poolA.QueryRow(ctx, "SELECT count(*), sum(count) FROM rate_limit WHERE key = 'k'").Scan(&rows, &count))
	assert.Equal(t, 1, rows)
	assert.Equal(t, 4, count)
}

// TestRateAllowStoreErrorDenies: a store error refuses the call (fail-closed).
func TestRateAllowStoreErrorDenies(t *testing.T) {
	pool, _ := dbtest.New(t)
	pool.Close()
	ok, err := rate.Allow(context.Background(), pool, "k", 100, time.Minute)
	require.Error(t, err)
	assert.False(t, ok)
}

// TestRateCountSurvivesCallerRollback: Allow counts in its own transaction, so
// a call stays counted when the caller's effect transaction rolls back.
func TestRateCountSurvivesCallerRollback(t *testing.T) {
	pool, _ := dbtest.New(t)
	ctx := context.Background()
	tx, err := pool.Begin(ctx)
	require.NoError(t, err)
	ok, err := rate.Allow(ctx, pool, "mfa:u", 1, 5*time.Minute)
	require.NoError(t, err)
	require.True(t, ok)
	require.NoError(t, tx.Rollback(ctx))

	ok, err = rate.Allow(ctx, pool, "mfa:u", 1, 5*time.Minute)
	require.NoError(t, err)
	assert.False(t, ok, "the first call is still counted")
}

func TestRateGC(t *testing.T) {
	pool, _ := dbtest.New(t)
	ctx := context.Background()
	_, err := pool.Exec(ctx, `INSERT INTO rate_limit (key, window_start, count) VALUES
		('old', now() - interval '2 hours 1 minute', 1), ('fresh', now() - interval '1 hour 59 minutes', 1)`)
	require.NoError(t, err)
	n, err := rate.GC(ctx, pool, rate.GCOlderThan)
	require.NoError(t, err)
	assert.Equal(t, int64(1), n)
	var key string
	require.NoError(t, pool.QueryRow(ctx, "SELECT key FROM rate_limit").Scan(&key))
	assert.Equal(t, "fresh", key)

	assert.Equal(t, 2*time.Hour, rate.GCOlderThan)
	assert.Equal(t, "rate_limit.gc", rate.GCArgs{}.Kind())
	assert.Equal(t, jobs.QueueMaintenance, rate.GCArgs{}.InsertOpts().Queue)
	assert.Len(t, rate.RegisterJobs(river.NewWorkers(), pool), 1)
}
