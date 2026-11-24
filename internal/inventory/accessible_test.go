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

// fakeRequestable marks the listed ids requestable and records its calls.
type fakeRequestable struct {
	ids   []uuid.UUID
	calls [][]uuid.UUID
	user  uuid.UUID
}

func (f *fakeRequestable) Requestable(_ context.Context, p principal.Principal, ids []uuid.UUID) (map[uuid.UUID]bool, error) {
	f.calls, f.user = append(f.calls, ids), p.UserID
	out := map[uuid.UUID]bool{}
	for _, id := range f.ids {
		out[id] = true
	}
	return out, nil
}

func TestListAccessibleResources(t *testing.T) {
	e := gwSetup(t)
	pool := e.newPool(t, "edge")
	_, err := e.gw.Update(e.ctx, pool.ID, "gw.example.com:2222", "")
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
	h := inventory.NewHandler(e.svc, fakeCreds{}, fakeAccess{[]uuid.UUID{inPool.ID, foreign.ID}}, e.gw, &fakeRequestable{})

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
	assert.True(t, r.GetAllowed())
	assert.False(t, r.GetRequestable())
	assert.Equal(t, "gw.example.com:2222", r.GetSshAddress())
	assert.Equal(t, strings.TrimSpace(string(gossh.MarshalAuthorizedKey(e.poolKey(t, pool.ID).PublicKey()))), r.GetHostKey())
}

func TestListAccessibleResourcesNoPool(t *testing.T) {
	e := gwSetup(t)
	r := e.create(t, "solo", nil)
	h := inventory.NewHandler(e.svc, fakeCreds{}, fakeAccess{[]uuid.UUID{r.ID}}, e.gw, &fakeRequestable{})
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
	h := inventory.NewHandler(e.svc, fakeCreds{}, fakeAccess{}, e.gw, &fakeRequestable{})

	_, err := h.ListAccessibleResources(mctx, connect.NewRequest(&werbotv1.ListAccessibleResourcesRequest{}))
	require.NoError(t, err)
	_, err = h.ListResources(mctx, connect.NewRequest(&werbotv1.ListResourcesRequest{}))
	assert.Equal(t, connect.CodePermissionDenied, connect.CodeOf(err))
	_, err = h.ListAccessibleResources(context.Background(), connect.NewRequest(&werbotv1.ListAccessibleResourcesRequest{}))
	assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}

// Resources are listed when allowed or requestable; Requestable runs once over every resource of the org.
func TestListAccessibleResourcesRequestable(t *testing.T) {
	e := gwSetup(t)
	r1, r2, r3 := e.create(t, "r1", nil), e.create(t, "r2", nil), e.create(t, "r3", nil)
	member := dbtest.User(t, e.pool, e.org, "member")
	mctx := principal.With(context.Background(), principal.Principal{UserID: member, OrgID: e.org, Role: "member"})
	req := &fakeRequestable{ids: []uuid.UUID{r1.ID}}
	h := inventory.NewHandler(e.svc, fakeCreds{}, fakeAccess{[]uuid.UUID{r2.ID}}, e.gw, req)

	res, err := h.ListAccessibleResources(mctx, connect.NewRequest(&werbotv1.ListAccessibleResourcesRequest{}))
	require.NoError(t, err)
	got := map[string][2]bool{}
	for _, r := range res.Msg.GetResources() {
		got[r.GetName()] = [2]bool{r.GetAllowed(), r.GetRequestable()}
	}
	assert.Equal(t, map[string][2]bool{"r1": {false, true}, "r2": {true, false}}, got, "r3 is neither")
	require.Len(t, req.calls, 1)
	assert.ElementsMatch(t, []uuid.UUID{r1.ID, r2.ID, r3.ID}, req.calls[0])
	assert.Equal(t, member, req.user)
}
