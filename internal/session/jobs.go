package session

import (
	"context"
	"time"

	"github.com/riverqueue/river"

	"github.com/werbot/werbot/internal/platform/jobs"
)

// EndLostArgs is the session.end_lost job.
type EndLostArgs struct{}

// Kind implements river.JobArgs.
func (EndLostArgs) Kind() string { return "session.end_lost" }

// InsertOpts runs the job in the maintenance queue.
func (EndLostArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{Queue: jobs.QueueMaintenance}
}

var endLostSchedule = river.PeriodicInterval(EndLostInterval)

type endLostWorker struct {
	river.WorkerDefaults[EndLostArgs]
	s       *Service
	started time.Time
}

func (w *endLostWorker) Work(ctx context.Context, _ *river.Job[EndLostArgs]) error {
	if time.Since(w.started) < EndLostStartupGrace {
		return nil
	}
	_, err := w.s.EndLost(ctx, EndLostAfter)
	return err
}

// StreamTicketGCArgs is the stream_ticket.gc job.
type StreamTicketGCArgs struct{}

// Kind implements river.JobArgs.
func (StreamTicketGCArgs) Kind() string { return "stream_ticket.gc" }

// InsertOpts runs the job in the maintenance queue.
func (StreamTicketGCArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{Queue: jobs.QueueMaintenance}
}

type ticketGCWorker struct {
	river.WorkerDefaults[StreamTicketGCArgs]
	s *Service
}

func (w *ticketGCWorker) Work(ctx context.Context, _ *river.Job[StreamTicketGCArgs]) error {
	_, err := w.s.GCStreamTickets(ctx)
	return err
}

// RegisterJobs adds the session.end_lost and stream_ticket.gc workers and
// returns their periodic jobs for jobs.NewClient. started is the process
// start: after a restart the instances need EndLostStartupGrace to heartbeat
// before sessions can be lost.
func RegisterJobs(workers *river.Workers, s *Service, started time.Time) []*river.PeriodicJob {
	river.AddWorker(workers, &endLostWorker{s: s, started: started})
	river.AddWorker(workers, &ticketGCWorker{s: s})
	return []*river.PeriodicJob{
		river.NewPeriodicJob(endLostSchedule,
			func() (river.JobArgs, *river.InsertOpts) { return EndLostArgs{}, nil },
			&river.PeriodicJobOpts{ID: "session.end_lost"}),
		river.NewPeriodicJob(river.PeriodicInterval(StreamTicketGCInterval),
			func() (river.JobArgs, *river.InsertOpts) { return StreamTicketGCArgs{}, nil },
			&river.PeriodicJobOpts{ID: "stream_ticket.gc"}),
	}
}
