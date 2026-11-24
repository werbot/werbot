package inventory_test

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
	"github.com/werbot/werbot/internal/inventory"
	"github.com/werbot/werbot/internal/platform/db/dbtest"
	"github.com/werbot/werbot/internal/platform/principal"
)

// An accessible resource carries the ws_url of its pool; a pool without one,
// or a resource without a pool, gets "".
func TestAccessibleResourcesWSURL(t *testing.T) {
	e := gwSetup(t)
	withURL, without := e.newPool(t, "edge"), e.newPool(t, "plain")
	_, err := e.gw.Update(e.ctx, withURL.ID, "gw.example.com:2222", "wss://gw.example.com/v1/ssh")
	require.NoError(t, err)
	a, err := e.svc.Create(e.ctx, e.org, inventory.CreateInput{Name: "a-ws", Type: "ssh", Config: sshCfg("h"), GatewayID: &withURL.ID})
	require.NoError(t, err)
	b, err := e.svc.Create(e.ctx, e.org, inventory.CreateInput{Name: "b-plain", Type: "ssh", Config: sshCfg("h"), GatewayID: &without.ID})
	require.NoError(t, err)

	member := dbtest.User(t, e.pool, e.org, "member")
	mctx := principal.With(context.Background(), principal.Principal{UserID: member, OrgID: e.org, Role: "member"})
	h := inventory.NewHandler(e.svc, fakeCreds{}, fakeAccess{[]uuid.UUID{a.ID, b.ID}}, e.gw, &fakeRequestable{})
	res, err := h.ListAccessibleResources(mctx, connect.NewRequest(&werbotv1.ListAccessibleResourcesRequest{}))
	require.NoError(t, err)
	got := map[string]string{}
	for _, r := range res.Msg.GetResources() {
		got[r.GetName()] = r.GetWsUrl()
	}
	assert.Equal(t, map[string]string{"a-ws": "wss://gw.example.com/v1/ssh", "b-plain": ""}, got)
}
