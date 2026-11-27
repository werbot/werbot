package dbtest_test

import (
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/platform/db/dbtest"
)

func TestRoleLogsInAndIsDropped(t *testing.T) {
	pool, url := dbtest.New(t)
	var name string
	t.Run("role", func(t *testing.T) {
		var password string
		name, password = dbtest.Role(t, "NOINHERIT")
		_, err := pool.Exec(t.Context(), "create table g (x int); grant select on g to "+name)
		require.NoError(t, err)
		cfg, err := pgx.ParseConfig(url)
		require.NoError(t, err)
		cfg.User, cfg.Password = name, password
		conn, err := pgx.ConnectConfig(t.Context(), cfg)
		require.NoError(t, err, "the role logs in with its password")
		var user string
		require.NoError(t, conn.QueryRow(t.Context(), "select current_user").Scan(&user))
		assert.Equal(t, name, user)
		require.NoError(t, conn.Close(t.Context()))
	})
	var n int
	require.NoError(t, pool.QueryRow(t.Context(), "select count(*) from pg_roles where rolname = $1", name).Scan(&n))
	assert.Zero(t, n, "the role and its privileges are dropped after its test")
}
