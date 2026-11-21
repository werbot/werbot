package access

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"

	"github.com/werbot/werbot/internal/platform/dbx"
	"github.com/werbot/werbot/internal/platform/jobs"
)

// Expire moves every pending approval past its deadline to expired, in all
// organizations, and returns how many it moved. A pending approval never
// becomes approved by time. The guarded UPDATE lets one of concurrent runs
// (several replicas) or of a run and Decide win each row. Grants need no work
// here: they carry their own expires_at. A command approval sends no NOTIFY:
// its gateway refuses on its own timeout.
func (s *Service) Expire(ctx context.Context) (int64, error) {
	var n int64
	err := dbx.System(ctx, s.pool, dbx.ReasonMaintenance, func(tx pgx.Tx) error {
		rows, err := newStore(tx).ExpireApprovals(ctx)
		if err != nil {
			return err
		}
		for _, r := range rows {
			if err := auditAs(ctx, tx, r.OrgID, "system", nil, "approval.expire", "approval", r.ID, map[string]any{"kind": r.Kind}); err != nil {
				return err
			}
			if ApprovalKind(r.Kind) != KindAccess {
				continue
			}
			var sel map[string]string
			if r.LabelSelector != nil {
				if err := json.Unmarshal(r.LabelSelector, &sel); err != nil {
					return fmt.Errorf("access: selector of approval %s: %w", r.ID, err)
				}
			}
			resource := r.ResourceName
			if resource == "" {
				resource = selectorText(sel)
			}
			if err := s.emails.EnqueueTx(ctx, tx, r.RequesterEmail, "access_decision", map[string]string{
				"url": s.RequestURL(r.ID), "resource": resource, "status": StatusExpired, "comment": ""}); err != nil {
				return err
			}
		}
		n = int64(len(rows))
		return nil
	})
	return n, err
}

// ExpireArgs is the approval.expire job.
type ExpireArgs struct{}

// Kind implements river.JobArgs.
func (ExpireArgs) Kind() string { return "approval.expire" }

// InsertOpts runs the job in the maintenance queue.
func (ExpireArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{Queue: jobs.QueueMaintenance}
}

type expireWorker struct {
	river.WorkerDefaults[ExpireArgs]
	s *Service
}

func (w *expireWorker) Work(ctx context.Context, _ *river.Job[ExpireArgs]) error {
	_, err := w.s.Expire(ctx)
	return err
}

// RegisterApprovalJobs adds the approval.expire worker and returns its
// periodic job (every minute, run by the River leader) for jobs.NewClient.
func RegisterApprovalJobs(workers *river.Workers, s *Service) []*river.PeriodicJob {
	river.AddWorker(workers, &expireWorker{s: s})
	return []*river.PeriodicJob{
		river.NewPeriodicJob(river.PeriodicInterval(time.Minute),
			func() (river.JobArgs, *river.InsertOpts) { return ExpireArgs{}, nil },
			&river.PeriodicJobOpts{ID: "approval.expire"}),
	}
}
