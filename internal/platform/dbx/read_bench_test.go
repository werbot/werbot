package dbx_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/werbot/werbot/internal/platform/db/dbtest"
	"github.com/werbot/werbot/internal/platform/dbx"
)

// BenchmarkOneRead compares a one-statement read in InOrg (three round trips)
// with ReadInOrg (one).
func BenchmarkOneRead(b *testing.B) {
	_, url := dbtest.New(b)
	pool, err := pgxpool.New(context.Background(), url)
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(pool.Close)
	ctx, org := context.Background(), uuid.New()
	const q = "SELECT current_setting('werbot.org_id')"
	var v string
	b.Run("InOrg", func(b *testing.B) {
		for b.Loop() {
			if err := dbx.InOrg(ctx, pool, org, uuid.Nil, func(tx pgx.Tx) error { return tx.QueryRow(ctx, q).Scan(&v) }); err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("ReadInOrg", func(b *testing.B) {
		for b.Loop() {
			if err := dbx.ReadInOrg(pool, org, uuid.Nil, func(r *dbx.Read) error { return r.QueryRow(ctx, q).Scan(&v) }); err != nil {
				b.Fatal(err)
			}
		}
	})
}
