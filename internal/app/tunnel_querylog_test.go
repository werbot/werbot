package app

import (
	"context"
	"log/slog"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/platform/db/dbtest"
	"github.com/werbot/werbot/internal/platform/metrics"
	"github.com/werbot/werbot/internal/tunnel"
)

// The adapters store a postgres session's database in session.start, begin
// its querylog recording and keep the query count of its end.
func TestTunnelSessionsQuerylog(t *testing.T) {
	s := newStage4(t, slog.New(slog.DiscardHandler))
	f := dbtest.Seed(t, s.pool)
	ctx := context.Background()
	_, err := s.pool.Exec(ctx, "UPDATE resource SET type = 'postgres' WHERE id = $1", f.ResourceID)
	require.NoError(t, err)
	d := newTunnelDeps(s.d, metrics.NewRegistry(), slog.New(slog.DiscardHandler))
	id, err := d.Sessions.Start(ctx, tunnel.SessionStart{OrgID: f.OrgID, GatewayID: f.GatewayID, InstanceID: f.InstanceID,
		UserID: f.MemberID, ResourceID: f.ResourceID, ClientIP: netip.MustParseAddr("10.0.0.1"), Kind: "stream", DBName: "app"})
	require.NoError(t, err)
	require.NoError(t, d.Recordings.Begin(ctx, f.OrgID, id))

	var dbName, format string
	require.NoError(t, s.pool.QueryRow(ctx, `SELECT data->>'db_name' FROM audit_event
		WHERE action = 'session.start' AND data->>'session_id' = $1`, id.String()).Scan(&dbName))
	require.NoError(t, s.pool.QueryRow(ctx, "SELECT format FROM recording WHERE session_id = $1", id).Scan(&format))
	assert.Equal(t, [2]string{"app", "querylog"}, [2]string{dbName, format})

	require.NoError(t, d.Sessions.End(ctx, f.OrgID, f.InstanceID, id, "closed", nil, &tunnel.SessionResult{Queries: 5}))
	var queries int64
	require.NoError(t, s.pool.QueryRow(ctx, "SELECT queries FROM session WHERE id = $1", id).Scan(&queries))
	assert.Equal(t, int64(5), queries)
}
