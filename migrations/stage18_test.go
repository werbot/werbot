package migrations_test

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/platform/db/dbtest"
	"github.com/werbot/werbot/migrations"
)

// TestMigrations18AgentReadonly: resource.agent_readonly_status defaults to
// none and accepts none, pending, ok, has_write and error only;
// agent_readonly_detail defaults to empty; 00280 goes down to 00262 and up again.
func TestMigrations18AgentReadonly(t *testing.T) {
	pool, _ := dbtest.New(t)
	ctx := context.Background()
	f := dbtest.Seed(t, pool)

	var status, detail string
	require.NoError(t, pool.QueryRow(ctx, "SELECT agent_readonly_status, agent_readonly_detail FROM resource WHERE id = $1",
		f.ResourceID).Scan(&status, &detail))
	assert.Equal(t, "none", status)
	assert.Empty(t, detail)

	set := func(s string) error {
		_, err := pool.Exec(ctx, "UPDATE resource SET agent_readonly_status = $1 WHERE id = $2", s, f.ResourceID)
		return err
	}
	for _, s := range []string{"pending", "ok", "has_write", "error", "none"} {
		require.NoError(t, set(s), s)
	}
	err := set("bogus")
	pgCode(t, err, "23514")
	assert.Contains(t, err.Error(), "resource_agent_readonly_status_check")

	sqlDB := stdlib.OpenDBFromPool(pool)
	defer func() { _ = sqlDB.Close() }()
	p, err := goose.NewProvider(goose.DialectPostgres, sqlDB, migrations.FS)
	require.NoError(t, err)
	_, err = p.DownTo(ctx, 262)
	require.NoError(t, err)
	for _, q := range []string{"SELECT agent_readonly_status FROM resource", "SELECT agent_readonly_detail FROM resource"} {
		_, err = pool.Exec(ctx, q)
		pgCode(t, err, "42703")
	}
	_, err = p.Up(ctx)
	require.NoError(t, err)
	require.NoError(t, pool.QueryRow(ctx, "SELECT agent_readonly_status FROM resource WHERE id = $1", f.ResourceID).Scan(&status))
	assert.Equal(t, "none", status, "an existing resource starts with none")
}
