package inventory_test

import (
	"context"
	"testing"

	"github.com/riverqueue/river"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/inventory"
	"github.com/werbot/werbot/internal/platform/db/dbtest"
	"github.com/werbot/werbot/internal/platform/jobs"
)

func TestGC(t *testing.T) {
	e := gwSetup(t)
	g := e.newPool(t, "edge")
	exec := func(sql string, args ...any) {
		_, err := e.pool.Exec(context.Background(), sql, args...)
		require.NoError(t, err)
	}
	stale := dbtest.Instance(t, e.pool, e.org, g.ID)
	fresh := dbtest.Instance(t, e.pool, e.org, g.ID)
	exec("UPDATE gateway_instance SET cert_expires_at = now() - interval '25 hours' WHERE id = $1", stale)
	exec("UPDATE gateway_instance SET cert_expires_at = now() - interval '23 hours' WHERE id = $1", fresh)
	oldTok, _, _, err := e.gw.CreateJoinToken(e.ctx, g.ID, 0, 0)
	require.NoError(t, err)
	newTok, _, _, err := e.gw.CreateJoinToken(e.ctx, g.ID, 0, 0)
	require.NoError(t, err)
	exec("UPDATE gateway_join_token SET expires_at = now() - interval '8 days' WHERE id = $1", oldTok)
	exec("UPDATE gateway_join_token SET expires_at = now() - interval '6 days' WHERE id = $1", newTok)
	audits := e.count(t, "SELECT count(*) FROM audit_event")

	instances, tokens, err := e.gw.GC(context.Background())
	require.NoError(t, err)
	assert.EqualValues(t, 1, instances)
	assert.EqualValues(t, 1, tokens)
	assert.Equal(t, 0, e.count(t, "SELECT count(*) FROM gateway_instance WHERE id = $1", stale))
	assert.Equal(t, 1, e.count(t, "SELECT count(*) FROM gateway_instance WHERE id = $1", fresh))
	assert.Equal(t, 0, e.count(t, "SELECT count(*) FROM gateway_join_token WHERE id = $1", oldTok))
	assert.Equal(t, 1, e.count(t, "SELECT count(*) FROM gateway_join_token WHERE id = $1", newTok))
	assert.Equal(t, audits, e.count(t, "SELECT count(*) FROM audit_event"), "GC writes no audit")

	assert.Equal(t, "gateway.gc", inventory.GCArgs{}.Kind())
	assert.Equal(t, jobs.QueueMaintenance, inventory.GCArgs{}.InsertOpts().Queue)
	assert.Len(t, inventory.RegisterGatewayJobs(river.NewWorkers(), e.gw), 1)
}
