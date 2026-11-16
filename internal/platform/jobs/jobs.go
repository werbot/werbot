// Package jobs owns the River job queue: schema migration, queues and the client.
package jobs

import (
	"context"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/riverqueue/river/rivermigrate"
)

const (
	// QueueDefault is River's default queue (10 workers).
	QueueDefault = river.QueueDefault
	// QueueEmail is separate so a hung SMTP server cannot occupy other queues' workers.
	QueueEmail = "email"
)

// queues is the only place for MaxWorkers.
var queues = map[string]river.QueueConfig{
	QueueDefault: {MaxWorkers: 10},
	QueueEmail:   {MaxWorkers: 2},
}

// Migrate applies the River schema. Call it only from bootstrap under
// db.BootstrapLock (and from the dbtest template): River does not lock itself.
func Migrate(ctx context.Context, pool *pgxpool.Pool) error {
	m, err := rivermigrate.New(riverpgxv5.New(pool), nil)
	if err != nil {
		return err
	}
	_, err = m.Migrate(ctx, rivermigrate.DirectionUp, nil)
	return err
}

// SchemaVersion is the latest River migration version of the linked River.
func SchemaVersion() int {
	m, err := rivermigrate.New(riverpgxv5.New(nil), nil)
	if err != nil {
		panic(err) // only a nil driver fails; riverpgxv5.New never returns one
	}
	all := m.AllVersions()
	return all[len(all)-1].Version
}

// NewClient builds a River client over pool. A job's queue comes from its
// args' InsertOpts; without it, QueueDefault.
func NewClient(pool *pgxpool.Pool, workers *river.Workers, periodic []*river.PeriodicJob, logger *slog.Logger) (*river.Client[pgx.Tx], error) {
	return river.NewClient(riverpgxv5.New(pool), &river.Config{
		Queues:       queues,
		Workers:      workers,
		PeriodicJobs: periodic,
		Logger:       logger,
		// Sealed e-mails and other done jobs go after 1h, not the 24h default.
		CompletedJobRetentionPeriod: time.Hour,
	})
}
