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

func TestMigrationsStage11(t *testing.T) {
	pool, _ := dbtest.New(t)
	ctx := context.Background()
	sqlDB := stdlib.OpenDBFromPool(pool)
	defer func() { _ = sqlDB.Close() }()
	p, err := goose.NewProvider(goose.DialectPostgres, sqlDB, migrations.FS)
	require.NoError(t, err)

	_, err = p.DownTo(ctx, 139)
	require.NoError(t, err)
	old := dbtest.Org(t, pool)
	_, err = p.Up(ctx)
	require.NoError(t, err)

	var mask string
	require.NoError(t, pool.QueryRow(ctx, "SELECT mask_output FROM org_setting WHERE org_id = $1", old).Scan(&mask))
	assert.Equal(t, "agents", mask)

	f := dbtest.Seed(t, pool)
	const rule = `INSERT INTO command_rule (org_id, name, subjects, resource_selector, classes, effect, message) VALUES ($1, 'x', '{}', '{}', %s, %s, %s)`
	for name, q := range map[string]string{
		"class rule empty classes": `INSERT INTO command_class_rule (org_id, pattern, classes) VALUES ($1, 'p', '{}')`,
		"class rule bogus class":   `INSERT INTO command_class_rule (org_id, pattern, classes) VALUES ($1, 'p', '{bogus}')`,
		"class rule empty pattern": `INSERT INTO command_class_rule (org_id, pattern, classes) VALUES ($1, '', '{read}')`,
		"rule empty classes":       fmt.Sprintf(rule, `'{}'`, `'deny'`, `''`),
		"rule bogus class":         fmt.Sprintf(rule, `'{bogus}'`, `'deny'`, `''`),
		"rule bogus effect":        fmt.Sprintf(rule, `'{read}'`, `'maybe'`, `''`),
		"rule long message":        fmt.Sprintf(rule, `'{read}'`, `'deny'`, `'`+strings.Repeat("m", 257)+`'`),
		"rule subjects not object": `INSERT INTO command_rule (org_id, name, subjects, resource_selector, classes, effect) VALUES ($1, 'x', '[]', '{}', '{read}', 'deny')`,
		"policy empty cedar":       `INSERT INTO custom_policy (org_id, name, cedar) VALUES ($1, 'x', '')`,
		"policy oversize cedar":    `INSERT INTO custom_policy (org_id, name, cedar) VALUES ($1, 'x', '` + strings.Repeat("c", 65537) + `')`,
	} {
		_, err := pool.Exec(ctx, q, f.OrgID)
		require.Error(t, err, name)
		pgCode(t, err, "23514")
	}
	_, err = pool.Exec(ctx, "UPDATE org_setting SET mask_output = 'some'")
	pgCode(t, err, "23514")

	for _, q := range []string{
		`INSERT INTO command_class_rule (org_id, pattern, classes) VALUES ($1, 'p', '{read,opaque}')`,
		fmt.Sprintf(rule, `'{read}'`, `'approval'`, `''`),
		`INSERT INTO custom_policy (org_id, name, cedar) VALUES ($1, 'x', 'forbid(principal, action, resource);')`,
	} {
		_, err = pool.Exec(ctx, q, f.OrgID)
		require.NoError(t, err)
		_, err = pool.Exec(ctx, q, f.OrgID)
		pgCode(t, err, "23505")
	}

	_, err = pool.Exec(ctx, "UPDATE org_setting SET mask_output = 'all' WHERE org_id = $1", f.OrgID)
	require.NoError(t, err)
	_, err = p.DownTo(ctx, 139)
	require.NoError(t, err)
	_, err = p.Up(ctx)
	require.NoError(t, err)
}

func TestStage11TriggersNotify(t *testing.T) {
	pool, _ := dbtest.New(t)
	ctx := context.Background()
	f := dbtest.Seed(t, pool)
	conn, err := pool.Acquire(ctx)
	require.NoError(t, err)
	defer conn.Release()
	_, err = conn.Exec(ctx, "LISTEN werbot_org_changed")
	require.NoError(t, err)

	for _, q := range []string{
		`INSERT INTO command_rule (org_id, name, subjects, resource_selector, classes, effect) VALUES ($1, 'x', '{}', '{}', '{read}', 'deny')`,
		`INSERT INTO command_class_rule (org_id, pattern, classes) VALUES ($1, 'p', '{read}')`,
		`INSERT INTO custom_policy (org_id, name, cedar) VALUES ($1, 'x', 'forbid(principal, action, resource);')`,
		`UPDATE org_setting SET mask_output = 'none' WHERE org_id = $1`,
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
