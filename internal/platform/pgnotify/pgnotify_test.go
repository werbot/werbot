package pgnotify_test

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/platform/db/dbtest"
	"github.com/werbot/werbot/internal/platform/pgnotify"
)

func recv(t *testing.T, ch <-chan string) string {
	t.Helper()
	select {
	case p := <-ch:
		return p
	case <-time.After(5 * time.Second):
		t.Fatal("notification not delivered")
		return ""
	}
}

func TestListenDeliversAndReconnects(t *testing.T) {
	pool, _ := dbtest.New(t)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	org, geo := make(chan string, 8), make(chan string, 8)
	var reconnects atomic.Int32
	done := make(chan error, 1)
	go func() {
		done <- pgnotify.Listen(ctx, pool, map[string]func(string){
			pgnotify.OrgChanged:   func(p string) { org <- p },
			pgnotify.GeoIPChanged: func(p string) { geo <- p },
		}, func() { reconnects.Add(1) })
	}()
	waitReconnects := func(n int32, within time.Duration) {
		t.Helper()
		require.Eventually(t, func() bool { return reconnects.Load() == n }, within, 20*time.Millisecond)
	}
	waitReconnects(1, 5*time.Second)

	// pg_stat_activity is cluster-wide: look at this database only.
	const listenPID = `SELECT pid FROM pg_stat_activity WHERE datname = current_database() AND query LIKE 'LISTEN%'`
	var n int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM (`+listenPID+`) s`).Scan(&n))
	require.Equal(t, 1, n)
	require.Zero(t, pool.Stat().AcquiredConns(), "listen connection must live outside the pool")

	orgID := dbtest.Org(t, pool)
	insertTeam := func(name string) {
		t.Helper()
		_, err := pool.Exec(ctx, `INSERT INTO team (org_id, name) VALUES ($1, $2)`, orgID, name)
		require.NoError(t, err)
	}
	insertTeam("a")
	require.Equal(t, orgID.String(), recv(t, org))

	_, err := pool.Exec(ctx, `INSERT INTO ip_blocklist (cidr, source) VALUES ('10.0.0.0/8', 'test')`)
	require.NoError(t, err)
	require.Equal(t, "*", recv(t, org))
	_, err = pool.Exec(ctx, `SELECT pg_notify('werbot_geoip_changed', '')`)
	require.NoError(t, err)
	require.Equal(t, "", recv(t, geo))

	var pid int
	require.NoError(t, pool.QueryRow(ctx, listenPID).Scan(&pid))
	_, err = pool.Exec(ctx, `SELECT pg_terminate_backend($1)`, pid)
	require.NoError(t, err)
	waitReconnects(2, 10*time.Second)

	insertTeam("b")
	require.Equal(t, orgID.String(), recv(t, org))
	_, err = pool.Exec(ctx, `SELECT pg_notify('werbot_geoip_changed', '')`)
	require.NoError(t, err)
	recv(t, geo)

	cancel()
	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("Listen did not return after cancel")
	}
}
