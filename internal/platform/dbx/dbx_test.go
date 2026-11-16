package dbx_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/platform/apperr"
	"github.com/werbot/werbot/internal/platform/db"
	"github.com/werbot/werbot/internal/platform/db/dbtest"
	"github.com/werbot/werbot/internal/platform/dbx"
)

func setting(ctx context.Context, q interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, name string) (string, error) {
	var v string
	err := q.QueryRow(ctx, "SELECT coalesce(current_setting($1, true), '')", name).Scan(&v)
	return v, err
}

func TestSystemTx(t *testing.T) {
	pool, _ := dbtest.New(t)
	ctx := context.Background()
	count := func() int {
		var n int
		require.NoError(t, pool.QueryRow(ctx, "SELECT count(*) FROM organization").Scan(&n))
		return n
	}
	insert := func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, "INSERT INTO organization (name) VALUES ('o')")
		return err
	}
	require.NoError(t, dbx.System(ctx, pool, dbx.ReasonBootstrap, func(tx pgx.Tx) error {
		v, err := setting(ctx, tx, "werbot.org_id")
		require.NoError(t, err)
		assert.Empty(t, v)
		return insert(tx)
	}))
	assert.Equal(t, 1, count())

	boom := errors.New("boom")
	err := dbx.System(ctx, pool, dbx.ReasonBootstrap, func(tx pgx.Tx) error {
		require.NoError(t, insert(tx))
		return boom
	})
	assert.ErrorIs(t, err, boom)
	assert.Equal(t, 1, count())
}

func TestInOrgSetsContext(t *testing.T) {
	pool, _ := dbtest.New(t)
	ctx := context.Background()
	org, user := uuid.New(), uuid.New()
	require.NoError(t, dbx.InOrg(ctx, pool, org, user, func(tx pgx.Tx) error {
		o, err := setting(ctx, tx, "werbot.org_id")
		require.NoError(t, err)
		u, err := setting(ctx, tx, "werbot.user_id")
		require.NoError(t, err)
		assert.Equal(t, org.String(), o)
		assert.Equal(t, user.String(), u)
		return nil
	}))
	// transaction-local: nothing leaks to the next user of the connection
	conn, err := pool.Acquire(ctx)
	require.NoError(t, err)
	defer conn.Release()
	o, err := setting(ctx, conn, "werbot.org_id")
	require.NoError(t, err)
	assert.Empty(t, o)

	called := false
	err = dbx.InOrg(ctx, pool, uuid.Nil, user, func(pgx.Tx) error { called = true; return nil })
	assert.ErrorIs(t, err, dbx.ErrNoOrg)
	assert.False(t, called)
}

func TestSystemRequiresReason(t *testing.T) {
	pool, _ := dbtest.New(t)
	called := false
	err := dbx.System(context.Background(), pool, "", func(pgx.Tx) error { called = true; return nil })
	assert.ErrorContains(t, err, "dbx: system reason required")
	assert.False(t, called)
}

func TestTxPoolExhaustedIsUnavailable(t *testing.T) {
	_, url := dbtest.New(t)
	ctx := context.Background()
	pool, err := db.Open(ctx, url+"&pool_max_conns=1")
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	c, err := pool.Acquire(ctx)
	require.NoError(t, err)
	t.Cleanup(c.Release)

	ctx2, cancel := context.WithTimeout(ctx, 200*time.Millisecond)
	defer cancel()
	start := time.Now()
	err = dbx.System(ctx2, pool, dbx.ReasonBootstrap, func(pgx.Tx) error { return nil })
	assert.Equal(t, connect.CodeUnavailable, connect.CodeOf(apperr.ToConnect(err)))
	assert.Less(t, time.Since(start), time.Second)
}
