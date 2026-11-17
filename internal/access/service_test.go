package access_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/access"
	"github.com/werbot/werbot/internal/connector"
	"github.com/werbot/werbot/internal/platform/apperr"
	"github.com/werbot/werbot/internal/platform/db/dbtest"
	"github.com/werbot/werbot/internal/platform/principal"
)

type env struct {
	svc   *access.Service
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
	return env{access.New(pool), pool, ctx, org, owner}
}

func ptr[T any](v T) *T { return &v }

func (e env) user(t *testing.T) access.CreateInput {
	t.Helper()
	return access.CreateInput{SubjectType: "user", SubjectID: dbtest.User(t, e.pool, e.org, "member")}
}

func TestCreateExplicitAndSelector(t *testing.T) {
	e := setup(t)
	r := dbtest.Resource(t, e.pool, e.org, "web", nil)
	in := e.user(t)
	in.ResourceID = &r
	b, err := e.svc.Create(e.ctx, e.org, in)
	require.NoError(t, err)
	assert.Equal(t, e.owner, b.CreatedBy)
	assert.Equal(t, &r, b.ResourceID)

	team := dbtest.Team(t, e.pool, e.org)
	s, err := e.svc.Create(e.ctx, e.org, access.CreateInput{SubjectType: "team", SubjectID: team, LabelSelector: map[string]string{"env": "prod"}})
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"env": "prod"}, s.LabelSelector)
	assert.Nil(t, s.ResourceID)

	all, err := e.svc.List(e.ctx, e.org, nil)
	require.NoError(t, err)
	assert.Len(t, all, 2)
}

func TestCreateRejectsTargets(t *testing.T) {
	e := setup(t)
	r := dbtest.Resource(t, e.pool, e.org, "web", nil)
	tooMany := map[string]string{}
	for _, k := range []string{"a", "b", "c", "d", "e", "f", "g", "h", "i"} {
		tooMany[k] = "v"
	}
	for name, mod := range map[string]func(*access.CreateInput){
		"both":        func(in *access.CreateInput) { in.ResourceID, in.LabelSelector = &r, map[string]string{"a": "b"} },
		"none":        func(*access.CreateInput) {},
		"empty":       func(in *access.CreateInput) { in.LabelSelector = map[string]string{} },
		"bad key":     func(in *access.CreateInput) { in.LabelSelector = map[string]string{"Bad Key": "x"} },
		"bad value":   func(in *access.CreateInput) { in.LabelSelector = map[string]string{"k": "bad value!"} },
		"too many":    func(in *access.CreateInput) { in.LabelSelector = tooMany },
		"bad subject": func(in *access.CreateInput) { in.ResourceID, in.SubjectType = &r, "group" },
	} {
		in := e.user(t)
		mod(&in)
		_, err := e.svc.Create(e.ctx, e.org, in)
		assert.ErrorIs(t, err, apperr.ErrInvalidArgument, name)
	}
}

func TestSubjectMustBeInOrg(t *testing.T) {
	e := setup(t)
	orgB := dbtest.Org(t, e.pool)
	sel := map[string]string{"env": "prod"}
	_, err := e.svc.Create(e.ctx, e.org, access.CreateInput{SubjectType: "user", SubjectID: dbtest.User(t, e.pool, orgB, "member"), LabelSelector: sel})
	assert.ErrorIs(t, err, apperr.ErrInvalidArgument)
	_, err = e.svc.Create(e.ctx, e.org, access.CreateInput{SubjectType: "team", SubjectID: dbtest.Team(t, e.pool, orgB), LabelSelector: sel})
	assert.ErrorIs(t, err, apperr.ErrInvalidArgument)
}

func TestResourceMustBeInOrg(t *testing.T) {
	e := setup(t)
	orgB := dbtest.Org(t, e.pool)
	in := e.user(t)
	in.ResourceID = ptr(dbtest.Resource(t, e.pool, orgB, "other", nil))
	_, err := e.svc.Create(e.ctx, e.org, in)
	assert.ErrorIs(t, err, apperr.ErrNotFound)
}

func TestSelfGrantIsAudited(t *testing.T) {
	e := setup(t)
	r := dbtest.Resource(t, e.pool, e.org, "web", nil)
	_, err := e.svc.Create(e.ctx, e.org, access.CreateInput{SubjectType: "user", SubjectID: e.owner, ResourceID: &r})
	require.NoError(t, err)
	var self string
	require.NoError(t, e.pool.QueryRow(context.Background(),
		`SELECT data->>'self_grant' FROM audit_event WHERE org_id = $1 AND action = 'access.binding.create'`, e.org).Scan(&self))
	assert.Equal(t, "true", self)
}

func TestListByResource(t *testing.T) {
	e := setup(t)
	r1 := dbtest.Resource(t, e.pool, e.org, "r1", nil)
	r2 := dbtest.Resource(t, e.pool, e.org, "r2", nil)
	a, b, s := e.user(t), e.user(t), e.user(t)
	a.ResourceID, b.ResourceID, s.LabelSelector = &r1, &r2, map[string]string{"env": "prod"}
	var ids []uuid.UUID
	for _, in := range []access.CreateInput{a, b, s} {
		got, err := e.svc.Create(e.ctx, e.org, in)
		require.NoError(t, err)
		ids = append(ids, got.ID)
	}
	got, err := e.svc.List(e.ctx, e.org, &r1)
	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.ElementsMatch(t, []uuid.UUID{ids[0], ids[2]}, []uuid.UUID{got[0].ID, got[1].ID})
}

func TestDelete(t *testing.T) {
	e := setup(t)
	in := e.user(t)
	in.LabelSelector = map[string]string{"a": "b"}
	b, err := e.svc.Create(e.ctx, e.org, in)
	require.NoError(t, err)
	orgB := dbtest.Org(t, e.pool)
	assert.ErrorIs(t, e.svc.Delete(e.ctx, orgB, b.ID), apperr.ErrNotFound)
	require.NoError(t, e.svc.Delete(e.ctx, e.org, b.ID))
	assert.ErrorIs(t, e.svc.Delete(e.ctx, e.org, b.ID), apperr.ErrNotFound)

	var data map[string]any
	require.NoError(t, e.pool.QueryRow(context.Background(),
		`SELECT data FROM audit_event WHERE org_id = $1 AND action = 'access.binding.delete' AND target_id = $2`,
		e.org, b.ID.String()).Scan(&data))
	assert.Equal(t, map[string]any{"subject_type": "user", "subject_id": in.SubjectID.String(),
		"label_selector": map[string]any{"a": "b"}}, data)

	r := dbtest.Resource(t, e.pool, e.org, "web", nil)
	in.LabelSelector, in.ResourceID = nil, &r
	b, err = e.svc.Create(e.ctx, e.org, in)
	require.NoError(t, err)
	require.NoError(t, e.svc.Delete(e.ctx, e.org, b.ID))
	data = nil
	require.NoError(t, e.pool.QueryRow(context.Background(),
		`SELECT data FROM audit_event WHERE org_id = $1 AND action = 'access.binding.delete' AND target_id = $2`,
		e.org, b.ID.String()).Scan(&data))
	assert.Equal(t, map[string]any{"subject_type": "user", "subject_id": in.SubjectID.String(), "resource_id": r.String()}, data)
}

func count(t *testing.T, e env, org uuid.UUID) (n int) {
	t.Helper()
	require.NoError(t, e.pool.QueryRow(context.Background(), `SELECT count(*) FROM access_binding WHERE org_id = $1`, org).Scan(&n))
	return n
}

func TestMemberRemovalDropsOnlyThatOrgBindings(t *testing.T) {
	e := setup(t)
	sel := map[string]string{"env": "prod"}
	u := dbtest.User(t, e.pool, e.org, "member")
	orgB := dbtest.Org(t, e.pool)
	_, err := e.pool.Exec(context.Background(), `INSERT INTO membership (org_id, user_id, role) VALUES ($1, $2, 'member')`, orgB, u)
	require.NoError(t, err)
	ctxB := principal.With(context.Background(), principal.Principal{UserID: u, OrgID: orgB, Role: "owner"})
	_, err = e.svc.Create(e.ctx, e.org, access.CreateInput{SubjectType: "user", SubjectID: u, LabelSelector: sel})
	require.NoError(t, err)
	_, err = e.svc.Create(ctxB, orgB, access.CreateInput{SubjectType: "user", SubjectID: u, LabelSelector: sel})
	require.NoError(t, err)

	_, err = e.pool.Exec(context.Background(), `DELETE FROM membership WHERE org_id = $1 AND user_id = $2`, e.org, u)
	require.NoError(t, err)
	assert.Equal(t, 0, count(t, e, e.org))
	assert.Equal(t, 1, count(t, e, orgB), "binding in the other org survives")
}

func TestTeamDeletionDropsBindings(t *testing.T) {
	e := setup(t)
	team := dbtest.Team(t, e.pool, e.org)
	_, err := e.svc.Create(e.ctx, e.org, access.CreateInput{SubjectType: "team", SubjectID: team, LabelSelector: map[string]string{"a": "b"}})
	require.NoError(t, err)
	_, err = e.pool.Exec(context.Background(), `DELETE FROM team WHERE id = $1`, team)
	require.NoError(t, err)
	assert.Equal(t, 0, count(t, e, e.org))
}

func TestCreateBindingPorts(t *testing.T) {
	e := setup(t)
	r := dbtest.Resource(t, e.pool, e.org, "db", nil)
	in := e.user(t)
	in.ResourceID = &r
	in.Ports = []connector.PortRange{{Port: 8000, EndPort: 8100}, {Port: 5432, EndPort: 5432}, {Port: 65535}}
	b, err := e.svc.Create(e.ctx, e.org, in)
	require.NoError(t, err)
	want := []connector.PortRange{{Port: 8000, EndPort: 8100}, {Port: 5432}, {Port: 65535}} // EndPort equal to Port reads back as 0
	assert.Equal(t, want, b.Ports)
	var ranges, audited string
	require.NoError(t, e.pool.QueryRow(context.Background(), `SELECT ports::text FROM access_binding WHERE id = $1`, b.ID).Scan(&ranges))
	assert.Equal(t, `{"[8000,8101)","[5432,5433)","[65535,65536)"}`, ranges)
	require.NoError(t, e.pool.QueryRow(context.Background(),
		`SELECT data->>'ports' FROM audit_event WHERE org_id = $1 AND action = 'access.binding.create'`, e.org).Scan(&audited))
	stored := `[{"port":8000,"end_port":8100},{"port":5432,"end_port":0},{"port":65535,"end_port":0}]`
	assert.JSONEq(t, stored, audited, "the stored form")

	all, err := e.svc.List(e.ctx, e.org, nil)
	require.NoError(t, err)
	require.Len(t, all, 1)
	assert.Equal(t, want, all[0].Ports)
	require.NoError(t, e.svc.Delete(e.ctx, e.org, b.ID))
	require.NoError(t, e.pool.QueryRow(context.Background(),
		`SELECT data->>'ports' FROM audit_event WHERE org_id = $1 AND action = 'access.binding.delete'`, e.org).Scan(&audited))
	assert.JSONEq(t, stored, audited, "delete")

	in.Ports = nil // no ports: every port
	b, err = e.svc.Create(e.ctx, e.org, in)
	require.NoError(t, err)
	assert.Empty(t, b.Ports)

	many := make([]connector.PortRange, 17)
	for i := range many {
		many[i] = connector.PortRange{Port: uint16(i + 1)}
	}
	for name, ports := range map[string][]connector.PortRange{
		"zero":      {{Port: 0}},
		"reversed":  {{Port: 9000, EndPort: 8000}},
		"overlap":   {{Port: 5432}, {Port: 5000, EndPort: 6000}},
		"17 ranges": many,
	} {
		in.Ports = ports
		_, err := e.svc.Create(e.ctx, e.org, in)
		assert.ErrorIs(t, err, apperr.ErrInvalidArgument, name)
	}
}
