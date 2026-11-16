package inventory_test

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	gossh "golang.org/x/crypto/ssh"

	"github.com/werbot/werbot/internal/inventory"
	"github.com/werbot/werbot/internal/platform/apperr"
	"github.com/werbot/werbot/internal/platform/db/dbtest"
	"github.com/werbot/werbot/internal/platform/principal"
)

func TestCreateGateway(t *testing.T) {
	e := gwSetup(t)
	g, err := e.gw.Create(e.ctx, "edge")
	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, g.ID)
	assert.Equal(t, e.org, g.OrgID)
	assert.Equal(t, "edge", g.Name)
	assert.False(t, g.Online)
	assert.Empty(t, g.Connectors)
	assert.Empty(t, g.Instances)
	assert.Equal(t, gossh.KeyAlgoED25519, e.poolKey(t, g.ID).PublicKey().Type())
	assert.Equal(t, 1, e.count(t,
		"SELECT count(*) FROM audit_event WHERE action = 'gateway.create' AND target_id = $1 AND actor_user_id = $2",
		g.ID.String(), e.owner))

	_, err = e.gw.Create(e.ctx, "edge")
	assert.ErrorIs(t, err, apperr.ErrAlreadyExists)
	for _, name := range []string{"", strings.Repeat("a", 64)} {
		_, err = e.gw.Create(e.ctx, name)
		assert.ErrorIs(t, err, apperr.ErrInvalidArgument, "%q", name)
	}

	admin := dbtest.User(t, e.pool, e.org, "admin")
	_, err = e.gw.Create(principal.With(context.Background(), principal.Principal{UserID: admin, OrgID: e.org, Role: "admin"}), "admin-pool")
	require.NoError(t, err)
	member := dbtest.User(t, e.pool, e.org, "member")
	_, err = e.gw.Create(principal.With(context.Background(), principal.Principal{UserID: member, OrgID: e.org, Role: "member"}), "x")
	assert.ErrorIs(t, err, apperr.ErrPermissionDenied)
}

func TestFirstPoolAssignsResources(t *testing.T) {
	e := gwSetup(t)
	a := e.create(t, "a", nil)
	b := e.create(t, "b", nil)
	assert.Nil(t, a.GatewayID, "no pools: NULL")

	first := e.newPool(t, "first")
	for _, id := range []uuid.UUID{a.ID, b.ID} {
		got, err := e.svc.Get(e.ctx, e.org, id)
		require.NoError(t, err)
		assert.Equal(t, &first.ID, got.GatewayID)
	}
	c := e.create(t, "c", nil)
	assert.Equal(t, &first.ID, c.GatewayID, "one pool: it")

	second := e.newPool(t, "second")
	got, err := e.svc.Get(e.ctx, e.org, a.ID)
	require.NoError(t, err)
	assert.Equal(t, &first.ID, got.GatewayID, "second pool leaves resources alone")

	_, err = e.svc.Create(e.ctx, e.org, inventory.CreateInput{Name: "d", Type: "ssh", Config: sshCfg("h")})
	require.ErrorIs(t, err, apperr.ErrInvalidArgument)
	assert.Contains(t, err.Error(), "gateway_id is required")
	_, err = e.svc.Update(e.ctx, e.org, inventory.UpdateInput{ID: a.ID, Name: "a"})
	require.ErrorIs(t, err, apperr.ErrInvalidArgument)
	assert.Contains(t, err.Error(), "gateway_id is required")
	d, err := e.svc.Create(e.ctx, e.org, inventory.CreateInput{Name: "d", Type: "ssh", Config: sshCfg("h"), GatewayID: &second.ID})
	require.NoError(t, err)
	assert.Equal(t, &second.ID, d.GatewayID)
}

func TestResourceRejectsForeignGateway(t *testing.T) {
	e := gwSetup(t)
	r := e.create(t, "web", nil)
	ctxB, orgB := e.otherOrg(t)
	poolB := dbtest.Gateway(t, e.pool, orgB, "b")

	for _, gw := range []uuid.UUID{poolB, uuid.New()} {
		_, err := e.svc.Update(e.ctx, e.org, inventory.UpdateInput{ID: r.ID, Name: "web", GatewayID: &gw})
		require.ErrorIs(t, err, apperr.ErrInvalidArgument)
		assert.Contains(t, err.Error(), "gateway not found")
		_, err = e.svc.Create(e.ctx, e.org, inventory.CreateInput{Name: "web2", Type: "ssh", Config: sshCfg("h"), GatewayID: &gw})
		require.ErrorIs(t, err, apperr.ErrInvalidArgument)
		assert.Contains(t, err.Error(), "gateway not found")
	}
	_, err := e.pool.Exec(context.Background(), "UPDATE resource SET gateway_id = $1 WHERE id = $2", poolB, r.ID)
	var pgErr *pgconn.PgError
	require.ErrorAs(t, err, &pgErr)
	assert.Equal(t, "23503", pgErr.Code, "the database rejects a pool of another organization")
	assert.NoError(t, e.gw.Delete(ctxB, poolB), "org A cannot pin org B's pool")
}

func TestListGateways(t *testing.T) {
	e := gwSetup(t)
	g := e.newPool(t, "edge")
	idle := e.newPool(t, "idle")
	set := func(id uuid.UUID, host string, connectors []string, ago string, ip string) {
		_, err := e.pool.Exec(context.Background(),
			`UPDATE gateway_instance SET hostname = $2, connectors = $3, last_seen_at = now() - $4::interval, client_ip = $5
			 WHERE id = $1`, id, host, connectors, ago, ip)
		require.NoError(t, err)
	}
	a := dbtest.Instance(t, e.pool, e.org, g.ID)
	b := dbtest.Instance(t, e.pool, e.org, g.ID)
	c := dbtest.Instance(t, e.pool, e.org, g.ID)
	set(a, "gw-a", []string{"echo", "ssh"}, "0s", "192.0.2.1")
	set(b, "gw-b", []string{"ssh"}, "0s", "192.0.2.2")
	set(c, "gw-c", []string{"rdp", "ssh"}, "1 minute", "192.0.2.3")

	list, err := e.gw.List(e.ctx)
	require.NoError(t, err)
	require.Len(t, list, 2)
	got := list[0]
	assert.Equal(t, g.ID, got.ID)
	assert.True(t, got.Online)
	assert.Equal(t, []string{"ssh"}, got.Connectors)
	require.Len(t, got.Instances, 3)
	for i, want := range []struct {
		id     uuid.UUID
		host   string
		online bool
	}{{a, "gw-a", true}, {b, "gw-b", true}, {c, "gw-c", false}} {
		in := got.Instances[i]
		assert.Equal(t, want.id, in.ID)
		assert.Equal(t, want.host, in.Hostname)
		assert.Equal(t, want.online, in.Online, want.host)
		assert.Equal(t, g.ID, in.GatewayID)
		assert.NotNil(t, in.LastSeenAt)
	}
	assert.Equal(t, "192.0.2.1", got.Instances[0].ClientIP.String())
	assert.Equal(t, []string{"echo", "ssh"}, got.Instances[0].Connectors)
	assert.Equal(t, idle.ID, list[1].ID)
	assert.False(t, list[1].Online)
	assert.Empty(t, list[1].Instances)

	_, err = e.pool.Exec(context.Background(), "UPDATE gateway_instance SET last_seen_at = now() - interval '1 minute'")
	require.NoError(t, err)
	list, err = e.gw.List(e.ctx)
	require.NoError(t, err)
	assert.False(t, list[0].Online)
	assert.Empty(t, list[0].Connectors)
}

func TestDeleteGateway(t *testing.T) {
	e := gwSetup(t)
	g := e.newPool(t, "edge")
	r := e.create(t, "web", nil)
	require.Equal(t, &g.ID, r.GatewayID)
	assert.ErrorIs(t, e.gw.Delete(e.ctx, g.ID), apperr.ErrFailedPrecondition)

	require.NoError(t, e.svc.Delete(e.ctx, e.org, r.ID))
	dbtest.Instance(t, e.pool, e.org, g.ID)
	_, _, _, err := e.gw.CreateJoinToken(e.ctx, g.ID, 0, 0)
	require.NoError(t, err)
	require.NoError(t, e.gw.Delete(e.ctx, g.ID))
	assert.Equal(t, 0, e.count(t, "SELECT count(*) FROM gateway_instance WHERE gateway_id = $1", g.ID))
	assert.Equal(t, 0, e.count(t, "SELECT count(*) FROM gateway_join_token WHERE gateway_id = $1", g.ID))
	assert.Equal(t, 1, e.count(t, "SELECT count(*) FROM audit_event WHERE action = 'gateway.delete' AND target_id = $1", g.ID.String()))
	assert.ErrorIs(t, e.gw.Delete(e.ctx, g.ID), apperr.ErrNotFound)

	// Deleting an organization cascades past the RESTRICT of its own resources.
	g2 := e.newPool(t, "edge")
	e.create(t, "web", nil)
	_, err = e.pool.Exec(context.Background(), "DELETE FROM organization WHERE id = $1", e.org)
	require.NoError(t, err)
	assert.Equal(t, 0, e.count(t, "SELECT count(*) FROM gateway WHERE id = $1", g2.ID))
}

func TestDeleteInstance(t *testing.T) {
	e := gwSetup(t)
	g := e.newPool(t, "edge")
	en := e.enroll(t, e.token(t, g.ID, 0), "gw-a")
	serial := parseCert(t, en.CertPEM).SerialNumber.Text(16)

	require.NoError(t, e.gw.DeleteInstance(e.ctx, en.InstanceID))
	assert.Equal(t, 0, e.count(t, "SELECT count(*) FROM gateway_instance WHERE id = $1", en.InstanceID))
	assert.Equal(t, 1, e.count(t,
		"SELECT count(*) FROM audit_event WHERE action = 'gateway.instance.delete' AND target_id = $1", en.InstanceID.String()))
	_, err := e.gw.Authenticate(context.Background(), g.ID, en.InstanceID, serial)
	assert.ErrorIs(t, err, apperr.ErrUnauthenticated)
	assert.ErrorIs(t, e.gw.DeleteInstance(e.ctx, en.InstanceID), apperr.ErrNotFound)
}

func TestGatewaysIsolatedByOrg(t *testing.T) {
	e := gwSetup(t)
	e.newPool(t, "a")
	ctxB, orgB := e.otherOrg(t)
	gB, err := e.gw.Create(ctxB, "b")
	require.NoError(t, err)
	tokBID, _, _, err := e.gw.CreateJoinToken(ctxB, gB.ID, 0, 0)
	require.NoError(t, err)
	instB := dbtest.Instance(t, e.pool, orgB, gB.ID)

	list, err := e.gw.List(e.ctx)
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, "a", list[0].Name)
	tokens, err := e.gw.ListJoinTokens(e.ctx, gB.ID)
	require.NoError(t, err)
	assert.Empty(t, tokens)
	assert.ErrorIs(t, e.gw.Delete(e.ctx, gB.ID), apperr.ErrNotFound)
	assert.ErrorIs(t, e.gw.DeleteInstance(e.ctx, instB), apperr.ErrNotFound)
	assert.ErrorIs(t, e.gw.RevokeJoinToken(e.ctx, tokBID), apperr.ErrNotFound)
	_, _, _, err = e.gw.CreateJoinToken(e.ctx, gB.ID, 0, 0)
	assert.ErrorIs(t, err, apperr.ErrNotFound)

	listB, err := e.gw.List(ctxB)
	require.NoError(t, err)
	require.Len(t, listB, 1)
	assert.Len(t, listB[0].Instances, 1)
	tokens, err = e.gw.ListJoinTokens(ctxB, gB.ID)
	require.NoError(t, err)
	require.Len(t, tokens, 1)
	assert.Nil(t, tokens[0].RevokedAt)
}

func TestGatewayRequiresAdmin(t *testing.T) {
	e := gwSetup(t)
	g := e.newPool(t, "edge")
	member := dbtest.User(t, e.pool, e.org, "member")
	ctx := principal.With(context.Background(), principal.Principal{UserID: member, OrgID: e.org, Role: "member"})
	_, err := e.gw.List(ctx)
	assert.ErrorIs(t, err, apperr.ErrPermissionDenied)
	assert.ErrorIs(t, e.gw.Delete(ctx, g.ID), apperr.ErrPermissionDenied)
	_, _, _, err = e.gw.CreateJoinToken(ctx, g.ID, 0, 0)
	assert.ErrorIs(t, err, apperr.ErrPermissionDenied)
	_, err = e.gw.List(context.Background())
	assert.ErrorIs(t, err, apperr.ErrUnauthenticated)
}
