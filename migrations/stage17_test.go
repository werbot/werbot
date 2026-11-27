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
