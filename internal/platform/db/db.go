// Package db opens Postgres pools, applies migrations and serializes
// one-off work across replicas with advisory locks.
package db

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pressly/goose/v3"
)

// BootstrapLock is the advisory lock key held while bootstrap runs.
// "WERBOT" + 1; other project advisory keys share the prefix with the next number.
const BootstrapLock int64 = 0x574552424f540001

// Open connects a pool and pings it. statement_timeout defaults to 30s
// unless the URL sets it; pool size comes from pool_max_conns in the URL.
func Open(ctx context.Context, url string) (*pgxpool.Pool, error) {
	return open(ctx, url, func(cfg *pgxpool.Config) {
		if _, ok := cfg.ConnConfig.RuntimeParams["statement_timeout"]; !ok {
			cfg.ConnConfig.RuntimeParams["statement_timeout"] = "30s"
		}
	})
}

// OpenBootstrap is Open for bootstrap only: statement_timeout is always 0
// and MaxConns is always 2, overriding the URL. Close it after bootstrap.
func OpenBootstrap(ctx context.Context, url string) (*pgxpool.Pool, error) {
	return open(ctx, url, func(cfg *pgxpool.Config) {
		cfg.ConnConfig.RuntimeParams["statement_timeout"] = "0"
		cfg.MaxConns = 2
	})
}

func open(ctx context.Context, url string, tune func(*pgxpool.Config)) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, fmt.Errorf("parse database url: %w", err)
	}
	tune(cfg)
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("create pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}
	return pool, nil
}

// Migrate applies all pending goose migrations from fsys (table goose_db_version).
func Migrate(ctx context.Context, sqlDB *sql.DB, fsys fs.FS) error {
	p, err := goose.NewProvider(goose.DialectPostgres, sqlDB, fsys)
	if err != nil {
		return fmt.Errorf("goose provider: %w", err)
	}
	if _, err := p.Up(ctx); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}
	return nil
}

// WithLock runs fn while holding the session advisory lock key on one pooled
// connection and returns fn's error. Waiting for the connection and the lock
// is bounded only by ctx when the pool has no statement_timeout (use
// OpenBootstrap); on an Open pool the server aborts the lock wait after
// statement_timeout with SQLSTATE 57014. The lock is always released: unlock
// runs with context.WithoutCancel(ctx), and if locking or unlocking fails the
// connection is closed before Release, so the lock dies with the session.
func WithLock(ctx context.Context, pool *pgxpool.Pool, key int64, fn func(context.Context) error) error {
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("acquire connection: %w", err)
	}
	defer conn.Release()
	if _, err := conn.Exec(ctx, "SELECT pg_advisory_lock($1)", key); err != nil {
		_ = conn.Conn().Close(context.WithoutCancel(ctx))
		return fmt.Errorf("advisory lock: %w", err)
	}
	defer func() {
		uctx := context.WithoutCancel(ctx)
		if _, err := conn.Exec(uctx, "SELECT pg_advisory_unlock($1)", key); err != nil {
			_ = conn.Conn().Close(uctx)
		}
	}()
	return fn(ctx)
}
