// Package dbtest gives tests a fresh, migrated Postgres database.
package dbtest

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"io/fs"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib" // registers the "pgx" database/sql driver for pgtestdb
	"github.com/peterldowns/pgtestdb"

	"github.com/werbot/werbot/internal/platform/db"
	"github.com/werbot/werbot/internal/platform/jobs"
	"github.com/werbot/werbot/migrations"
)

// New creates a database from a pgtestdb template with migrations.FS applied
// and returns a db.Open pool (statement_timeout 30s, as in production) and its URL.
func New(t testing.TB) (pool *pgxpool.Pool, url string) {
	t.Helper()
	conf := pgtestdb.Custom(t, pgtestdb.Config{
		DriverName: "pgx",
		Host:       "localhost",
		Port:       "5433",
		User:       "postgres",
		Password:   "password",
		Options:    "sslmode=disable",
	}, migrator{})
	url = conf.URL()
	pool, err := db.Open(context.Background(), url)
	if err != nil {
		t.Fatalf("dbtest: open %s: %v", conf.Database, err)
	}
	t.Cleanup(pool.Close)
	return pool, url
}

type migrator struct{}

// Hash covers names and contents of every migration and the River schema
// version, so any change rebuilds the template.
func (migrator) Hash() (string, error) {
	entries, err := fs.ReadDir(migrations.FS, ".")
	if err != nil {
		return "", err
	}
	h := sha256.New()
	for _, e := range entries {
		data, err := fs.ReadFile(migrations.FS, e.Name())
		if err != nil {
			return "", err
		}
		_, _ = fmt.Fprintf(h, "%s\x00%d\x00", e.Name(), len(data))
		_, _ = h.Write(data)
	}
	_, _ = fmt.Fprintf(h, "river:%d", jobs.SchemaVersion())
	return hex.EncodeToString(h.Sum(nil)), nil
}

// Migrate runs without db.WithLock: pgtestdb serializes template creation.
func (migrator) Migrate(ctx context.Context, sqlDB *sql.DB, cfg pgtestdb.Config) error {
	if err := db.Migrate(ctx, sqlDB, migrations.FS); err != nil {
		return err
	}
	pool, err := pgxpool.New(ctx, cfg.URL())
	if err != nil {
		return err
	}
	defer pool.Close()
	return jobs.Migrate(ctx, pool)
}
