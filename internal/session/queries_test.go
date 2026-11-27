package session

import (
	"net/netip"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// session.start carries the database the client asked for; an empty one adds no key.
func TestStartDBName(t *testing.T) {
	e := setup(t)
	dbName := func(name string) (value *string, present bool) {
		id, err := e.svc.Start(bg, Start{OrgID: e.org, UserID: e.user, ResourceID: e.res, GatewayID: e.gw, InstanceID: e.a,
			ClientIP: netip.MustParseAddr("10.0.0.1"), Kind: "stream", DBName: name})
		require.NoError(t, err)
		require.NoError(t, e.pool.QueryRow(bg, `SELECT data->>'db_name', data ? 'db_name' FROM audit_event
			WHERE action = 'session.start' AND data->>'session_id' = $1`, id.String()).Scan(&value, &present))
		return value, present
	}
	v, ok := dbName("app")
	require.True(t, ok)
	assert.Equal(t, "app", *v)
	_, ok = dbName("")
	assert.False(t, ok, "no db_name key without a database")
}

// End stores the query count of the first close, as bigint; a repeat close keeps it.
func TestEndQueries(t *testing.T) {
	e := setup(t)
	queries := func(id uuid.UUID) (n int64) {
		require.NoError(t, e.pool.QueryRow(bg, "SELECT queries FROM session WHERE id = $1", id).Scan(&n))
		return n
	}
	id := e.open(t, &e.a, 0)
	require.NoError(t, e.svc.End(bg, e.org, e.a, id, "closed", nil, &Result{Queries: 7}))
	assert.Equal(t, int64(7), queries(id))
	require.NoError(t, e.svc.End(bg, e.org, e.a, id, "closed", nil, &Result{Queries: 3}))
	assert.Equal(t, int64(7), queries(id))

	big := e.open(t, &e.a, 0)
	require.NoError(t, e.svc.End(bg, e.org, e.a, big, "closed", nil, &Result{Queries: 3_000_000_000}))
	assert.Equal(t, int64(3_000_000_000), queries(big))

	none := e.open(t, &e.a, 0)
	require.NoError(t, e.svc.End(bg, e.org, e.a, none, "closed", nil, nil))
	assert.Zero(t, queries(none))
}
