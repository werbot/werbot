//go:build cluster

package cluster

import (
	"net/http"
	"net/netip"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
)

// TestTopology checks the stand itself: networks, replicas, stream placement,
// SSH through the pool and the client IP chain behind nginx.
func TestTopology(t *testing.T) {
	st.Healthy(t)
	ctx := t.Context()
	public, private := Project()+"_public", Project()+"_private"

	for _, svc := range []string{"server1", "server2", "web1", "web2"} {
		assert.Equal(t, []string{public}, st.Container(t, svc).Networks, svc)
	}
	assert.Equal(t, []string{private}, st.Container(t, "target").Networks, "target")
	for _, svc := range []string{"gw1", "gw2"} {
		assert.Equal(t, []string{private, public}, st.Container(t, svc).Networks, svc)
	}

	assert.Equal(t, http.StatusOK, Ready(st.Addr(t, "web1", 8090)), "web1 /readyz")
	assert.Equal(t, http.StatusOK, Ready(st.Addr(t, "web2", 8090)), "web2 /readyz")
	edgeVer, err := st.Edge.System.GetVersion(ctx, connect.NewRequest(&werbotv1.GetVersionRequest{}))
	require.NoError(t, err)
	uiVer, err := st.UI.System.GetVersion(ctx, connect.NewRequest(&werbotv1.GetVersionRequest{}))
	require.NoError(t, err)
	assert.Equal(t, edgeVer.Msg.GetVersion(), uiVer.Msg.GetVersion())
	assert.Equal(t, edgeVer.Msg.GetCommit(), uiVer.Msg.GetCommit())

	gws, err := st.Edge.Gateways.ListGateways(ctx, connect.NewRequest(&werbotv1.ListGatewaysRequest{}))
	require.NoError(t, err)
	var pool *werbotv1.Gateway
	for _, g := range gws.Msg.GetGateways() {
		if g.GetName() == "pool-a" {
			pool = g
		}
	}
	require.NotNil(t, pool, "pool-a")
	assert.True(t, pool.GetOnline(), "pool-a online")
	assert.Contains(t, pool.GetConnectors(), "ssh")
	var online []string
	for _, in := range pool.GetInstances() {
		if in.GetOnline() {
			online = append(online, in.GetHostname())
		}
	}
	assert.ElementsMatch(t, []string{"gw1", "gw2"}, online)

	assert.Equal(t, int64(1), Metric(t, st.Addr(t, "server1", 9100), "werbot_tunnel_streams"), "server1 streams")
	assert.Equal(t, int64(1), Metric(t, st.Addr(t, "server2", 9100), "werbot_tunnel_streams"), "server2 streams")

	out, code := Exec(t, st.Dial(t, st.Addr(t, "nginx", 2222), "alice+target"), "hostname")
	assert.Equal(t, "target\n", out)
	assert.Equal(t, 0, code)
	_, clientIP, _ := st.LastSession(t, st.Resources["target"])
	assert.True(t, clientIP.IsValid(), "session client_ip")
	assert.NotEqual(t, netip.MustParseAddr(IPNginx), clientIP, "PROXY protocol carries the client, not nginx")

	ip, err := st.Edge.System.GetClientIP(ctx, connect.NewRequest(&werbotv1.GetClientIPRequest{}))
	require.NoError(t, err)
	assert.NotEmpty(t, ip.Msg.GetIp())
	assert.NotEqual(t, IPNginx, ip.Msg.GetIp(), "X-Forwarded-For from the trusted nginx")
}
