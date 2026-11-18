//go:build cluster

package cluster

import (
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
)

// TestScenario01LoginOnAUseOnB: a session created on replica A works on B and
// dies on both after logout on B: the session lives in Postgres, not in memory.
func TestScenario01LoginOnAUseOnB(t *testing.T) {
	st.Healthy(t)
	ctx := t.Context()
	login, err := st.NewAPI(st.A.BaseURL, "").Auth.Login(ctx, connect.NewRequest(&werbotv1.LoginRequest{
		Email: "alice@example.com", Password: "cluster-password-42",
	}))
	require.NoError(t, err)
	cookie := SessionCookie(login.Header())
	require.NotEmpty(t, cookie, "login sets the session cookie")

	onB, onA := st.NewAPI(st.B.BaseURL, cookie), st.NewAPI(st.A.BaseURL, cookie)
	me, err := onB.Auth.GetSession(ctx, connect.NewRequest(&werbotv1.GetSessionRequest{}))
	require.NoError(t, err)
	assert.Equal(t, "alice", me.Msg.GetUser().GetLogin())

	gws, err := onB.Gateways.ListGateways(ctx, connect.NewRequest(&werbotv1.ListGatewaysRequest{}))
	require.NoError(t, err)
	var names []string
	for _, g := range gws.Msg.GetGateways() {
		names = append(names, g.GetName())
	}
	assert.Contains(t, names, "pool-a")

	_, err = onB.Auth.Logout(ctx, connect.NewRequest(&werbotv1.LogoutRequest{}))
	require.NoError(t, err)
	_, err = onA.Auth.GetSession(ctx, connect.NewRequest(&werbotv1.GetSessionRequest{}))
	assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err), "logout on B ends the session on A")
}

// TestScenario02BindingReachesInstanceOnB: a binding made through replica A
// opens target2 on gw2 (server2, replica B) within BindingPropagation.
func TestScenario02BindingReachesInstanceOnB(t *testing.T) {
	st.Healthy(t) // canonical placement: gw2 -> server2
	c := st.Dial(t, st.Addr(t, "nginx", 2202), "alice+target2")
	out, code := Exec(t, c, "hostname")
	require.Equal(t, "Access denied", strings.TrimSpace(out), "denied message ends with CRLF")
	require.Equal(t, 1, code)

	bind := st.createBinding(t, st.A, "target2")
	t.Cleanup(func() { st.deleteBinding(t, bind) })
	t0 := time.Now()
	allowed := func() bool {
		out, code := Exec(t, c, "hostname")
		return out == "target\n" && code == 0
	}
	require.Eventually(t, allowed, BindingPropagation, 50*time.Millisecond)
	t.Logf("propagation %s", time.Since(t0))
}

// TestScenario06ConcurrentBootstrapOnEmptyDB: two replicas started together on
// an empty database leave exactly one CA, one signing key and one migration
// history; the pool trusts the server certificates of both replicas.
func TestScenario06ConcurrentBootstrapOnEmptyDB(t *testing.T) {
	st.Healthy(t)
	ctx := t.Context()
	d := st.StartedAt["server1"].Sub(st.StartedAt["server2"])
	assert.Less(t, d.Abs(), 2*time.Second, "server1 and server2 started together")

	var n int
	for _, table := range []string{"ca_key", "signing_key"} {
		require.NoError(t, st.DB.QueryRow(ctx, "SELECT count(*) FROM "+table).Scan(&n))
		assert.Equal(t, 1, n, table)
	}
	rows, err := st.DB.Query(ctx, `SELECT version_id FROM goose_db_version WHERE version_id > 0 GROUP BY 1 HAVING count(*) > 1`)
	require.NoError(t, err)
	assert.False(t, rows.Next(), "a migration applied twice")
	rows.Close()
	require.NoError(t, rows.Err())

	files, err := filepath.Glob("../../migrations/*.sql")
	require.NoError(t, err)
	require.NotEmpty(t, files)
	require.NoError(t, st.DB.QueryRow(ctx, `SELECT count(DISTINCT version_id) FROM goose_db_version WHERE version_id > 0`).Scan(&n))
	assert.Equal(t, len(files), n, "applied migrations")

	for _, svc := range []string{"server1", "server2"} {
		assert.Equal(t, int64(1), Metric(t, st.Addr(t, svc, 9100), "werbot_tunnel_streams"), svc+" streams")
	}
}

// TestScenario07GeoIPReloadOnEveryReplica: a new geoip_db plus NOTIFY (what
// geoip.refresh does) changes decisions on both replicas without a restart.
func TestScenario07GeoIPReloadOnEveryReplica(t *testing.T) {
	st.Healthy(t)
	ctx := t.Context()
	allow := func(api API) bool {
		res, err := api.Policy.CheckAccess(ctx, connect.NewRequest(&werbotv1.CheckAccessRequest{
			UserId: st.AliceID.String(), ResourceId: st.Resources["geo"].String(), Ip: "203.0.113.7",
		}))
		require.NoError(t, err)
		return res.Msg.GetAllow()
	}
	require.False(t, allow(st.A), "A before")
	require.False(t, allow(st.B), "B before")

	st.publishGeoIP(t, "203.0.113.0/24", "NL")
	t0 := time.Now()
	require.Eventually(t, func() bool { return allow(st.A) && allow(st.B) }, 5*time.Second, 100*time.Millisecond)
	t.Logf("geoip reload %s", time.Since(t0))
}

// TestScenario08PostgresRestart: both replicas and both gateways survive a
// Postgres restart without restarting werbot, and bindings propagate again.
func TestScenario08PostgresRestart(t *testing.T) {
	st.Healthy(t)
	t.Cleanup(func() { st.Healthy(t) })
	ctx := t.Context()
	svcs := []string{"server1", "server2", "gw1", "gw2"}
	before := map[string]time.Time{}
	for _, svc := range svcs {
		before[svc] = st.Container(t, svc).StartedAt
	}

	_, err := st.Compose(ctx, "restart", "postgres")
	require.NoError(t, err)
	_, err = st.Compose(ctx, "up", "-d", "--wait", "postgres")
	require.NoError(t, err)
	t0 := time.Now()
	require.Eventually(t, func() bool {
		return Ready(st.Addr(t, "nginx", 8081)) == http.StatusOK && Ready(st.Addr(t, "nginx", 8082)) == http.StatusOK
	}, PGRecoverMax, 500*time.Millisecond)
	t.Logf("ready again %s after postgres is healthy", time.Since(t0))

	t0 = time.Now()
	bind := st.createBinding(t, st.Edge, "target2")
	t.Cleanup(func() { st.deleteBinding(t, bind) })
	for _, port := range []int{2201, 2202} {
		require.Eventually(t, func() bool {
			c, err := st.dial(st.Addr(t, "nginx", port), "alice+target2")
			if err != nil {
				return false
			}
			defer c.Close()
			out, code, err := run(c, "hostname")
			return err == nil && out == "target\n" && code == 0
		}, ListenRecoverMax, 500*time.Millisecond, "ssh via :%d", port)
		t.Logf("binding visible via :%d after %s", port, time.Since(t0))
	}

	for _, svc := range svcs {
		c := st.Container(t, svc)
		assert.True(t, c.Running, svc)
		assert.True(t, before[svc].Equal(c.StartedAt), "%s restarted: %s -> %s", svc, before[svc], c.StartedAt)
	}
}
