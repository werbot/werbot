package inventory_test

import (
	"context"
	"strings"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	gossh "golang.org/x/crypto/ssh"

	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
	"github.com/werbot/werbot/internal/platform/apperr"
)

func TestUpdateGateway(t *testing.T) {
	e := gwRPCSetup(t)
	g := e.newPool(t, "edge")
	update := func(c interface {
		UpdateGateway(context.Context, *connect.Request[werbotv1.UpdateGatewayRequest]) (*connect.Response[werbotv1.UpdateGatewayResponse], error)
	}, addr string) error {
		_, err := c.UpdateGateway(context.Background(), connect.NewRequest(&werbotv1.UpdateGatewayRequest{Id: g.ID.String(), SshAddress: addr}))
		return err
	}
	list := func() *werbotv1.Gateway {
		res, err := e.admin.ListGateways(context.Background(), connect.NewRequest(&werbotv1.ListGatewaysRequest{}))
		require.NoError(t, err)
		require.Len(t, res.Msg.GetGateways(), 1)
		return res.Msg.GetGateways()[0]
	}

	require.NoError(t, update(e.admin, "gw.example.com:2222"))
	got := list()
	assert.Equal(t, "gw.example.com:2222", got.GetSshAddress())
	assert.Regexp(t, "^ssh-ed25519 ", got.GetHostKey())
	assert.Equal(t, strings.TrimSpace(string(gossh.MarshalAuthorizedKey(e.poolKey(t, g.ID).PublicKey()))), got.GetHostKey())
	assert.Equal(t, 1, e.count(t, `SELECT count(*) FROM audit_event WHERE action = 'gateway.update' AND target_id = $1
		AND actor_user_id = $2 AND data = '{"ssh_address": "gw.example.com:2222", "ws_url": ""}'::jsonb`, g.ID.String(), e.owner))

	require.NoError(t, update(e.admin, "[::1]:22"))
	require.NoError(t, update(e.admin, ""))
	assert.Empty(t, list().GetSshAddress(), "empty resets")

	for _, bad := range []string{"gw", "gw:0", "gw:70000", "a b:22", ":22", "gw:x"} {
		err := update(e.admin, bad)
		assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err), "%q", bad)
	}
	assert.Equal(t, connect.CodePermissionDenied, connect.CodeOf(update(e.member, "gw:22")))

	_, err := e.admin.UpdateGateway(context.Background(), connect.NewRequest(&werbotv1.UpdateGatewayRequest{Id: "x"}))
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err), "validation")
	otherCtx, _ := e.otherOrg(t)
	_, err = e.gw.Update(otherCtx, g.ID, "gw:22", "")
	assert.ErrorIs(t, err, apperr.ErrNotFound, "pool of another org")
}
