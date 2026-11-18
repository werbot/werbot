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
	notified := func(d time.Duration) bool {
		ctx, cancel := context.WithTimeout(bg, d)
		defer cancel()
		n, err := conn.WaitForNotification(ctx)
		if err != nil {
			return false
		}
		assert.Equal(t, e.org.String(), n.Payload)
		return true
	}

	k := newKey(t)
	require.NoError(t, e.svc.PinHostKey(e.ctx, e.org, r.ID, k))
	assert.True(t, notified(5*time.Second), "first pin")
	require.NoError(t, e.svc.PinHostKey(e.ctx, e.org, r.ID, k))
	assert.False(t, notified(300*time.Millisecond), "the same key again changes nothing")
	require.NoError(t, e.svc.ClearHostKey(e.ctx, e.org, r.ID))
	assert.True(t, notified(5*time.Second), "clear")
}
