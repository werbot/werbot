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
