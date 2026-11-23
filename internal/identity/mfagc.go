package identity

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"

	"github.com/werbot/werbot/internal/platform/dbx"
	"github.com/werbot/werbot/internal/platform/jobs"
)

// MFAGCOlderThan is the age after which mfa.gc deletes MFA challenges and
// verifications; no MFA rule accepts a verification older than 24h.
const MFAGCOlderThan = 24 * time.Hour

// MFAGC deletes MFA challenges created and verifications passed more than
// olderThan ago and returns how many rows it deleted.
func MFAGC(ctx context.Context, pool *pgxpool.Pool, olderThan time.Duration) (n int64, err error) {
	err = dbx.System(ctx, pool, dbx.ReasonMaintenance, func(tx pgx.Tx) error {
		st := newStore(tx)
		c, err := st.DeleteOldMFAChallenges(ctx, olderThan.Seconds())
		if err != nil {
			return err
		}
		v, err := st.DeleteOldMFAVerifications(ctx, olderThan.Seconds())
		n = c + v
		return err
	})
	if err != nil {
		return 0, fmt.Errorf("identity: mfa gc: %w", err)
	}
	return n, nil
}

// MFAGCArgs is the mfa.gc job.
type MFAGCArgs struct{}

// Kind implements river.JobArgs.
func (MFAGCArgs) Kind() string { return "mfa.gc" }

// InsertOpts runs the job in the maintenance queue.
func (MFAGCArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{Queue: jobs.QueueMaintenance}
}

type mfaGCWorker struct {
	river.WorkerDefaults[MFAGCArgs]
	pool *pgxpool.Pool
}

func (w *mfaGCWorker) Work(ctx context.Context, _ *river.Job[MFAGCArgs]) error {
	_, err := MFAGC(ctx, w.pool, MFAGCOlderThan)
	return err
}

// RegisterMFAJobs adds the mfa.gc worker and returns its periodic job
// (hourly, run by the River leader) for jobs.NewClient.
func RegisterMFAJobs(workers *river.Workers, pool *pgxpool.Pool) []*river.PeriodicJob {
	river.AddWorker(workers, &mfaGCWorker{pool: pool})
	return []*river.PeriodicJob{
		river.NewPeriodicJob(river.PeriodicInterval(time.Hour),
			func() (river.JobArgs, *river.InsertOpts) { return MFAGCArgs{}, nil },
			&river.PeriodicJobOpts{ID: "mfa.gc"}),
	}
}
