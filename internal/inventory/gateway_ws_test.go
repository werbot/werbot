package inventory_test

import (
	"context"
	"strings"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
)

// UpdateGateway replaces both addresses on every call, validates ws_url, is
// owner/admin only and audits both addresses.
func TestUpdateGatewayWSURL(t *testing.T) {
	e := gwRPCSetup(t)
	g := e.newPool(t, "edge")
	update := func(c interface {
		UpdateGateway(context.Context, *connect.Request[werbotv1.UpdateGatewayRequest]) (*connect.Response[werbotv1.UpdateGatewayResponse], error)
	}, ssh, ws string) error {
		_, err := c.UpdateGateway(context.Background(), connect.NewRequest(&werbotv1.UpdateGatewayRequest{Id: g.ID.String(), SshAddress: ssh, WsUrl: ws}))
		return err
	}
	list := func() *werbotv1.Gateway {
		res, err := e.admin.ListGateways(context.Background(), connect.NewRequest(&werbotv1.ListGatewaysRequest{}))
		require.NoError(t, err)
		require.Len(t, res.Msg.GetGateways(), 1)
		return res.Msg.GetGateways()[0]
	}

	require.NoError(t, update(e.admin, "gw.example.com:2222", "wss://gw.example.com/v1/ssh"))
	got := list()
	assert.Equal(t, "wss://gw.example.com/v1/ssh", got.GetWsUrl())
	assert.Equal(t, "gw.example.com:2222", got.GetSshAddress())
	assert.Equal(t, 1, e.count(t, `SELECT count(*) FROM audit_event WHERE action = 'gateway.update' AND target_id = $1
		AND data = '{"ssh_address": "gw.example.com:2222", "ws_url": "wss://gw.example.com/v1/ssh"}'::jsonb`, g.ID.String()))

	require.NoError(t, update(e.admin, "", "ws://gw.example.com:8080"))
	got = list()
	assert.Empty(t, got.GetSshAddress(), "both addresses are replaced")
	assert.Equal(t, "ws://gw.example.com:8080", got.GetWsUrl())
	require.NoError(t, update(e.admin, "", ""))
	assert.Empty(t, list().GetWsUrl(), "empty resets")

	for _, bad := range []string{"http://x", "wss://", "wss://u:p@h", "wss://h/?a=1", "wss://h/?", "wss://h/#f", "wss://h/a%20b", "wss://h/a b",
		"gw.example.com", "WSS://h/v1/ssh", "Ws://h", "wss://h:70000", "wss://h:0", "wss://h/" + strings.Repeat("a", 2048)} {
		err := update(e.admin, "", bad)
		assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err), "%q", bad)
	}
	assert.Equal(t, connect.CodePermissionDenied, connect.CodeOf(update(e.member, "", "wss://h/v1/ssh")))
	assert.Equal(t, 3, e.count(t, "SELECT count(*) FROM audit_event WHERE action = 'gateway.update' AND target_id = $1", g.ID.String()),
		"refused updates write no audit row")
}
