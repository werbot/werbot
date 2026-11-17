package session_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/platform/apperr"
	"github.com/werbot/werbot/internal/platform/db/dbtest"
	"github.com/werbot/werbot/internal/session"
)

// listEnv is org A with sessions of member, otherMember and admin (a, b, c,
// started an hour apart in that order) and org B with one session.
type listEnv struct {
	pool    *pgxpool.Pool
	svc     *session.Service
	a, b    dbtest.Fixture
	ids     [3]uuid.UUID // member, otherMember, admin
	idB     uuid.UUID
	started time.Time
}

func newListEnv(t *testing.T) listEnv {
	t.Helper()
	pool, _ := dbtest.New(t)
	e := listEnv{pool: pool, svc: session.New(pool), a: dbtest.Seed(t, pool), b: dbtest.Seed(t, pool),
		started: time.Now().Add(-10 * time.Hour).Truncate(time.Second)}
	for i, u := range []uuid.UUID{e.a.MemberID, e.a.OtherMemberID, e.a.AdminID} {
		e.ids[i] = e.a.StartSession(t, pool, u)
		_, err := pool.Exec(t.Context(), "UPDATE session SET started_at = $2 WHERE id = $1", e.ids[i], e.started.Add(time.Duration(i)*time.Hour))
		require.NoError(t, err)
	}
	e.idB = e.b.StartSession(t, pool, e.b.MemberID)
	return e
}

func ids(items []session.Summary) []uuid.UUID {
	out := make([]uuid.UUID, len(items))
	for i, s := range items {
		out[i] = s.ID
	}
	return out
}

func TestListSessionsVisibility(t *testing.T) {
	e := newListEnv(t)
	items, next, err := e.svc.ListSessions(t.Context(), e.a.Principal(e.a.OwnerID), session.ListFilter{})
	require.NoError(t, err)
	assert.Equal(t, []uuid.UUID{e.ids[2], e.ids[1], e.ids[0]}, ids(items))
	assert.Zero(t, next)
	assert.Equal(t, "web", items[0].ResourceName)
	assert.Equal(t, e.a.GatewayID, *items[0].GatewayID)

	items, _, err = e.svc.ListSessions(t.Context(), e.a.Principal(e.a.MemberID), session.ListFilter{})
	require.NoError(t, err)
	assert.Equal(t, []uuid.UUID{e.ids[0]}, ids(items))

	items, _, err = e.svc.ListSessions(t.Context(), e.b.Principal(e.b.AdminID), session.ListFilter{})
	require.NoError(t, err)
	assert.Equal(t, []uuid.UUID{e.idB}, ids(items))
}

func TestListSessionsMemberForeignFilter(t *testing.T) {
	e := newListEnv(t)
	_, _, err := e.svc.ListSessions(t.Context(), e.a.Principal(e.a.MemberID), session.ListFilter{UserID: &e.a.OtherMemberID})
	assert.ErrorIs(t, err, apperr.ErrPermissionDenied)
	items, _, err := e.svc.ListSessions(t.Context(), e.a.Principal(e.a.MemberID), session.ListFilter{UserID: &e.a.MemberID})
	require.NoError(t, err)
	assert.Len(t, items, 1)
}

func TestListSessionsFilters(t *testing.T) {
	e := newListEnv(t)
	p := e.a.Principal(e.a.OwnerID)
	list := func(f session.ListFilter) []uuid.UUID {
		items, _, err := e.svc.ListSessions(t.Context(), p, f)
		require.NoError(t, err)
		return ids(items)
	}
	_, err := e.pool.Exec(t.Context(), "UPDATE session SET ended_at = now(), end_reason = 'client_closed' WHERE id = $1", e.ids[1])
	require.NoError(t, err)

	assert.Equal(t, []uuid.UUID{e.ids[1]}, list(session.ListFilter{UserID: &e.a.OtherMemberID}))
	assert.Len(t, list(session.ListFilter{ResourceID: &e.a.ResourceID}), 3)
	assert.Empty(t, list(session.ListFilter{ResourceID: &e.b.ResourceID}))
	assert.Equal(t, []uuid.UUID{e.ids[1]}, list(session.ListFilter{Status: "ended"}))
	assert.Equal(t, []uuid.UUID{e.ids[2], e.ids[0]}, list(session.ListFilter{Status: "active"}))

	from, to := e.started.Add(time.Hour), e.started.Add(2*time.Hour) // [from, to): only the middle one
	assert.Equal(t, []uuid.UUID{e.ids[1]}, list(session.ListFilter{From: &from, To: &to}))
	assert.Equal(t, []uuid.UUID{e.ids[2], e.ids[1]}, list(session.ListFilter{From: &from}))
	assert.Equal(t, []uuid.UUID{e.ids[1], e.ids[0]}, list(session.ListFilter{To: &to}))

	items, _, err := e.svc.ListSessions(t.Context(), p, session.ListFilter{Status: "ended"})
	require.NoError(t, err)
	assert.Equal(t, "client_closed", items[0].EndReason)
	assert.NotNil(t, items[0].EndedAt)
}

func TestListSessionsPagination(t *testing.T) {
	e := newListEnv(t)
	p := e.a.Principal(e.a.OwnerID)
	items, next, err := e.svc.ListSessions(t.Context(), p, session.ListFilter{PageSize: 2})
	require.NoError(t, err)
	assert.Equal(t, []uuid.UUID{e.ids[2], e.ids[1]}, ids(items))
	assert.Equal(t, 2, next)

	items, next, err = e.svc.ListSessions(t.Context(), p, session.ListFilter{Page: 2, PageSize: 2})
	require.NoError(t, err)
	assert.Equal(t, []uuid.UUID{e.ids[0]}, ids(items))
	assert.Zero(t, next)
}

func TestListSessionsInvalid(t *testing.T) {
	e := newListEnv(t)
	p := e.a.Principal(e.a.OwnerID)
	t0 := e.started
	for name, f := range map[string]session.ListFilter{
		"to equals from": {From: &t0, To: &t0},
		"status":         {Status: "x"},
		"page":           {Page: -1},
	} {
		_, _, err := e.svc.ListSessions(t.Context(), p, f)
		assert.ErrorIs(t, err, apperr.ErrInvalidArgument, name)
	}
}

func TestListSessionsKeepsDeletedHistory(t *testing.T) {
	e := newListEnv(t)
	_, err := e.pool.Exec(t.Context(), "DELETE FROM resource WHERE id = $1", e.a.ResourceID)
	require.NoError(t, err)
	items, _, err := e.svc.ListSessions(t.Context(), e.a.Principal(e.a.OwnerID), session.ListFilter{})
	require.NoError(t, err)
	require.Len(t, items, 3)
	assert.Nil(t, items[0].ResourceID)
	assert.Empty(t, items[0].ResourceName)
}

func TestGetSessionVisibility(t *testing.T) {
	e := newListEnv(t)
	_, err := e.svc.GetSession(t.Context(), e.a.Principal(e.a.MemberID), e.ids[1])
	assert.ErrorIs(t, err, apperr.ErrNotFound)
	s, err := e.svc.GetSession(t.Context(), e.a.Principal(e.a.MemberID), e.ids[0])
	require.NoError(t, err)
	assert.Equal(t, e.ids[0], s.ID)
	_, err = e.svc.GetSession(t.Context(), e.a.Principal(e.a.AdminID), e.ids[1])
	assert.NoError(t, err)
	_, err = e.svc.GetSession(t.Context(), e.a.Principal(e.a.AdminID), e.idB)
	assert.ErrorIs(t, err, apperr.ErrNotFound)
}
