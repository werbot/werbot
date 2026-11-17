package migrations_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/platform/db/dbtest"
)

func pgCode(t *testing.T, err error, code string) {
	t.Helper()
	var e *pgconn.PgError
	require.ErrorAs(t, err, &e)
	assert.Equal(t, code, e.Code)
}

func TestGeoIPDBSingleRow(t *testing.T) {
	pool, _ := dbtest.New(t)
	ctx := context.Background()
	const q = "INSERT INTO geoip_db (id, month, data) VALUES ($1, $2, '\\x00')"
	_, err := pool.Exec(ctx, q, 1, "2026-11-01")
	require.NoError(t, err)
	_, err = pool.Exec(ctx, q, 2, "2026-11-01")
	pgCode(t, err, "23514")
	_, err = pool.Exec(ctx, q, 1, "2026-11-01")
	pgCode(t, err, "23505")
	_, err = pool.Exec(ctx, "UPDATE geoip_db SET month = '2026-11-15'")
	pgCode(t, err, "23514")
}

func TestResourceNameCheck(t *testing.T) {
	pool, _ := dbtest.New(t)
	org := dbtest.Org(t, pool)
	_, err := pool.Exec(context.Background(),
		"INSERT INTO resource (org_id, name, type, category, config) VALUES ($1, 'Prod_DB', 'ssh', 'servers', '{}')", org)
	pgCode(t, err, "23514")
}

func TestChildRowsCannotCrossOrgs(t *testing.T) {
	pool, _ := dbtest.New(t)
	ctx := context.Background()
	orgA, orgB := dbtest.Org(t, pool), dbtest.Org(t, pool)
	res := dbtest.Resource(t, pool, orgA, "web", nil)
	for name, q := range map[string]string{
		"credential":     "INSERT INTO credential (org_id, resource_id, kind, ciphertext, key_id) VALUES ($1, $2, 'password', '\\x00', 'k')",
		"resource_label": "INSERT INTO resource_label (org_id, resource_id, key, value) VALUES ($1, $2, 'env', 'prod')",
		"access_binding": "INSERT INTO access_binding (org_id, resource_id, subject_type, subject_id, created_by) VALUES ($1, $2, 'user', gen_random_uuid(), gen_random_uuid())",
		"network_rule":   "INSERT INTO resource_network_rule (org_id, resource_id, cidr, effect) VALUES ($1, $2, '10.0.0.0/8', 'allow')",
	} {
		_, err := pool.Exec(ctx, q, orgB, res)
		require.Error(t, err, name)
		pgCode(t, err, "23503")
	}
}

func TestRecordingCannotCrossOrgs(t *testing.T) {
	pool, _ := dbtest.New(t)
	ctx := context.Background()
	orgA, orgB := dbtest.Org(t, pool), dbtest.Org(t, pool)
	var sess uuid.UUID
	require.NoError(t, pool.QueryRow(ctx,
		"INSERT INTO session (org_id, client_ip) VALUES ($1, '10.0.0.1') RETURNING id", orgA).Scan(&sess))
	const q = "INSERT INTO recording (session_id, org_id, store) VALUES ($1, $2, 'fs')"
	_, err := pool.Exec(ctx, q, sess, orgB)
	pgCode(t, err, "23503")
	_, err = pool.Exec(ctx, q, sess, orgA)
	require.NoError(t, err)
}

func TestBindingNeedsExactlyOneTarget(t *testing.T) {
	pool, _ := dbtest.New(t)
	ctx := context.Background()
	org := dbtest.Org(t, pool)
	res := dbtest.Resource(t, pool, org, "web", nil)
	const q = "INSERT INTO access_binding (org_id, subject_type, subject_id, created_by, resource_id, label_selector) VALUES ($1, 'user', $2, $2, $3, $4)"
	id := uuid.New()
	_, err := pool.Exec(ctx, q, org, id, nil, nil)
	pgCode(t, err, "23514")
	_, err = pool.Exec(ctx, q, org, id, res, `{"env":"prod"}`)
	pgCode(t, err, "23514")
}

func TestDeleteResourceCascades(t *testing.T) {
	pool, _ := dbtest.New(t)
	ctx := context.Background()
	org := dbtest.Org(t, pool)
	res := dbtest.Resource(t, pool, org, "web", map[string]string{"env": "prod"})
	for _, q := range []string{
		"INSERT INTO credential (org_id, resource_id, kind, ciphertext, key_id) VALUES ($1, $2, 'password', '\\x00', 'k')",
		"INSERT INTO resource_host_key (org_id, resource_id, key_type, public_key, fingerprint_sha256) VALUES ($1, $2, 'ssh-ed25519', 'k', 'f')",
		"INSERT INTO access_binding (org_id, resource_id, subject_type, subject_id, created_by) VALUES ($1, $2, 'user', gen_random_uuid(), gen_random_uuid())",
		"INSERT INTO resource_network_rule (org_id, resource_id, cidr, effect) VALUES ($1, $2, '10.0.0.0/8', 'allow')",
		"INSERT INTO resource_country_rule (org_id, resource_id, country, effect) VALUES ($1, $2, 'DE', 'deny')",
		"INSERT INTO resource_schedule (org_id, resource_id, weekday, hour) VALUES ($1, $2, 1, 9)",
	} {
		_, err := pool.Exec(ctx, q, org, res)
		require.NoError(t, err, q)
	}
	_, err := pool.Exec(ctx, "DELETE FROM resource WHERE id = $1", res)
	require.NoError(t, err)
	for _, table := range []string{"resource_label", "credential", "resource_host_key", "access_binding",
		"resource_network_rule", "resource_country_rule", "resource_schedule"} {
		var n int
		require.NoError(t, pool.QueryRow(ctx, "SELECT count(*) FROM "+table).Scan(&n))
		assert.Zero(t, n, table)
	}
}

func TestDeletingSubjectRemovesItsBindings(t *testing.T) {
	pool, _ := dbtest.New(t)
	ctx := context.Background()
	org := dbtest.Org(t, pool)
	res := dbtest.Resource(t, pool, org, "web", nil)
	user := dbtest.User(t, pool, org, "member")
	other := dbtest.User(t, pool, org, "member")
	team := dbtest.Team(t, pool, org, user)
	bind := func(typ string, subject uuid.UUID) {
		_, err := pool.Exec(ctx, `INSERT INTO access_binding (org_id, resource_id, subject_type, subject_id, created_by)
			VALUES ($1, $2, $3, $4, $5)`, org, res, typ, subject, user)
		require.NoError(t, err)
	}
	bind("user", user)
	bind("user", other)
	bind("team", team)
	count := func() (n int) {
		require.NoError(t, pool.QueryRow(ctx, "SELECT count(*) FROM access_binding").Scan(&n))
		return n
	}

	_, err := pool.Exec(ctx, "DELETE FROM team WHERE id = $1", team)
	require.NoError(t, err)
	assert.Equal(t, 2, count())
	_, err = pool.Exec(ctx, "DELETE FROM membership WHERE org_id = $1 AND user_id = $2", org, user)
	require.NoError(t, err)
	assert.Equal(t, 1, count())
	var subject uuid.UUID
	require.NoError(t, pool.QueryRow(ctx, "SELECT subject_id FROM access_binding").Scan(&subject))
	assert.Equal(t, other, subject)
}
