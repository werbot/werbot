package dbx_test

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/platform/db/dbtest"
	"github.com/werbot/werbot/internal/platform/dbx"
)

// sqlLog records every statement the pool sends.
type sqlLog struct {
	mu  sync.Mutex
	sql []string
}

func (l *sqlLog) TraceQueryStart(ctx context.Context, _ *pgx.Conn, d pgx.TraceQueryStartData) context.Context {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.sql = append(l.sql, d.SQL)
	return ctx
}

func (*sqlLog) TraceQueryEnd(context.Context, *pgx.Conn, pgx.TraceQueryEndData) {}

func (l *sqlLog) reset() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	out := l.sql
	l.sql = nil
	return out
}

// tracedPool opens a one-connection pool on the test database, so every
// transaction runs on the same server session.
func tracedPool(t *testing.T) (*pgxpool.Pool, *sqlLog) {
	t.Helper()
	_, url := dbtest.New(t)
	cfg, err := pgxpool.ParseConfig(url)
	require.NoError(t, err)
	cfg.MaxConns = 1
	log := &sqlLog{}
	cfg.ConnConfig.Tracer = log
	pool, err := pgxpool.NewWithConfig(context.Background(), cfg)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	return pool, log
}

func TestInOrgSetsContextInBegin(t *testing.T) {
	pool, log := tracedPool(t)
	ctx := context.Background()
	org, user := uuid.New(), uuid.New()
	log.reset()

	require.NoError(t, dbx.InOrg(ctx, pool, org, user, func(tx pgx.Tx) error {
		sent := log.reset()
		require.Len(t, sent, 1, "BEGIN and set_config go in one round trip: %q", sent)
		assert.True(t, strings.HasPrefix(sent[0], "BEGIN;"), sent[0])
		assert.Contains(t, sent[0], "set_config('werbot.org_id', '"+org.String()+"', true)")
		assert.Contains(t, sent[0], "set_config('werbot.user_id', '"+user.String()+"', true)")
		o, err := setting(ctx, tx, "werbot.org_id")
		require.NoError(t, err)
		u, err := setting(ctx, tx, "werbot.user_id")
		require.NoError(t, err)
		assert.Equal(t, org.String(), o)
		assert.Equal(t, user.String(), u)
		return nil
	}))

	// is_local: the next transaction on the same connection sees nothing.
	require.NoError(t, pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
		o, err := setting(ctx, tx, "werbot.org_id")
		require.NoError(t, err)
		u, err := setting(ctx, tx, "werbot.user_id")
		require.NoError(t, err)
		assert.Empty(t, o)
		assert.Empty(t, u)
		return nil
	}))
}

func TestInOrgRollsBackOnError(t *testing.T) {
	pool, _ := tracedPool(t)
	ctx := context.Background()
	boom := errors.New("boom")
	err := dbx.InOrg(ctx, pool, uuid.New(), uuid.Nil, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, "INSERT INTO organization (name) VALUES ('o')")
		require.NoError(t, err)
		return boom
	})
	require.ErrorIs(t, err, boom)
	var n int
	require.NoError(t, pool.QueryRow(ctx, "SELECT count(*) FROM organization").Scan(&n))
	assert.Zero(t, n)
}
