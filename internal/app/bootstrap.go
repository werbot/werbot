package app

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/stdlib"

	"github.com/werbot/werbot/internal/platform/db"
	"github.com/werbot/werbot/migrations"
)

// bootstrap is the only place for one-off startup work (contract §1.6). It runs
// the steps under db.BootstrapLock, so replicas starting together take turns:
// waiting for the lock is bounded only by ctx, and a second replica sees the
// finished schema instead of failing. Later stages append steps to the closure
// in this order: (1) goose Up, (2) jobs.Migrate, (3) tlsca.LoadOrCreate,
// (4) session.LoadSigner.
func bootstrap(ctx context.Context, databaseURL string) error {
	octx, cancel := context.WithTimeout(ctx, 10*time.Second)
	bpool, err := db.OpenBootstrap(octx, databaseURL)
	cancel()
	if err != nil {
		return fmt.Errorf("bootstrap: %w", err)
	}
	defer bpool.Close()
	return db.WithLock(ctx, bpool, db.BootstrapLock, func(ctx context.Context) error {
		// (1) goose Up. sqlDB is closed before the closure returns, otherwise
		// bpool.Close waits for its connection.
		sqlDB := stdlib.OpenDBFromPool(bpool)
		err := db.Migrate(ctx, sqlDB, migrations.FS)
		if cerr := sqlDB.Close(); err == nil {
			err = cerr
		}
		if err != nil {
			return fmt.Errorf("bootstrap: %w", err)
		}
		return nil
	})
}
