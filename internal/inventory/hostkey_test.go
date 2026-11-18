package inventory_test

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A pin and a clear change the pool snapshot (it carries the pinned
// fingerprint), so both notify werbot_org_changed; a repeated pin does not.
func TestHostKeyChangeNotifies(t *testing.T) {
	e := setup(t)
	r := e.create(t, "web", nil)
	bg := context.Background()
	conn, err := pgx.ConnectConfig(bg, e.pool.Config().ConnConfig)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close(bg) })
	_, err = conn.Exec(bg, "LISTEN werbot_org_changed")
	require.NoError(t, err)
	next := func() string {
		ctx, cancel := context.WithTimeout(bg, 5*time.Second)
		defer cancel()
		n, err := conn.WaitForNotification(ctx)
		require.NoError(t, err)
		return n.Payload
	}

	k := newKey(t)
	require.NoError(t, e.svc.PinHostKey(e.ctx, e.org, r.ID, k))
	assert.Equal(t, e.org.String(), next(), "first pin")
	require.NoError(t, e.svc.PinHostKey(e.ctx, e.org, r.ID, k))
	// Postgres delivers in commit order: a notify of the repeated pin would come before the sentinel.
	_, err = e.pool.Exec(bg, "SELECT pg_notify('werbot_org_changed', 'sentinel')")
	require.NoError(t, err)
	assert.Equal(t, "sentinel", next(), "the same key again changes nothing")
	require.NoError(t, e.svc.ClearHostKey(e.ctx, e.org, r.ID))
	assert.Equal(t, e.org.String(), next(), "clear")
}
