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
