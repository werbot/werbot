package inventory_test

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/wrapperspb"

	sshv1 "github.com/werbot/werbot/gen/werbot/connector/ssh/v1"
	"github.com/werbot/werbot/internal/connector"
	_ "github.com/werbot/werbot/internal/connector/ssh"
	"github.com/werbot/werbot/internal/inventory"
	"github.com/werbot/werbot/internal/platform/apperr"
	"github.com/werbot/werbot/internal/platform/db/dbtest"
	"github.com/werbot/werbot/internal/platform/principal"
)

type env struct {
	svc   *inventory.Service
	pool  *pgxpool.Pool
	ctx   context.Context
	org   uuid.UUID
	owner uuid.UUID
}

func setup(t *testing.T) env {
	t.Helper()
	pool, _ := dbtest.New(t)
	org := dbtest.Org(t, pool)
	owner := dbtest.User(t, pool, org, "owner")
	ctx := principal.With(context.Background(), principal.Principal{UserID: owner, OrgID: org, Role: "owner"})
	return env{inventory.New(pool), pool, ctx, org, owner}
}

func sshCfg(host string) *sshv1.Config { return &sshv1.Config{Host: host, Login: "root"} }

func (e env) create(t *testing.T, name string, labels map[string]string) inventory.Resource {
	t.Helper()
	r, err := e.svc.Create(e.ctx, e.org, inventory.CreateInput{Name: name, Type: "ssh", Config: sshCfg("h"), Labels: labels})
	require.NoError(t, err)
	return r
}

func TestCreateGetRoundTrip(t *testing.T) {
	e := setup(t)
	r, err := e.svc.Create(e.ctx, e.org, inventory.CreateInput{Name: "web-1", Type: "ssh",
		Config: sshCfg("10.0.0.5"), Labels: map[string]string{"env": "prod"}})
	require.NoError(t, err)
	got, err := e.svc.Get(e.ctx, e.org, r.ID)
	require.NoError(t, err)
	assert.Equal(t, connector.CategoryServers, got.Category)
	assert.True(t, proto.Equal(&sshv1.Config{Host: "10.0.0.5", Port: 22, Login: "root"}, got.Config))
	assert.Equal(t, map[string]string{"env": "prod"}, got.Labels)
	assert.Nil(t, got.HostKey)

	var n int
	require.NoError(t, e.pool.QueryRow(context.Background(),
		"SELECT count(*) FROM audit_event WHERE action = 'resource.create' AND target_id = $1 AND actor_user_id = $2",
		r.ID.String(), e.owner).Scan(&n))
	assert.Equal(t, 1, n)
}

func TestCreateRejects(t *testing.T) {
	e := setup(t)
	good := sshCfg("h")
	for name, in := range map[string]inventory.CreateInput{
		"bad name":    {Name: "Web_1", Type: "ssh", Config: good},
		"bad type":    {Name: "a", Type: "nope", Config: good},
		"bad config":  {Name: "a", Type: "ssh", Config: &sshv1.Config{Host: "h", Port: 70000, Login: "root"}},
		"wrong proto": {Name: "a", Type: "ssh", Config: wrapperspb.String("x")},
		"nil config":  {Name: "a", Type: "ssh"},
		"bad label":   {Name: "a", Type: "ssh", Config: good, Labels: map[string]string{"Env": "x"}},
		"bad value":   {Name: "a", Type: "ssh", Config: good, Labels: map[string]string{"env": "a b"}},
	} {
		_, err := e.svc.Create(e.ctx, e.org, in)
		require.ErrorIs(t, err, apperr.ErrInvalidArgument, name)
		assert.Equal(t, 1, strings.Count(err.Error(), apperr.ErrInvalidArgument.Error()), "single prefix: %s", err)
	}
	many := map[string]string{}
	for i := range 33 {
		many["k"+string(rune('a'+i%26))+string(rune('a'+i/26))] = "v"
	}
	_, err := e.svc.Create(e.ctx, e.org, inventory.CreateInput{Name: "a", Type: "ssh", Config: good, Labels: many})
	assert.ErrorIs(t, err, apperr.ErrInvalidArgument)
}

func TestCreateDuplicateName(t *testing.T) {
	e := setup(t)
	e.create(t, "web", nil)
	_, err := e.svc.Create(e.ctx, e.org, inventory.CreateInput{Name: "web", Type: "ssh", Config: sshCfg("h")})
	assert.ErrorIs(t, err, apperr.ErrAlreadyExists)
}

func TestUpdateAndDelete(t *testing.T) {
	e := setup(t)
	r := e.create(t, "web", map[string]string{"env": "prod"})
	g := uuid.New()
	u, err := e.svc.Update(e.ctx, e.org, inventory.UpdateInput{ID: r.ID, Name: "web2", GatewayID: &g, Labels: map[string]string{"tier": "db"}})
	require.NoError(t, err)
	assert.Equal(t, "web2", u.Name)
	assert.Equal(t, &g, u.GatewayID)
	assert.Equal(t, map[string]string{"tier": "db"}, u.Labels)
	assert.Equal(t, "h", u.Config.(*sshv1.Config).Host, "nil config keeps the old one")

	u, err = e.svc.Update(e.ctx, e.org, inventory.UpdateInput{ID: r.ID, Name: "web2", Config: sshCfg("h2")})
	require.NoError(t, err)
	assert.Equal(t, "h2", u.Config.(*sshv1.Config).Host)
	assert.Nil(t, u.GatewayID)
	assert.Empty(t, u.Labels)

	_, err = e.svc.Update(e.ctx, e.org, inventory.UpdateInput{ID: r.ID, Name: "web2", Config: wrapperspb.String("x")})
	assert.ErrorIs(t, err, apperr.ErrInvalidArgument)
	_, err = e.svc.Update(e.ctx, e.org, inventory.UpdateInput{ID: r.ID, Name: "Bad_Name"})
	require.ErrorIs(t, err, apperr.ErrInvalidArgument)
	assert.Equal(t, 1, strings.Count(err.Error(), apperr.ErrInvalidArgument.Error()), "single prefix: %s", err)

	require.NoError(t, e.svc.Delete(e.ctx, e.org, r.ID))
	_, err = e.svc.Get(e.ctx, e.org, r.ID)
	assert.ErrorIs(t, err, apperr.ErrNotFound)
}

func TestListLabelFilter(t *testing.T) {
	e := setup(t)
	e.create(t, "a", map[string]string{"env": "prod", "tier": "db"})
	e.create(t, "b", map[string]string{"env": "prod"})
	e.create(t, "c", map[string]string{"env": "dev"})
	for _, tc := range []struct {
		filter map[string]string
		want   int
	}{
		{map[string]string{"env": "prod"}, 2},
		{map[string]string{"env": "prod", "tier": "db"}, 1},
		{nil, 3},
	} {
		got, err := e.svc.List(e.ctx, e.org, tc.filter)
		require.NoError(t, err)
		assert.Len(t, got, tc.want, "%v", tc.filter)
	}
	all, _ := e.svc.List(e.ctx, e.org, nil)
	assert.Equal(t, "a", all[0].Name)
}

func TestCrossOrgInvisible(t *testing.T) {
	e := setup(t)
	r := e.create(t, "web", nil)
	orgB := dbtest.Org(t, e.pool)
	ownerB := dbtest.User(t, e.pool, orgB, "owner")
	ctx := principal.With(context.Background(), principal.Principal{UserID: ownerB, OrgID: orgB, Role: "owner"})

	_, err := e.svc.Get(ctx, orgB, r.ID)
	assert.ErrorIs(t, err, apperr.ErrNotFound)
	_, err = e.svc.Update(ctx, orgB, inventory.UpdateInput{ID: r.ID, Name: "x"})
	assert.ErrorIs(t, err, apperr.ErrNotFound)
	assert.ErrorIs(t, e.svc.Delete(ctx, orgB, r.ID), apperr.ErrNotFound)
	assert.ErrorIs(t, e.svc.ClearHostKey(ctx, orgB, r.ID), apperr.ErrNotFound)
	assert.ErrorIs(t, e.svc.PinHostKey(ctx, orgB, r.ID, newKey(t)), apperr.ErrNotFound)
	list, err := e.svc.List(ctx, orgB, nil)
	require.NoError(t, err)
	assert.Empty(t, list)
}

func TestResourcesForGateway(t *testing.T) {
	e := setup(t)
	g1 := uuid.New()
	a := e.create(t, "a", nil)
	e.create(t, "b", nil)
	_, err := e.svc.Update(e.ctx, e.org, inventory.UpdateInput{ID: a.ID, Name: "a", GatewayID: &g1})
	require.NoError(t, err)
	got, err := e.svc.ResourcesForGateway(e.ctx, e.org, g1)
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, a.ID, got[0].ID)
}

func newKey(t *testing.T) ssh.PublicKey {
	t.Helper()
	pub, _, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	k, err := ssh.NewPublicKey(pub)
	require.NoError(t, err)
	return k
}

func TestPinHostKeyTOFU(t *testing.T) {
	e := setup(t)
	r := e.create(t, "web", nil)
	k1, k2 := newKey(t), newKey(t)
	require.NoError(t, e.svc.PinHostKey(e.ctx, e.org, r.ID, k1))
	require.NoError(t, e.svc.PinHostKey(e.ctx, e.org, r.ID, k1))
	assert.ErrorIs(t, e.svc.PinHostKey(e.ctx, e.org, r.ID, k2), apperr.ErrFailedPrecondition)
	require.NoError(t, e.svc.ClearHostKey(e.ctx, e.org, r.ID))
	require.NoError(t, e.svc.PinHostKey(e.ctx, e.org, r.ID, k2))
	got, err := e.svc.Get(e.ctx, e.org, r.ID)
	require.NoError(t, err)
	require.NotNil(t, got.HostKey)
	assert.Equal(t, ssh.FingerprintSHA256(k2), got.HostKey.FingerprintSHA256)
	assert.Equal(t, k2.Type(), got.HostKey.KeyType)

	var n int
	require.NoError(t, e.pool.QueryRow(context.Background(),
		"SELECT count(*) FROM audit_event WHERE action IN ('resource.host_key.pin','resource.host_key.clear') AND target_id = $1",
		r.ID.String()).Scan(&n))
	assert.Equal(t, 3, n)
}
