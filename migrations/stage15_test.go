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

// TestMigrations15Transport: session.transport defaults to tcp, accepts tcp,
// ws and internal only, gateway.ws_url defaults to "", and 00220 goes down to
// 00205 and up again with web sessions backfilled to internal.
func TestMigrations15Transport(t *testing.T) {
	pool, _ := dbtest.New(t)
	ctx := context.Background()
	f := dbtest.Seed(t, pool)

	insert := func(front, transport string) error {
		_, err := pool.Exec(ctx, `INSERT INTO session (org_id, user_id, resource_id, gateway_id, gateway_instance_id, client_ip, front, transport)
			VALUES ($1, $2, $3, $4, $5, '10.0.0.1', $6, $7)`, f.OrgID, f.MemberID, f.ResourceID, f.GatewayID, f.InstanceID, front, transport)
		return err
	}
	for _, tr := range []string{"tcp", "ws", "internal"} {
		require.NoError(t, insert("ssh", tr), tr)
	}
	pgCode(t, insert("ssh", "udp"), "23514")

	var wsURL, transport string
	require.NoError(t, pool.QueryRow(ctx, "SELECT ws_url FROM gateway WHERE id = $1", f.GatewayID).Scan(&wsURL))
	assert.Empty(t, wsURL)
	_, err := pool.Exec(ctx, `INSERT INTO session (org_id, user_id, resource_id, gateway_id, gateway_instance_id, client_ip)
		VALUES ($1, $2, $3, $4, $5, '10.0.0.2')`, f.OrgID, f.MemberID, f.ResourceID, f.GatewayID, f.InstanceID)
	require.NoError(t, err)
	require.NoError(t, pool.QueryRow(ctx, "SELECT transport FROM session WHERE client_ip = '10.0.0.2'").Scan(&transport))
	assert.Equal(t, "tcp", transport)

	sqlDB := stdlib.OpenDBFromPool(pool)
	defer func() { _ = sqlDB.Close() }()
	p, err := goose.NewProvider(goose.DialectPostgres, sqlDB, migrations.FS)
	require.NoError(t, err)
	_, err = p.DownTo(ctx, 205)
	require.NoError(t, err)
	for _, q := range []string{"SELECT transport FROM session", "SELECT ws_url FROM gateway"} {
		_, err = pool.Exec(ctx, q)
		pgCode(t, err, "42703")
	}
	_, err = pool.Exec(ctx, "UPDATE session SET front = 'web' WHERE client_ip = '10.0.0.2'")
	require.NoError(t, err)
	_, err = p.Up(ctx)
	require.NoError(t, err)
	require.NoError(t, pool.QueryRow(ctx, "SELECT transport FROM session WHERE client_ip = '10.0.0.2'").Scan(&transport))
	assert.Equal(t, "internal", transport, "an older web session is internal")
	require.NoError(t, pool.QueryRow(ctx, "SELECT transport FROM session WHERE front = 'ssh' LIMIT 1").Scan(&transport))
	assert.Equal(t, "tcp", transport)
}

// TestMigrations15Preset: resource_preset accepts lower-case names and the
// tcp and udp base types only, a preset in use cannot be deleted, a resource
// cannot point at a preset of another organization, and 00221 goes down and up.
func TestMigrations15Preset(t *testing.T) {
	pool, _ := dbtest.New(t)
	ctx := context.Background()
	f := dbtest.Seed(t, pool)
	other := dbtest.Org(t, pool)

	insert := func(org any, name, base string) (id string, err error) {
		err = pool.QueryRow(ctx, `INSERT INTO resource_preset (org_id, name, base_type, display_name, default_ports, category)
			VALUES ($1, $2, $3, 'Display', '[{"port":5432}]', 'databases') RETURNING id::text`, org, name, base).Scan(&id)
		return id, err
	}
	_, err := insert(f.OrgID, "Bad", "tcp")
	pgCode(t, err, "23514")
	_, err = insert(f.OrgID, "pg", "ssh")
	pgCode(t, err, "23514")
	id, err := insert(f.OrgID, "pg", "tcp")
	require.NoError(t, err)
	_, err = insert(f.OrgID, "pg", "udp")
	pgCode(t, err, "23505")
	otherID, err := insert(other, "pg", "tcp")
	require.NoError(t, err)

	_, err = pool.Exec(ctx, "UPDATE resource SET preset_id = $1 WHERE id = $2", otherID, f.ResourceID)
	pgCode(t, err, "23503")
	_, err = pool.Exec(ctx, "UPDATE resource SET preset_id = $1 WHERE id = $2", id, f.ResourceID)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, "DELETE FROM resource_preset WHERE id = $1", id)
	pgCode(t, err, "23001")

	sqlDB := stdlib.OpenDBFromPool(pool)
	defer func() { _ = sqlDB.Close() }()
	p, err := goose.NewProvider(goose.DialectPostgres, sqlDB, migrations.FS)
	require.NoError(t, err)
	_, err = p.Down(ctx)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, "SELECT preset_id FROM resource")
	pgCode(t, err, "42703")
	_, err = p.Up(ctx)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, "SELECT preset_id FROM resource")
	require.NoError(t, err)
}
