package inventory

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"

	"github.com/werbot/werbot/internal/platform/dbx"
	"github.com/werbot/werbot/internal/platform/jobs"
)

// GC deletes instances whose certificate expired over 24h ago and join tokens
// expired over 7 days ago, across all organizations.
func (g *Gateways) GC(ctx context.Context) (instances, tokens int64, err error) {
	err = dbx.System(ctx, g.pool, dbx.ReasonMaintenance, func(tx pgx.Tx) error {
		st := newStore(tx)
		if instances, err = st.DeleteExpiredInstances(ctx); err != nil {
			return err
		}
		tokens, err = st.DeleteExpiredJoinTokens(ctx)
		return err
	})
	if err != nil {
		return 0, 0, err
	}
	return instances, tokens, nil
}

// GCArgs is the hourly gateway.gc job.
type GCArgs struct{}

// Kind implements river.JobArgs.
func (GCArgs) Kind() string { return "gateway.gc" }

// InsertOpts runs the job in the maintenance queue.
func (GCArgs) InsertOpts() river.InsertOpts { return river.InsertOpts{Queue: jobs.QueueMaintenance} }

type gcWorker struct {
	river.WorkerDefaults[GCArgs]
	g *Gateways
}

func (w *gcWorker) Work(ctx context.Context, _ *river.Job[GCArgs]) error {
	_, _, err := w.g.GC(ctx)
	return err
}

// RegisterGatewayJobs adds the gateway.gc worker and returns its hourly
// periodic job for jobs.NewClient.
func RegisterGatewayJobs(workers *river.Workers, g *Gateways) []*river.PeriodicJob {
	river.AddWorker(workers, &gcWorker{g: g})
	return []*river.PeriodicJob{
		river.NewPeriodicJob(river.PeriodicInterval(time.Hour),
			func() (river.JobArgs, *river.InsertOpts) { return GCArgs{}, nil },
			&river.PeriodicJobOpts{ID: "gateway.gc"}),
	}
}
