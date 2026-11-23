package session

import (
	"net/netip"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/platform/apperr"
	"github.com/werbot/werbot/internal/platform/db/dbtest"
)

// The gateway names the session's user; Start records a session, with its
// audit row, only for a member of the organization.
func TestStartRequiresMember(t *testing.T) {
	e := setup(t)
	count := func(sql string) (n int) {
		require.NoError(t, e.pool.QueryRow(bg, sql).Scan(&n))
		return n
	}
	st := Start{OrgID: e.org, ResourceID: e.res, GatewayID: e.gw, InstanceID: e.a, ClientIP: netip.MustParseAddr("10.0.0.1")}

	otherOrg := dbtest.Org(t, e.pool)
	for name, user := range map[string]uuid.UUID{"other org": dbtest.User(t, e.pool, otherOrg, "owner"), "unknown": uuid.New()} {
		st.UserID = user
		_, err := e.svc.Start(bg, st)
		require.ErrorIs(t, err, apperr.ErrNotFound, name)
		assert.EqualError(t, err, "user not found: not found", name)
	}
	assert.Zero(t, count("SELECT count(*) FROM session"), "no session row for a non-member")
	assert.Zero(t, count("SELECT count(*) FROM audit_event WHERE action = 'session.start'"), "no audit row for a non-member")

	st.UserID = e.user
	id, err := e.svc.Start(bg, st)
	require.NoError(t, err)
	assert.Equal(t, 1, count("SELECT count(*) FROM session WHERE id = '"+id.String()+"'"))
	assert.Equal(t, 1, count("SELECT count(*) FROM audit_event WHERE action = 'session.start' AND data->>'session_id' = '"+id.String()+"'"))
}

// Start stores the front of the session ("" is ssh) and marks only a web
// session in its session.start audit row; another front is refused.
func TestStartStoresFront(t *testing.T) {
	e := setup(t)
	start := func(front string) (uuid.UUID, error) {
		return e.svc.Start(bg, Start{OrgID: e.org, UserID: e.user, ResourceID: e.res, GatewayID: e.gw, InstanceID: e.a,
			ClientIP: netip.MustParseAddr("10.0.0.1"), Front: front})
	}
	stored := func(id uuid.UUID) (front string, audit *string) {
		require.NoError(t, e.pool.QueryRow(bg, `SELECT s.front, a.data->>'front' FROM session s
			JOIN audit_event a ON a.action = 'session.start' AND a.data->>'session_id' = s.id::text WHERE s.id = $1`, id).Scan(&front, &audit))
		return front, audit
	}

	id, err := start("web")
	require.NoError(t, err)
	front, audit := stored(id)
	assert.Equal(t, "web", front)
	require.NotNil(t, audit)
	assert.Equal(t, "web", *audit)

	for _, f := range []string{"", "ssh"} {
		id, err = start(f)
		require.NoError(t, err)
		front, audit = stored(id)
		assert.Equal(t, "ssh", front, f)
		assert.Nil(t, audit, "an ssh session.start keeps its audit data")
	}

	_, err = start("tcp")
	require.ErrorIs(t, err, apperr.ErrInvalidArgument)
	var n int
	require.NoError(t, e.pool.QueryRow(bg, "SELECT count(*) FROM session").Scan(&n))
	assert.Equal(t, 3, n, "an unknown front starts no session")
}
