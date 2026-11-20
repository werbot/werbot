package pgnotify_test

import (
	"context"
	"net"
	"os"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
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
	var reconnects, losses atomic.Int32
	done := make(chan error, 1)
	go func() {
		done <- pgnotify.Listen(ctx, pool, map[string]func(string){
			pgnotify.OrgChanged:   func(p string) { org <- p },
			pgnotify.GeoIPChanged: func(p string) { geo <- p },
		}, func() { reconnects.Add(1) }, func() { losses.Add(1) })
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
	require.Equal(t, orgID.String(), recv(t, org), "org_setting insert")
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
	require.Equal(t, int32(1), losses.Load(), "a dropped connection is reported before the reconnect")

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
	require.Equal(t, int32(1), losses.Load(), "cancel is not a lost connection")
}

// stallConn blocks Write once stall is set, until a deadline is set on the socket.
type stallConn struct {
	net.Conn
	stall    *atomic.Bool
	released chan struct{}
	once     sync.Once
}

func (c *stallConn) release() { c.once.Do(func() { close(c.released) }) }

func (c *stallConn) SetDeadline(t time.Time) error { c.release(); return c.Conn.SetDeadline(t) }
func (c *stallConn) SetWriteDeadline(t time.Time) error {
	c.release()
	return c.Conn.SetWriteDeadline(t)
}

func (c *stallConn) Write(b []byte) (int, error) {
	if c.stall.Load() {
		<-c.released
		return 0, os.ErrDeadlineExceeded
	}
	return c.Conn.Write(b)
}

func TestSessionCloseIsBounded(t *testing.T) {
	pool, _ := dbtest.New(t)
	var stall atomic.Bool
	cfg := pool.Config().ConnConfig.Copy()
	cfg.DialFunc = func(ctx context.Context, network, addr string) (net.Conn, error) {
		c, err := (&net.Dialer{}).DialContext(ctx, network, addr)
		if err != nil {
			return nil, err
		}
		return &stallConn{Conn: c, stall: &stall, released: make(chan struct{})}, nil
	}
	conn, err := pgx.ConnectConfig(t.Context(), cfg)
	require.NoError(t, err)

	stall.Store(true)
	done := make(chan struct{})
	go func() {
		pgnotify.CloseConnWithTimeout(t.Context(), conn, 100*time.Millisecond)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("close hung on a stalled socket")
	}
}

func TestOrgChangedTriggers(t *testing.T) {
	pool, _ := dbtest.New(t)
	ctx := t.Context()

	rows, err := pool.Query(ctx, `SELECT tgrelid::regclass::text FROM pg_trigger WHERE tgfoid = 'werbot_notify_org_changed'::regproc AND NOT tgisinternal ORDER BY 1`)
	require.NoError(t, err)
	got, err := pgx.CollectRows(rows, pgx.RowTo[string])
	require.NoError(t, err)
	want := []string{"access_binding", "access_grant", "command_class_rule", "command_rule", "custom_policy", "gateway", "gateway_instance", "gateway_instance", "ip_blocklist", "lock", "membership", "org_setting", "resource", "resource_country_rule", "resource_label", "resource_network_rule", "resource_schedule", "team", "team_member"}
	require.True(t, slices.Equal(want, got), "triggered tables = %v", got)

	conn, err := pgx.ConnectConfig(ctx, pool.Config().ConnConfig.Copy())
	require.NoError(t, err)
	defer func() { _ = conn.Close(context.WithoutCancel(ctx)) }()
	_, err = conn.Exec(ctx, "LISTEN "+pgnotify.OrgChanged)
	require.NoError(t, err)

	orgID := dbtest.Org(t, pool)
	var teamID string
	require.NoError(t, pool.QueryRow(ctx, `INSERT INTO team (org_id, name) VALUES ($1, 't') RETURNING id`, orgID).Scan(&teamID))
	_, err = pool.Exec(ctx, `DELETE FROM team WHERE id = $1`, teamID)
	require.NoError(t, err)

	for _, step := range []string{"org_setting insert", "insert", "delete"} {
		wctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		n, err := conn.WaitForNotification(wctx)
		cancel()
		require.NoError(t, err, step)
		require.Equal(t, orgID.String(), n.Payload, step)
	}
}
