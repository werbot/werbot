package relay

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"

	"github.com/werbot/werbot/internal/platform/dbx"
	"github.com/werbot/werbot/internal/platform/jobs"
)

// GC deletes server_instance rows without a heartbeat for InstanceGCAfter
// and returns how many it deleted.
func (r *Relay) GC(ctx context.Context) (int64, error) {
	var n int64
	err := dbx.System(ctx, r.pool, dbx.ReasonMaintenance, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, "DELETE FROM server_instance WHERE last_seen_at < now() - make_interval(secs => $1)",
			InstanceGCAfter.Seconds())
		n = tag.RowsAffected()
		return err
	})
	return n, err
}

// GCArgs is the server_instance.gc job.
type GCArgs struct{}

// Kind implements river.JobArgs.
func (GCArgs) Kind() string { return "server_instance.gc" }

// InsertOpts runs the job in the maintenance queue.
func (GCArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{Queue: jobs.QueueMaintenance}
}

type gcWorker struct {
	river.WorkerDefaults[GCArgs]
	r *Relay
}

func (w *gcWorker) Work(ctx context.Context, _ *river.Job[GCArgs]) error {
	_, err := w.r.GC(ctx)
	return err
}

// RegisterJobs adds the server_instance.gc worker and returns its periodic
// job (every GCInterval, run by the River leader) for jobs.NewClient.
func RegisterJobs(workers *river.Workers, r *Relay) []*river.PeriodicJob {
	river.AddWorker(workers, &gcWorker{r: r})
	return []*river.PeriodicJob{
		river.NewPeriodicJob(river.PeriodicInterval(GCInterval),
			func() (river.JobArgs, *river.InsertOpts) { return GCArgs{}, nil },
			&river.PeriodicJobOpts{ID: "server_instance.gc"}),
	}
}
