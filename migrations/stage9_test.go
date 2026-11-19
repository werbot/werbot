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

func TestMigrationsStage9(t *testing.T) {
	pool, _ := dbtest.New(t)
	ctx := context.Background()
	sqlDB := stdlib.OpenDBFromPool(pool)
	defer func() { _ = sqlDB.Close() }()
	p, err := goose.NewProvider(goose.DialectPostgres, sqlDB, migrations.FS)
	require.NoError(t, err)

	_, err = p.DownTo(ctx, 99)
	require.NoError(t, err)
	_, err = p.Up(ctx)
	require.NoError(t, err)

	org := dbtest.Org(t, pool)
	for name, q := range map[string]string{
		"ttl below 1 hour":   "UPDATE org_setting SET cli_session_ttl = '10 minutes'",
		"ttl above 30 days":  "UPDATE org_setting SET cli_session_ttl = '31 days'",
		"max below 1 day":    "UPDATE org_setting SET cli_session_max = '12 hours'",
		"max above 90 days":  "UPDATE org_setting SET cli_session_max = '91 days'",
		"second signing key": "INSERT INTO oauth_signing_key (id, ciphertext, key_id, public_jwk) VALUES (2, '\\x00', 'k', '{}')",
		"bogus device status": "INSERT INTO oauth_device_code (device_code_hash, user_code, client_id, scopes, expires_at, status) " +
			"VALUES ('\\x01', 'ABCD', 'c', '{}', now(), 'bogus')",
	} {
		_, err := pool.Exec(ctx, q)
		require.Error(t, err, name)
		pgCode(t, err, "23514")
	}
	_, err = pool.Exec(ctx, "INSERT INTO oauth_signing_key (id, ciphertext, key_id, public_jwk) VALUES (1, '\\x00', 'k', '{}')")
	require.NoError(t, err)

	// a CLI session needs a membership of the same organization
	_, err = pool.Exec(ctx, `INSERT INTO cli_session (org_id, user_id, client_id, refresh_hash, expires_at, absolute_expires_at)
		VALUES ($1, gen_random_uuid(), 'werbot-cli', '\x02', now(), now())`, org)
	pgCode(t, err, "23503")

	gw := dbtest.Gateway(t, pool, org, "pool")
	var addr string
	require.NoError(t, pool.QueryRow(ctx, "SELECT ssh_address FROM gateway WHERE id = $1", gw).Scan(&addr))
	assert.Empty(t, addr)
}

func TestOrgSettingBackfill(t *testing.T) {
	pool, _ := dbtest.New(t)
	ctx := context.Background()
	sqlDB := stdlib.OpenDBFromPool(pool)
	defer func() { _ = sqlDB.Close() }()
	p, err := goose.NewProvider(goose.DialectPostgres, sqlDB, migrations.FS)
	require.NoError(t, err)

	_, err = p.DownTo(ctx, 102)
	require.NoError(t, err)
	var org string
	require.NoError(t, pool.QueryRow(ctx, "INSERT INTO organization (name) VALUES ('old') RETURNING id").Scan(&org))
	_, err = p.Up(ctx)
	require.NoError(t, err)

	var ttl, maxAge string
	require.NoError(t, pool.QueryRow(ctx,
		"SELECT cli_session_ttl::text, cli_session_max::text FROM org_setting WHERE org_id = $1", org).Scan(&ttl, &maxAge))
	assert.Equal(t, "7 days", ttl)
	assert.Equal(t, "30 days", maxAge)
}
