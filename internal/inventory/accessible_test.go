package inventory_test

import (
	"context"
	"net/netip"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	gossh "golang.org/x/crypto/ssh"

	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
	"github.com/werbot/werbot/internal/connector"
	"github.com/werbot/werbot/internal/inventory"
	"github.com/werbot/werbot/internal/platform/db/dbtest"
	"github.com/werbot/werbot/internal/platform/principal"
)

// fakeAccess allows the listed ids, whatever their organization.
type fakeAccess struct{ ids []uuid.UUID }

func (f fakeAccess) AccessibleResources(context.Context, uuid.UUID, uuid.UUID, netip.Addr, time.Time) ([]uuid.UUID, error) {
	return f.ids, nil
}

func TestListAccessibleResources(t *testing.T) {
	e := gwSetup(t)
	pool := e.newPool(t, "edge")
	_, err := e.gw.Update(e.ctx, pool.ID, "gw.example.com:2222")
	require.NoError(t, err)
	inPool, err := e.svc.Create(e.ctx, e.org, inventory.CreateInput{Name: "b-web", Type: "ssh", Config: sshCfg("h"), GatewayID: &pool.ID,
		Labels: map[string]string{"env": "prod"}})
	require.NoError(t, err)
	e.create(t, "c-hidden", nil)
	foreignCtx, foreignOrg := e.otherOrg(t)
	foreign, err := e.svc.Create(foreignCtx, foreignOrg, inventory.CreateInput{Name: "a-foreign", Type: "ssh", Config: sshCfg("h")})
	require.NoError(t, err)

	member := dbtest.User(t, e.pool, e.org, "member")
	mctx := principal.With(context.Background(), principal.Principal{UserID: member, OrgID: e.org, Role: "member"})
	h := inventory.NewHandler(e.svc, fakeCreds{}, fakeAccess{[]uuid.UUID{inPool.ID, foreign.ID}}, e.gw)

	res, err := h.ListAccessibleResources(mctx, connect.NewRequest(&werbotv1.ListAccessibleResourcesRequest{}))
	require.NoError(t, err)
	require.Len(t, res.Msg.GetResources(), 1, "hidden and foreign-org resources are not listed")
	r := res.Msg.GetResources()[0]
	assert.Equal(t, inPool.ID.String(), r.GetId())
	assert.Equal(t, "b-web", r.GetName())
	assert.Equal(t, "ssh", r.GetType())
	typ, _ := connector.Lookup("ssh")
	assert.Len(t, r.GetKinds(), len(typ.Kinds))
	assert.Equal(t, map[string]string{"env": "prod"}, r.GetLabels())
	assert.Equal(t, "gw.example.com:2222", r.GetSshAddress())
	assert.Equal(t, strings.TrimSpace(string(gossh.MarshalAuthorizedKey(e.poolKey(t, pool.ID).PublicKey()))), r.GetHostKey())
}

func TestListAccessibleResourcesNoPool(t *testing.T) {
	e := gwSetup(t)
	r := e.create(t, "solo", nil)
	h := inventory.NewHandler(e.svc, fakeCreds{}, fakeAccess{[]uuid.UUID{r.ID}}, e.gw)
	res, err := h.ListAccessibleResources(e.ctx, connect.NewRequest(&werbotv1.ListAccessibleResourcesRequest{}))
	require.NoError(t, err)
	require.Len(t, res.Msg.GetResources(), 1)
	assert.Empty(t, res.Msg.GetResources()[0].GetSshAddress())
	assert.Empty(t, res.Msg.GetResources()[0].GetHostKey())
}

func TestListAccessibleResourcesMemberOnly(t *testing.T) {
	e := gwSetup(t)
	member := dbtest.User(t, e.pool, e.org, "member")
	mctx := principal.With(context.Background(), principal.Principal{UserID: member, OrgID: e.org, Role: "member"})
	h := inventory.NewHandler(e.svc, fakeCreds{}, fakeAccess{}, e.gw)

	_, err := h.ListAccessibleResources(mctx, connect.NewRequest(&werbotv1.ListAccessibleResourcesRequest{}))
	require.NoError(t, err)
	_, err = h.ListResources(mctx, connect.NewRequest(&werbotv1.ListResourcesRequest{}))
	assert.Equal(t, connect.CodePermissionDenied, connect.CodeOf(err))
	_, err = h.ListAccessibleResources(context.Background(), connect.NewRequest(&werbotv1.ListAccessibleResourcesRequest{}))
	assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}
