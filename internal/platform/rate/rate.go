// Package rate counts requests per key in fixed windows in Postgres, so a
// limit holds across every replica on one database.
package rate

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/werbot/werbot/internal/platform/dbx"
)

// GCOlderThan is the age after which rate_limit.gc deletes a window.
const GCOlderThan = 2 * time.Hour

// allowSQL counts one call of key in the window of now() and returns the
// window's count. $2 is the window length in seconds.
const allowSQL = `INSERT INTO rate_limit (key, window_start, count)
VALUES ($1, to_timestamp(floor(extract(epoch FROM now())::float8 / $2::float8) * $2::float8), 1)
ON CONFLICT (key, window_start) DO UPDATE SET count = rate_limit.count + 1
RETURNING count`

// Allow counts one call of key and reports whether it is within limit calls
// per window. It commits in its own transaction, so call it before and
// outside the caller's effect transaction: the count then survives a rollback.
// An error always comes with false (fail-closed).
func Allow(ctx context.Context, pool *pgxpool.Pool, key string, limit int, window time.Duration) (bool, error) {
	var n int
	err := dbx.System(ctx, pool, dbx.ReasonRate, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, allowSQL, key, window.Seconds()).Scan(&n)
	})
	if err != nil {
		return false, err
	}
	return n <= limit, nil
}

// GC deletes the windows that started more than olderThan ago and returns
// how many it deleted.
func GC(ctx context.Context, pool *pgxpool.Pool, olderThan time.Duration) (int64, error) {
	var n int64
	err := dbx.System(ctx, pool, dbx.ReasonRate, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, "DELETE FROM rate_limit WHERE window_start < now() - make_interval(secs => $1)", olderThan.Seconds())
		n = tag.RowsAffected()
		return err
	})
	return n, err
}
