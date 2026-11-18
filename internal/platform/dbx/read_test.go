package dbx_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/platform/dbx"
)

// Batch tracing: every query of a batch goes to the log with a "batch:" prefix.
func (l *sqlLog) TraceBatchStart(ctx context.Context, _ *pgx.Conn, _ pgx.TraceBatchStartData) context.Context {
	return ctx
}

func (l *sqlLog) TraceBatchQuery(_ context.Context, _ *pgx.Conn, d pgx.TraceBatchQueryData) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.sql = append(l.sql, "batch:"+d.SQL)
}

func (*sqlLog) TraceBatchEnd(context.Context, *pgx.Conn, pgx.TraceBatchEndData) {}

func TestReadInOrgOneRoundTrip(t *testing.T) {
	pool, log := tracedPool(t)
	ctx := context.Background()
	org, user := uuid.New(), uuid.New()
	log.reset()

	var o, u string
	require.NoError(t, dbx.ReadInOrg(pool, org, user, func(r *dbx.Read) error {
		return r.QueryRow(ctx, "SELECT current_setting('werbot.org_id'), current_setting('werbot.user_id')").Scan(&o, &u)
	}))
	assert.Equal(t, org.String(), o)
	assert.Equal(t, user.String(), u)
	sent := log.reset()
	require.Len(t, sent, 4, "one batch: BEGIN, the context, the statement, COMMIT: %q", sent)
	assert.Equal(t, "batch:BEGIN", sent[0])
	assert.Contains(t, sent[1], "set_config('werbot.org_id', $1, true)")
	assert.Equal(t, "batch:COMMIT", sent[3])

	// committed and is_local: the next transaction on the one connection sees nothing.
	require.NoError(t, pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
		v, err := setting(ctx, tx, "werbot.org_id")
		assert.Empty(t, v)
		return err
	}))
}

// backendPID is the server process of the one connection of a tracedPool.
func backendPID(t *testing.T, pool *pgxpool.Pool) (pid int) {
	t.Helper()
	require.NoError(t, pool.QueryRow(context.Background(), "SELECT pg_backend_pid()").Scan(&pid))
	return pid
}

func TestReadInOrgErrorRollsBack(t *testing.T) {
	pool, _ := tracedPool(t)
	ctx := context.Background()
	pid := backendPID(t, pool)
	err := dbx.ReadInOrg(pool, uuid.New(), uuid.Nil, func(r *dbx.Read) error {
		var n int
		return r.QueryRow(ctx, `WITH o AS (INSERT INTO organization (name) VALUES ('o') RETURNING id)
			SELECT 1 / (count(*) - 1) FROM o`).Scan(&n)
	})
	var pgErr *pgconn.PgError
	require.ErrorAs(t, err, &pgErr)
	assert.Equal(t, "22012", pgErr.Code, "division by zero")

	var n int
	require.NoError(t, pool.QueryRow(ctx, "SELECT count(*) FROM organization").Scan(&n))
	assert.Zero(t, n, "the insert of the failed statement is rolled back")
	assert.Equal(t, pid, backendPID(t, pool), "ROLLBACK keeps the connection: the pool does not replace it")
}

// A panic in fn still returns the connection to the pool.
func TestReadInOrgPanicReleases(t *testing.T) {
	pool, _ := tracedPool(t)
	ctx := context.Background()
	func() {
		defer func() { require.Equal(t, "boom", recover()) }()
		_ = dbx.ReadInOrg(pool, uuid.New(), uuid.Nil, func(r *dbx.Read) error {
			_ = r.QueryRow(ctx, "SELECT 1")
			panic("boom")
		})
	}()
	assert.Zero(t, pool.Stat().AcquiredConns())
	qctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	require.NoError(t, pool.QueryRow(qctx, "SELECT 1").Scan(new(int)), "the one connection is usable")
}

func TestReadInOrgNoRows(t *testing.T) {
	pool, _ := tracedPool(t)
	ctx := context.Background()
	err := dbx.ReadInOrg(pool, uuid.New(), uuid.Nil, func(r *dbx.Read) error {
		var n int
		return r.QueryRow(ctx, "SELECT 1 WHERE false").Scan(&n)
	})
	require.ErrorIs(t, err, pgx.ErrNoRows)
	require.NoError(t, pool.QueryRow(ctx, "SELECT 1").Scan(new(int)), "the connection is usable again")
}

func TestReadInOrgOneStatement(t *testing.T) {
	pool, _ := tracedPool(t)
	ctx := context.Background()
	require.ErrorIs(t, dbx.ReadInOrg(pool, uuid.Nil, uuid.Nil, func(*dbx.Read) error { return nil }), dbx.ErrNoOrg)
	err := dbx.ReadInOrg(pool, uuid.New(), uuid.Nil, func(r *dbx.Read) error {
		if err := r.QueryRow(ctx, "SELECT 1").Scan(new(int)); err != nil {
			return err
		}
		return r.QueryRow(ctx, "SELECT 2").Scan(new(int))
	})
	require.Error(t, err)
	_, err = (&dbx.Read{}).Exec(ctx, "SELECT 1")
	assert.True(t, err != nil && !errors.Is(err, pgx.ErrNoRows))
}
