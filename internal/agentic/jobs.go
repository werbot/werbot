package agentic

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"

	"github.com/werbot/werbot/internal/platform/dbx"
	"github.com/werbot/werbot/internal/platform/jobs"
)

// ExpireRuns ends every running or paused run past its expires_at, in all
// organizations, with reason "expired" (audit agent.run.end by the system),
// and returns how many it ended. The guarded UPDATE lets one of concurrent
// calls (several replicas) or of a call and EndRun win each row.
func (s *Service) ExpireRuns(ctx context.Context) (int64, error) {
	var n int64
	err := dbx.System(ctx, s.pool, dbx.ReasonMaintenance, func(tx pgx.Tx) error {
		rows, err := newStore(tx).ExpireRuns(ctx)
		if err != nil {
			return err
		}
		for _, r := range rows {
			if err := audit(ctx, tx, r.OrgID, "system", nil, "agent.run.end", "agent_run", r.ID,
				map[string]any{"reason": "expired"}); err != nil {
				return err
			}
		}
		n = int64(len(rows))
		return nil
	})
	return n, err
}

// ExpireArgs is the agent_run.expire job.
type ExpireArgs struct{}

// Kind implements river.JobArgs.
func (ExpireArgs) Kind() string { return "agent_run.expire" }

// InsertOpts runs the job in the maintenance queue.
func (ExpireArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{Queue: jobs.QueueMaintenance}
}

type expireWorker struct {
	river.WorkerDefaults[ExpireArgs]
	s *Service
}

func (w *expireWorker) Work(ctx context.Context, _ *river.Job[ExpireArgs]) error {
	_, err := w.s.ExpireRuns(ctx)
	return err
}

// RegisterJobs adds the agent_run.expire worker and returns its periodic job
// (every minute, run by the River leader) for jobs.NewClient.
func RegisterJobs(workers *river.Workers, s *Service) []*river.PeriodicJob {
	river.AddWorker(workers, &expireWorker{s: s})
	return []*river.PeriodicJob{
		river.NewPeriodicJob(river.PeriodicInterval(time.Minute),
			func() (river.JobArgs, *river.InsertOpts) { return ExpireArgs{}, nil },
			&river.PeriodicJobOpts{ID: "agent_run.expire"}),
	}
}
