package app

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

// Each pool snapshot resource carries its configuration version, the
// resource's updated_at in microseconds, which GetProbeCredential checks
// (Ruling C16): a configuration change moves it.
func TestSnapshotCarriesConfigVersion(t *testing.T) {
	s := newAgentStage(t)
	ctx := context.Background()
	pg := s.pg(t)
	version := func() (snap, db int64) {
		t.Helper()
		got, err := buildSnapshot(ctx, s.d.Snapshots, s.f.OrgID, s.f.GatewayID)
		require.NoError(t, err)
		for _, r := range got.GetResources() {
			if r.GetId() == pg.String() {
				snap = r.GetConfigVersion()
			}
		}
		require.NoError(t, s.pool.QueryRow(ctx, "SELECT (extract(epoch FROM updated_at) * 1000000)::bigint FROM resource WHERE id = $1", pg).Scan(&db))
		return snap, db
	}
	snap, db := version()
	require.Equal(t, db, snap)
	s.exec(t, "UPDATE resource SET updated_at = now() + interval '1 minute' WHERE id = $1", pg)
	moved, db := version()
	require.Equal(t, db, moved)
	require.Greater(t, moved, snap)
}
