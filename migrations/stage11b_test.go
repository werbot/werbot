package migrations_test

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/platform/db/dbtest"
	"github.com/werbot/werbot/migrations"
)

func TestMigrationsStage11b(t *testing.T) {
	pool, _ := dbtest.New(t)
	ctx := context.Background()
	sqlDB := stdlib.OpenDBFromPool(pool)
	defer func() { _ = sqlDB.Close() }()
	p, err := goose.NewProvider(goose.DialectPostgres, sqlDB, migrations.FS)
	require.NoError(t, err)

	_, err = p.DownTo(ctx, 149)
	require.NoError(t, err)
	_, err = p.Up(ctx)
	require.NoError(t, err)

	f := dbtest.Seed(t, pool)
	const pr = `INSERT INTO path_rule (org_id, name, subjects, resource_selector, paths, access, effect, message) VALUES ($1, 'x', '{}', '{}', %s, %s, %s, %s)`
	const tr = `INSERT INTO terminal_rule (org_id, name, subjects, resource_selector, mode) VALUES ($1, 'x', '{}', '{}', %s)`
	manyPaths := "ARRAY[" + strings.TrimSuffix(strings.Repeat("'/p',", 33), ",") + "]"
	for name, q := range map[string]string{
		"paths empty":      fmt.Sprintf(pr, `'{}'`, `'read'`, `'only'`, `''`),
		"paths null item":  fmt.Sprintf(pr, `'{NULL}'`, `'read'`, `'only'`, `''`),
		"paths over 32":    fmt.Sprintf(pr, manyPaths, `'read'`, `'only'`, `''`),
		"access exec":      fmt.Sprintf(pr, `'{/a}'`, `'exec'`, `'only'`, `''`),
		"effect allow":     fmt.Sprintf(pr, `'{/a}'`, `'read'`, `'allow'`, `''`),
		"message too long": fmt.Sprintf(pr, `'{/a}'`, `'read'`, `'only'`, `'`+strings.Repeat("m", 257)+`'`),
		"subjects array":   `INSERT INTO path_rule (org_id, name, subjects, resource_selector, paths, access, effect) VALUES ($1, 'x', '[]', '{}', '{/a}', 'read', 'only')`,
		"mode shell":       fmt.Sprintf(tr, `'shell'`),
		"terminal name":    `INSERT INTO terminal_rule (org_id, name, subjects, resource_selector, mode) VALUES ($1, '', '{}', '{}', 'deny')`,
	} {
		_, err := pool.Exec(ctx, q, f.OrgID)
		require.Error(t, err, name)
		pgCode(t, err, "23514")
	}

	for _, q := range []string{fmt.Sprintf(pr, `'{/a,/b}'`, `'any'`, `'approval'`, `''`), fmt.Sprintf(tr, `'console'`)} {
		_, err = pool.Exec(ctx, q, f.OrgID)
		require.NoError(t, err)
		_, err = pool.Exec(ctx, q, f.OrgID)
		pgCode(t, err, "23505")
	}

	var col string
	require.NoError(t, pool.QueryRow(ctx, `SELECT data_type FROM information_schema.columns WHERE table_name = 'session' AND column_name = 'paths'`).Scan(&col))
	assert.Equal(t, "ARRAY", col)

	_, err = p.DownTo(ctx, 149)
	require.NoError(t, err)
	_, err = p.Up(ctx)
	require.NoError(t, err)
}

func TestStage11bTriggersNotify(t *testing.T) {
	pool, _ := dbtest.New(t)
	ctx := context.Background()
	f := dbtest.Seed(t, pool)
	conn, err := pool.Acquire(ctx)
	require.NoError(t, err)
	defer conn.Release()
	_, err = conn.Exec(ctx, "LISTEN werbot_org_changed")
	require.NoError(t, err)

	for _, q := range []string{
		`INSERT INTO path_rule (org_id, name, subjects, resource_selector, paths, access, effect) VALUES ($1, 'x', '{}', '{}', '{/a}', 'read', 'only')`,
		`UPDATE path_rule SET effect = 'deny' WHERE org_id = $1`,
		`INSERT INTO terminal_rule (org_id, name, subjects, resource_selector, mode) VALUES ($1, 'x', '{}', '{}', 'console')`,
		`DELETE FROM terminal_rule WHERE org_id = $1`,
	} {
		_, err = pool.Exec(ctx, q, f.OrgID)
		require.NoError(t, err)
	}

	other := dbtest.Org(t, pool) // marks the end: notifications arrive in commit order
	got := 0
	for {
		wctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		n, err := conn.Conn().WaitForNotification(wctx)
		cancel()
		require.NoError(t, err)
		if n.Payload == other.String() {
			break
		}
		assert.Equal(t, f.OrgID.String(), n.Payload)
		got++
	}
	assert.Equal(t, 4, got)
}
