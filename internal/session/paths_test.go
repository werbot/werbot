package session

import (
	"net/netip"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/platform/db/dbtest"
	"github.com/werbot/werbot/internal/platform/principal"
)

func (e env) start(t *testing.T, paths []string) uuid.UUID {
	t.Helper()
	id, err := e.svc.Start(bg, Start{OrgID: e.org, UserID: e.user, ResourceID: e.res, GatewayID: e.gw, InstanceID: e.a,
		ClientIP: netip.MustParseAddr("10.0.0.1"), Kind: "exec", Command: "cat /etc/hosts", Classes: []string{"read"}, Paths: paths})
	require.NoError(t, err)
	return id
}

func (e env) stored(t *testing.T, id uuid.UUID) (cmd *string, classes, paths []string) {
	t.Helper()
	require.NoError(t, e.pool.QueryRow(bg, "SELECT command, classes, paths FROM session WHERE id = $1", id).Scan(&cmd, &classes, &paths))
	return cmd, classes, paths
}

func TestStartStoresPaths(t *testing.T) {
	e := setup(t)
	_, _, paths := e.stored(t, e.start(t, []string{"/etc/hosts"}))
	assert.Equal(t, []string{"/etc/hosts"}, paths)
	_, _, paths = e.stored(t, e.start(t, nil))
	assert.Nil(t, paths, "nil paths are NULL")
}

func TestEndStoresConsoleResult(t *testing.T) {
	e := setup(t)
	id, err := e.svc.Start(bg, Start{OrgID: e.org, UserID: e.user, ResourceID: e.res, GatewayID: e.gw, InstanceID: e.a,
		ClientIP: netip.MustParseAddr("10.0.0.1")})
	require.NoError(t, err)

	r := &Result{Command: "cp a b", Classes: []string{"read", "write"}, Paths: []string{"~/a", "~/b"}}
	require.NoError(t, e.svc.End(bg, e.org, e.a, id, "closed", nil, r))
	cmd, classes, paths := e.stored(t, id)
	require.NotNil(t, cmd)
	assert.Equal(t, [3]any{"cp a b", []string{"read", "write"}, []string{"~/a", "~/b"}}, [3]any{*cmd, classes, paths})

	require.NoError(t, e.svc.End(bg, e.org, e.a, id, "again", nil, &Result{Command: "rm x", Classes: []string{"destructive"}, Paths: []string{"x"}}))
	cmd, classes, paths = e.stored(t, id)
	assert.Equal(t, [3]any{"cp a b", []string{"read", "write"}, []string{"~/a", "~/b"}}, [3]any{*cmd, classes, paths}, "only the first close stores the result")
}

func TestEndWithoutResultKeepsColumns(t *testing.T) {
	e := setup(t)
	id := e.start(t, []string{"/etc/hosts"})
	require.NoError(t, e.svc.End(bg, e.org, e.a, id, "closed", nil, nil))
	cmd, classes, paths := e.stored(t, id)
	assert.Equal(t, [3]any{"cat /etc/hosts", []string{"read"}, []string{"/etc/hosts"}}, [3]any{*cmd, classes, paths})
}

func TestDenyCommandDenied(t *testing.T) {
	e := setup(t)
	id := e.start(t, nil)
	other := dbtest.Org(t, e.pool)
	otherGw := dbtest.Gateway(t, e.pool, other, "edge")
	otherID, err := e.svc.Start(bg, Start{OrgID: other, UserID: dbtest.User(t, e.pool, other, "member"), ResourceID: dbtest.Resource(t, e.pool, other, "web", nil),
		GatewayID: otherGw, InstanceID: dbtest.Instance(t, e.pool, other, otherGw), ClientIP: netip.MustParseAddr("10.0.0.1")})
	require.NoError(t, err)

	d := Denial{UserID: &e.user, Resource: "web", Kind: "terminal", Target: "cat /etc/hosts", Reason: "denied",
		Classes: []string{"read"}, Paths: []string{"/etc/hosts"}, Reasons: []string{"path:r1"}}
	foreign := d
	foreign.SessionID = &otherID // a session of another organization
	d.SessionID = &id
	plain := Denial{Resource: "web", Kind: "exec", Reason: "no_access"}
	require.NoError(t, e.svc.Deny(bg, e.org, []Denial{d, foreign, plain}))

	var n int
	require.NoError(t, e.pool.QueryRow(bg, `SELECT count(*) FROM audit_event WHERE org_id = $1 AND action = 'session.deny'`, e.org).Scan(&n))
	assert.Equal(t, 1, n, "a denial without a session stays session.deny")

	var kind, typ, target, data string
	require.NoError(t, e.pool.QueryRow(bg, `SELECT actor_kind, target_type, target_id, data::text FROM audit_event
		WHERE org_id = $1 AND action = 'session.command_denied'`, e.org).Scan(&kind, &typ, &target, &data))
	assert.Equal(t, [3]string{"gateway", "session", id.String()}, [3]string{kind, typ, target})
	assert.JSONEq(t, `{"command":"cat /etc/hosts","classes":["read"],"paths":["/etc/hosts"],"reasons":["path:r1"],"resource":"web","reason":"denied"}`, data)

	require.NoError(t, e.pool.QueryRow(bg, `SELECT count(*) FROM audit_event WHERE action = 'session.command_denied'`).Scan(&n))
	assert.Equal(t, 1, n, "a session of another organization writes no event")
}

func TestSessionPathsInAPI(t *testing.T) {
	e := setup(t)
	p := principal.Principal{OrgID: e.org, UserID: e.user, Role: principal.RoleOwner}
	id := e.start(t, []string{"/etc/hosts", "~/a"})

	got, err := e.svc.GetSession(bg, p, id)
	require.NoError(t, err)
	assert.Equal(t, []string{"/etc/hosts", "~/a"}, got.Paths)
	list, _, err := e.svc.ListSessions(bg, p, ListFilter{})
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, []string{"/etc/hosts", "~/a"}, list[0].Paths)
	assert.Equal(t, []string{"/etc/hosts", "~/a"}, toProto(list[0]).GetPaths())
}
