package rate

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"

	"github.com/werbot/werbot/internal/platform/jobs"
)

// GCArgs is the rate_limit.gc job.
type GCArgs struct{}

// Kind implements river.JobArgs.
func (GCArgs) Kind() string { return "rate_limit.gc" }

// InsertOpts runs the job in the maintenance queue.
func (GCArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{Queue: jobs.QueueMaintenance}
}

type gcWorker struct {
	river.WorkerDefaults[GCArgs]
	pool *pgxpool.Pool
}

func (w *gcWorker) Work(ctx context.Context, _ *river.Job[GCArgs]) error {
	_, err := GC(ctx, w.pool, GCOlderThan)
	return err
}

// RegisterJobs adds the rate_limit.gc worker and returns its periodic job
// (hourly, run by the River leader) for jobs.NewClient.
func RegisterJobs(workers *river.Workers, pool *pgxpool.Pool) []*river.PeriodicJob {
	river.AddWorker(workers, &gcWorker{pool: pool})
	return []*river.PeriodicJob{
		river.NewPeriodicJob(river.PeriodicInterval(time.Hour),
			func() (river.JobArgs, *river.InsertOpts) { return GCArgs{}, nil },
			&river.PeriodicJobOpts{ID: "rate_limit.gc"}),
	}
}
