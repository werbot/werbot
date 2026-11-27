package migrations_test

import (
	"context"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/platform/db/dbtest"
	"github.com/werbot/werbot/migrations"
)

// TestMigrations17Querylog: recording.format defaults to asciicast and accepts
// asciicast and querylog only, session.queries defaults to 0, and 00260 goes
// down to 00221 and up again.
func TestMigrations17Querylog(t *testing.T) {
	pool, _ := dbtest.New(t)
	ctx := context.Background()
	f := dbtest.Seed(t, pool)

	newSession := func(ip string) (id string) {
		require.NoError(t, pool.QueryRow(ctx, `INSERT INTO session (org_id, user_id, resource_id, gateway_id, gateway_instance_id, client_ip)
			VALUES ($1, $2, $3, $4, $5, $6) RETURNING id`, f.OrgID, f.MemberID, f.ResourceID, f.GatewayID, f.InstanceID, ip).Scan(&id))
		return id
	}
	insertRec := func(sessionID string, format ...string) error {
		if len(format) == 0 {
			_, err := pool.Exec(ctx, "INSERT INTO recording (session_id, org_id, store) VALUES ($1, $2, 'fs')", sessionID, f.OrgID)
			return err
		}
		_, err := pool.Exec(ctx, "INSERT INTO recording (session_id, org_id, store, format) VALUES ($1, $2, 'fs', $3)", sessionID, f.OrgID, format[0])
		return err
	}

	s1, s2, s3 := newSession("10.0.0.1"), newSession("10.0.0.2"), newSession("10.0.0.3")
	require.NoError(t, insertRec(s1))
	require.NoError(t, insertRec(s2, "querylog"))
	pgCode(t, insertRec(s3, "json"), "23514")

	var format string
	var queries int64
	require.NoError(t, pool.QueryRow(ctx, "SELECT format FROM recording WHERE session_id = $1", s1).Scan(&format))
	assert.Equal(t, "asciicast", format)
	require.NoError(t, pool.QueryRow(ctx, "SELECT queries FROM session WHERE id = $1", s1).Scan(&queries))
	assert.Zero(t, queries)

	sqlDB := stdlib.OpenDBFromPool(pool)
	defer func() { _ = sqlDB.Close() }()
	p, err := goose.NewProvider(goose.DialectPostgres, sqlDB, migrations.FS)
	require.NoError(t, err)
	_, err = p.DownTo(ctx, 221)
	require.NoError(t, err)
	for _, q := range []string{"SELECT format FROM recording", "SELECT queries FROM session"} {
		_, err = pool.Exec(ctx, q)
		pgCode(t, err, "42703")
	}
	_, err = p.Up(ctx)
	require.NoError(t, err)
	require.NoError(t, pool.QueryRow(ctx, "SELECT format FROM recording WHERE session_id = $1", s1).Scan(&format))
	assert.Equal(t, "asciicast", format, "an older recording is asciicast")
}

// TestMigrations17CredentialSlot: a resource has one credential per slot
// (primary, agent_readonly), kind client_cert is accepted, and 00261 goes down
// to 00221 dropping the non-primary and client_cert rows, then up again.
func TestMigrations17CredentialSlot(t *testing.T) {
	pool, _ := dbtest.New(t)
	ctx := context.Background()
	f := dbtest.Seed(t, pool)

	insert := func(slot, kind string) error {
		_, err := pool.Exec(ctx, `INSERT INTO credential (org_id, resource_id, kind, ciphertext, key_id, slot)
			VALUES ($1, $2, $3, '\x00', 'k', $4)`, f.OrgID, f.ResourceID, kind, slot)
		return err
	}
	require.NoError(t, insert("primary", "password"))
	require.NoError(t, insert("agent_readonly", "password"))
	err := insert("primary", "password")
	pgCode(t, err, "23505")
	assert.Contains(t, err.Error(), "credential_resource_id_slot_key")
	pgCode(t, insert("other", "password"), "23514")
	_, err = pool.Exec(ctx, "UPDATE credential SET kind = 'client_cert' WHERE slot = 'agent_readonly'")
	require.NoError(t, err)
	_, err = pool.Exec(ctx, "UPDATE credential SET kind = 'api_key' WHERE slot = 'primary'")
	pgCode(t, err, "23514")

	sqlDB := stdlib.OpenDBFromPool(pool)
	defer func() { _ = sqlDB.Close() }()
	p, err := goose.NewProvider(goose.DialectPostgres, sqlDB, migrations.FS)
	require.NoError(t, err)
	_, err = p.DownTo(ctx, 221)
	require.NoError(t, err)
	var n int
	require.NoError(t, pool.QueryRow(ctx, "SELECT count(*) FROM credential").Scan(&n))
	assert.Equal(t, 1, n, "only the primary row survives")
	_, err = pool.Exec(ctx, "SELECT slot FROM credential")
	pgCode(t, err, "42703")
	_, err = p.Up(ctx)
	require.NoError(t, err)
	require.NoError(t, insert("agent_readonly", "client_cert"))
}

// TestMigrations17PresetRename: 00262 renames a preset named after the builtin
// type postgres to postgres-<8 hex of its id>, its resources keep preset_id,
// other presets keep their names; Down changes nothing, and Up runs again.
func TestMigrations17PresetRename(t *testing.T) {
	pool, _ := dbtest.New(t)
	ctx := context.Background()
	f := dbtest.Seed(t, pool)

	sqlDB := stdlib.OpenDBFromPool(pool)
	defer func() { _ = sqlDB.Close() }()
	p, err := goose.NewProvider(goose.DialectPostgres, sqlDB, migrations.FS)
	require.NoError(t, err)
	_, err = p.DownTo(ctx, 261)
	require.NoError(t, err)

	preset := func(name string) (id string) {
		require.NoError(t, pool.QueryRow(ctx, `INSERT INTO resource_preset (org_id, name, base_type, display_name, default_ports, category)
			VALUES ($1, $2, 'tcp', 'PostgreSQL', '[{"port":5432}]', 'databases') RETURNING id`, f.OrgID, name).Scan(&id))
		return id
	}
	pg, other := preset("postgres"), preset("pgtcp")
	_, err = pool.Exec(ctx, "UPDATE resource SET preset_id = $1 WHERE id = $2", pg, f.ResourceID)
	require.NoError(t, err)

	_, err = p.Up(ctx)
	require.NoError(t, err)
	name := func(id string) (n string) {
		require.NoError(t, pool.QueryRow(ctx, "SELECT name FROM resource_preset WHERE id = $1", id).Scan(&n))
		return n
	}
	want := "postgres-" + strings.ReplaceAll(pg, "-", "")[:8]
	assert.Equal(t, want, name(pg))
	assert.Equal(t, "pgtcp", name(other))
	var presetID string
	require.NoError(t, pool.QueryRow(ctx, "SELECT preset_id FROM resource WHERE id = $1", f.ResourceID).Scan(&presetID))
	assert.Equal(t, pg, presetID, "the resource keeps its preset")

	_, err = p.DownTo(ctx, 221)
	require.NoError(t, err)
	assert.Equal(t, want, name(pg), "down keeps the new name")
	_, err = p.Up(ctx)
	require.NoError(t, err)
}
