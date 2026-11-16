package jobs_test

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/platform/db/dbtest"
	"github.com/werbot/werbot/internal/platform/jobs"
)

type pingArgs struct {
	Msg string `json:"msg"`
}

func (pingArgs) Kind() string { return "test_ping" }

type pingWorker struct {
	river.WorkerDefaults[pingArgs]
	done chan string
}

func (w *pingWorker) Work(_ context.Context, j *river.Job[pingArgs]) error {
	w.done <- j.Args.Msg
	return nil
}

type blockArgs struct{}

func (blockArgs) Kind() string                 { return "test_block" }
func (blockArgs) InsertOpts() river.InsertOpts { return river.InsertOpts{Queue: jobs.QueueEmail} }

type blockWorker struct {
	river.WorkerDefaults[blockArgs]
	started, release chan struct{}
}

func (w *blockWorker) Work(ctx context.Context, _ *river.Job[blockArgs]) error {
	w.started <- struct{}{}
	select {
	case <-w.release:
	case <-ctx.Done():
	}
	return nil
}

func TestMigrateIdempotent(t *testing.T) {
	pool, _ := dbtest.New(t)
	ctx := context.Background()
	require.NoError(t, jobs.Migrate(ctx, pool))
	var n int
	require.NoError(t, pool.QueryRow(ctx, "SELECT count(*) FROM river_job").Scan(&n))
	assert.Equal(t, 0, n)
}

func TestClientRunsJob(t *testing.T) {
	pool, _ := dbtest.New(t)
	ctx := context.Background()
	ping := &pingWorker{done: make(chan string, 1)}
	workers := river.NewWorkers()
	river.AddWorker(workers, ping)
	client, err := jobs.NewClient(pool, workers, nil, slog.New(slog.DiscardHandler))
	require.NoError(t, err)
	require.NoError(t, client.Start(ctx))
	t.Cleanup(func() { _ = client.Stop(ctx) })

	_, err = client.Insert(ctx, pingArgs{Msg: "hi"}, nil)
	require.NoError(t, err)
	select {
	case msg := <-ping.done:
		assert.Equal(t, "hi", msg)
	case <-time.After(5 * time.Second):
		t.Fatal("job did not run")
	}
}

func TestEmailQueueDoesNotStarveDefault(t *testing.T) {
	pool, _ := dbtest.New(t)
	ctx := context.Background()
	ping := &pingWorker{done: make(chan string, 1)}
	block := &blockWorker{started: make(chan struct{}, 3), release: make(chan struct{})}
	workers := river.NewWorkers()
	river.AddWorker(workers, ping)
	river.AddWorker(workers, block)
	client, err := jobs.NewClient(pool, workers, nil, slog.New(slog.DiscardHandler))
	require.NoError(t, err)
	require.NoError(t, client.Start(ctx))
	t.Cleanup(func() { _ = client.Stop(ctx) })
	t.Cleanup(func() { close(block.release) }) // LIFO: workers are released before Stop

	for range 3 {
		_, err = client.Insert(ctx, blockArgs{}, nil)
		require.NoError(t, err)
	}
	for range 2 {
		select {
		case <-block.started:
		case <-time.After(5 * time.Second):
			t.Fatal("email worker did not start")
		}
	}
	select {
	case <-block.started:
		t.Fatal("third email job started: email MaxWorkers must be 2")
	case <-time.After(time.Second):
	}

	_, err = client.Insert(ctx, pingArgs{Msg: "hi"}, nil)
	require.NoError(t, err)
	select {
	case msg := <-ping.done:
		assert.Equal(t, "hi", msg)
	case <-time.After(5 * time.Second):
		t.Fatal("default job starved by email queue")
	}

	// The ping job may still be finishing: wait until only email jobs run.
	running := func() map[string]int {
		rows, err := pool.Query(ctx, "SELECT queue, count(*) FROM river_job WHERE state = 'running' GROUP BY queue")
		require.NoError(t, err)
		got, err := pgx.CollectRows(rows, pgx.RowToMap)
		require.NoError(t, err)
		m := map[string]int{}
		for _, r := range got {
			m[r["queue"].(string)] = int(r["count"].(int64))
		}
		return m
	}
	assert.Eventually(t, func() bool { return len(running()) == 1 }, 5*time.Second, 50*time.Millisecond)
	assert.Equal(t, map[string]int{"email": 2}, running())
}

func TestFeedsQueueRunsJobs(t *testing.T) { assertQueueRunsJobs(t, jobs.QueueFeeds) }

func TestMaintenanceQueueRunsJobs(t *testing.T) { assertQueueRunsJobs(t, jobs.QueueMaintenance) }

func assertQueueRunsJobs(t *testing.T, queue string) {
	t.Helper()
	pool, _ := dbtest.New(t)
	ctx := context.Background()
	ping := &pingWorker{done: make(chan string, 1)}
	workers := river.NewWorkers()
	river.AddWorker(workers, ping)
	client, err := jobs.NewClient(pool, workers, nil, slog.New(slog.DiscardHandler))
	require.NoError(t, err)
	require.NoError(t, client.Start(ctx))
	t.Cleanup(func() { _ = client.Stop(ctx) })

	_, err = client.Insert(ctx, pingArgs{Msg: queue}, &river.InsertOpts{Queue: queue})
	require.NoError(t, err)
	select {
	case msg := <-ping.done:
		assert.Equal(t, queue, msg)
	case <-time.After(5 * time.Second):
		t.Fatalf("%s job did not run", queue)
	}
}
