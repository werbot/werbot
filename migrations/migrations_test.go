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

func TestOrganizationIDIsUUIDv7(t *testing.T) {
	pool, _ := dbtest.New(t)
	var version int
	require.NoError(t, pool.QueryRow(context.Background(),
		"INSERT INTO organization (name) VALUES ('acme') RETURNING uuid_extract_version(id)").Scan(&version))
	assert.Equal(t, 7, version)
}

func TestUUIDKeysDefaultToUUIDv7(t *testing.T) {
	pool, _ := dbtest.New(t)
	ctx := context.Background()
	rows, err := pool.Query(ctx, `SELECT table_name, column_default FROM information_schema.columns
		WHERE table_schema = 'public' AND column_name = 'id' AND data_type = 'uuid'`)
	require.NoError(t, err)
	defaults := map[string]string{}
	for rows.Next() {
		var table, def string
		require.NoError(t, rows.Scan(&table, &def))
		defaults[table] = def
	}
	require.NoError(t, rows.Err())
	assert.Contains(t, defaults, "organization")
	assert.Contains(t, defaults, "audit_event")
	for table, def := range defaults {
		assert.Equal(t, "uuidv7()", def, table)
	}

	var version int
	require.NoError(t, pool.QueryRow(ctx,
		"INSERT INTO audit_event (actor_kind, action) VALUES ('system', 'test') RETURNING uuid_extract_version(id)").Scan(&version))
	assert.Equal(t, 7, version)
}

func TestRecordingMigrationsDownUp(t *testing.T) {
	pool, _ := dbtest.New(t)
	ctx := context.Background()
	sqlDB := stdlib.OpenDBFromPool(pool)
	defer func() { _ = sqlDB.Close() }()
	p, err := goose.NewProvider(goose.DialectPostgres, sqlDB, migrations.FS)
	require.NoError(t, err)

	exists := func(table string) bool {
		var ok bool
		require.NoError(t, sqlDB.QueryRowContext(ctx, "SELECT to_regclass($1) IS NOT NULL", table).Scan(&ok))
		return ok
	}
	require.True(t, exists("recording") && exists("signing_key"))
	_, err = p.DownTo(ctx, 59)
	require.NoError(t, err)
	assert.False(t, exists("recording") || exists("signing_key"))
	_, err = p.Up(ctx)
	require.NoError(t, err)
	assert.True(t, exists("recording") && exists("signing_key"))
}

func TestMigrations6b(t *testing.T) {
	pool, _ := dbtest.New(t)
	ctx := context.Background()
	sqlDB := stdlib.OpenDBFromPool(pool)
	defer func() { _ = sqlDB.Close() }()
	p, err := goose.NewProvider(goose.DialectPostgres, sqlDB, migrations.FS)
	require.NoError(t, err)

	// A stage 5/6 session row survives 00080 and reads as a terminal session.
	_, err = p.DownTo(ctx, 79)
	require.NoError(t, err)
	f := dbtest.Seed(t, pool)
	var id string
	require.NoError(t, pool.QueryRow(ctx, `INSERT INTO session (org_id, user_id, resource_id, gateway_id, gateway_instance_id, client_ip)
		VALUES ($1, $2, $3, $4, $5, '10.0.0.1') RETURNING id`, f.OrgID, f.MemberID, f.ResourceID, f.GatewayID, f.InstanceID).Scan(&id))
	_, err = p.Up(ctx)
	require.NoError(t, err)
	var kind string
	var up int64
	require.NoError(t, pool.QueryRow(ctx, "SELECT kind, bytes_up FROM session WHERE id = $1", id).Scan(&kind, &up))
	assert.Equal(t, "terminal", kind)
	assert.Zero(t, up)

	_, err = pool.Exec(ctx, "UPDATE session SET kind = 'bogus' WHERE id = $1", id)
	require.ErrorContains(t, err, "session_kind_check")
	_, err = pool.Exec(ctx, "UPDATE resource SET idle_timeout = '30 seconds' WHERE id = $1", f.ResourceID)
	require.Error(t, err)
	_, err = pool.Exec(ctx, "UPDATE resource SET idle_timeout = '10 minutes', max_duration = '8 hours' WHERE id = $1", f.ResourceID)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, "UPDATE resource SET max_duration = '169 hours' WHERE id = $1", f.ResourceID)
	require.Error(t, err)
	_, err = pool.Exec(ctx, "INSERT INTO resource_health (resource_id, gateway_instance_id, org_id, ok, checked_at) VALUES ($1, $2, $3, true, now())",
		f.ResourceID, f.InstanceID, f.OrgID)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, "INSERT INTO resource_health (resource_id, gateway_instance_id, org_id, ok, checked_at) VALUES ($1, $2, $3, true, now())",
		f.ResourceID, f.OtherInstanceID, f.OtherGatewayID)
	require.Error(t, err, "the organization must match the resource")

	_, err = p.DownTo(ctx, 79)
	require.NoError(t, err)
	_, err = p.Up(ctx)
	require.NoError(t, err)
}
