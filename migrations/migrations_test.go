package migrations_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/platform/db/dbtest"
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
