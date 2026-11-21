package session

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/platform/principal"
)

func TestStartStoresCommandClasses(t *testing.T) {
	e := setup(t)
	st := Start{OrgID: e.org, UserID: e.user, ResourceID: e.res, GatewayID: e.gw, InstanceID: e.a, ClientIP: netip.MustParseAddr("10.0.0.1")}

	st.Kind, st.Command, st.Classes = "exec", "ls -la", []string{"read"}
	execID, err := e.svc.Start(bg, st)
	require.NoError(t, err)
	st.Kind, st.Command, st.Classes = "terminal", "", nil
	termID, err := e.svc.Start(bg, st)
	require.NoError(t, err)

	var cmd *string
	var classes []string
	require.NoError(t, e.pool.QueryRow(bg, "SELECT command, classes FROM session WHERE id = $1", execID).Scan(&cmd, &classes))
	require.NotNil(t, cmd)
	assert.Equal(t, "ls -la", *cmd)
	assert.Equal(t, []string{"read"}, classes)

	require.NoError(t, e.pool.QueryRow(bg, "SELECT command, classes FROM session WHERE id = $1", termID).Scan(&cmd, &classes))
	assert.Nil(t, cmd, "terminal has no command")
	assert.Nil(t, classes)
}

func TestSessionCommandInAPI(t *testing.T) {
	e := setup(t)
	p := principal.Principal{OrgID: e.org, UserID: e.user, Role: principal.RoleOwner}
	id, err := e.svc.Start(bg, Start{OrgID: e.org, UserID: e.user, ResourceID: e.res, GatewayID: e.gw, InstanceID: e.a,
		ClientIP: netip.MustParseAddr("10.0.0.1"), Kind: "exec", Command: "id", Classes: []string{"read", "privilege"}})
	require.NoError(t, err)

	got, err := e.svc.GetSession(bg, p, id)
	require.NoError(t, err)
	assert.Equal(t, "id", got.Command)
	assert.Equal(t, []string{"read", "privilege"}, got.Classes)

	list, _, err := e.svc.ListSessions(bg, p, ListFilter{})
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, "id", list[0].Command)
	assert.Equal(t, []string{"read", "privilege"}, list[0].Classes)
	assert.Equal(t, "id", toProto(list[0]).GetCommand())
	assert.Equal(t, []string{"read", "privilege"}, toProto(list[0]).GetClasses())
}
