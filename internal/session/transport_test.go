package session

import (
	"net/netip"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/platform/apperr"
)

// Start stores the client transport ("" is tcp), writes it to the
// session.start audit row, and refuses a transport outside tcp, ws, internal.
func TestStartStoresTransport(t *testing.T) {
	e := setup(t)
	start := func(transport string) (uuid.UUID, error) {
		return e.svc.Start(bg, Start{OrgID: e.org, UserID: e.user, ResourceID: e.res, GatewayID: e.gw, InstanceID: e.a,
			ClientIP: netip.MustParseAddr("10.0.0.1"), Transport: transport})
	}
	for transport, want := range map[string]string{"": "tcp", "tcp": "tcp", "ws": "ws", "internal": "internal"} {
		id, err := start(transport)
		require.NoError(t, err, transport)
		var stored, audited string
		require.NoError(t, e.pool.QueryRow(bg, `SELECT s.transport, a.data->>'transport' FROM session s
			JOIN audit_event a ON a.action = 'session.start' AND a.data->>'session_id' = s.id::text WHERE s.id = $1`, id).Scan(&stored, &audited))
		assert.Equal(t, want, stored, transport)
		assert.Equal(t, want, audited, transport)
	}

	_, err := start("udp")
	require.ErrorIs(t, err, apperr.ErrInvalidArgument)
	var n int
	require.NoError(t, e.pool.QueryRow(bg, "SELECT count(*) FROM session").Scan(&n))
	assert.Equal(t, 4, n, "an unknown transport starts no session")
}
