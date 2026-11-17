package audit_test

import (
	"bytes"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/audit"
	"github.com/werbot/werbot/internal/platform/apperr"
	"github.com/werbot/werbot/internal/platform/auditlog"
	"github.com/werbot/werbot/internal/platform/db/dbtest"
	"github.com/werbot/werbot/internal/platform/dbx"
)

type env struct {
	pool *pgxpool.Pool
	svc  *audit.Service
	a, b dbtest.Fixture
}

func newEnv(t *testing.T) env {
	t.Helper()
	pool, _ := dbtest.New(t)
	return env{pool: pool, svc: audit.New(audit.NewPostgres(pool)), a: dbtest.Seed(t, pool), b: dbtest.Seed(t, pool)}
}

// write stores one event of org f at created, with the actor user (nil for none), and returns its id.
func (e env) write(t *testing.T, f dbtest.Fixture, actor *uuid.UUID, action, targetType, targetID string, created time.Time) uuid.UUID {
	t.Helper()
	require.NoError(t, dbx.InOrg(t.Context(), e.pool, f.OrgID, f.OwnerID, func(tx pgx.Tx) error {
		return auditlog.Write(t.Context(), tx, auditlog.Event{OrgID: &f.OrgID, ActorUserID: actor, ActorKind: "user",
			Action: action, TargetType: targetType, TargetID: targetID, Data: map[string]any{"k": "v"}})
	}))
	var id uuid.UUID
	require.NoError(t, e.pool.QueryRow(t.Context(),
		"UPDATE audit_event SET created_at = $1 WHERE id = (SELECT id FROM audit_event WHERE org_id = $2 ORDER BY id DESC LIMIT 1) RETURNING id",
		created, f.OrgID).Scan(&id))
	return id
}

func actions(evs []audit.Event) []string {
	out := make([]string, len(evs))
	for i, ev := range evs {
		out[i] = ev.Action
	}
	return out
}

func TestListEventsCrossOrg(t *testing.T) {
	e := newEnv(t)
	now := time.Now()
	e.write(t, e.a, nil, "member.invite", "", "", now)
	e.write(t, e.b, nil, "member.remove", "", "", now)
	_, err := e.pool.Exec(t.Context(), "INSERT INTO audit_event (actor_kind, action) VALUES ('system', 'system.boot')")
	require.NoError(t, err)

	evs, next, err := e.svc.ListEvents(t.Context(), e.a.Principal(e.a.AdminID), audit.Filter{})
	require.NoError(t, err)
	assert.Equal(t, []string{"member.invite"}, actions(evs))
	assert.Zero(t, next)
	assert.Equal(t, map[string]any{"k": "v"}, evs[0].Data)
}

func TestListEventsActionFilter(t *testing.T) {
	e := newEnv(t)
	now := time.Now()
	for _, a := range []string{"session.start", "session.end", "sessions.x", "member.invite"} {
		e.write(t, e.a, nil, a, "", "", now)
	}
	p := e.a.Principal(e.a.AdminID)

	evs, _, err := e.svc.ListEvents(t.Context(), p, audit.Filter{Action: "session"})
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"session.start", "session.end"}, actions(evs))

	evs, _, err = e.svc.ListEvents(t.Context(), p, audit.Filter{Action: "session.start"})
	require.NoError(t, err)
	assert.Equal(t, []string{"session.start"}, actions(evs))
}

func TestListEventsActorAndTarget(t *testing.T) {
	e := newEnv(t)
	now := time.Now()
	e.write(t, e.a, &e.a.MemberID, "resource.create", "resource", "r1", now)
	e.write(t, e.a, &e.a.AdminID, "resource.update", "resource", "r2", now)
	e.write(t, e.a, nil, "gateway.join", "gateway", "r1", now)
	p := e.a.Principal(e.a.AdminID)

	evs, _, err := e.svc.ListEvents(t.Context(), p, audit.Filter{ActorUserID: &e.a.MemberID})
	require.NoError(t, err)
	require.Len(t, evs, 1)
	assert.Equal(t, "resource.create", evs[0].Action)
	assert.NotEmpty(t, evs[0].ActorLogin)

	evs, _, err = e.svc.ListEvents(t.Context(), p, audit.Filter{TargetType: "resource", TargetID: "r2"})
	require.NoError(t, err)
	assert.Equal(t, []string{"resource.update"}, actions(evs))
}

func TestListEventsRangeAndPagination(t *testing.T) {
	e := newEnv(t)
	t0 := time.Now().Add(-10 * time.Hour).Truncate(time.Second)
	for i, a := range []string{"x.a", "x.b", "x.c", "x.d", "x.e"} {
		e.write(t, e.a, nil, a, "", "", t0.Add(time.Duration(i)*time.Hour))
	}
	p := e.a.Principal(e.a.AdminID)

	from, to := t0.Add(time.Hour), t0.Add(4*time.Hour)
	evs, _, err := e.svc.ListEvents(t.Context(), p, audit.Filter{From: &from, To: &to})
	require.NoError(t, err)
	assert.Equal(t, []string{"x.d", "x.c", "x.b"}, actions(evs), "[From, To), newest first")

	evs, next, err := e.svc.ListEvents(t.Context(), p, audit.Filter{PageSize: 2})
	require.NoError(t, err)
	assert.Equal(t, []string{"x.e", "x.d"}, actions(evs))
	assert.Equal(t, 2, next)
	evs, next, err = e.svc.ListEvents(t.Context(), p, audit.Filter{Page: 3, PageSize: 2})
	require.NoError(t, err)
	assert.Equal(t, []string{"x.a"}, actions(evs))
	assert.Zero(t, next)

	for _, f := range []audit.Filter{{From: &to, To: &from}, {From: &from, To: &from}, {Page: -1}, {Page: 1 << 30}, {PageSize: -1}} {
		_, _, err = e.svc.ListEvents(t.Context(), p, f)
		assert.ErrorIs(t, err, apperr.ErrInvalidArgument, "%+v", f)
	}
	evs, _, err = e.svc.ListEvents(t.Context(), p, audit.Filter{PageSize: 100000})
	require.NoError(t, err, "page size above the max is clamped")
	assert.Len(t, evs, 5)
}

func TestGetEventOtherOrg(t *testing.T) {
	e := newEnv(t)
	idA := e.write(t, e.a, &e.a.OwnerID, "member.invite", "", "", time.Now())
	idB := e.write(t, e.b, nil, "member.invite", "", "", time.Now())
	p := e.a.Principal(e.a.AdminID)

	ev, err := e.svc.GetEvent(t.Context(), p, idA)
	require.NoError(t, err)
	assert.Equal(t, idA, ev.ID)
	assert.NotEmpty(t, ev.ActorLogin)

	_, err = e.svc.GetEvent(t.Context(), p, idB)
	assert.ErrorIs(t, err, apperr.ErrNotFound)
}

func TestAuditServiceDeniesNonAdmin(t *testing.T) {
	e := newEnv(t)
	id := e.write(t, e.a, nil, "member.invite", "", "", time.Now())
	p := e.a.Principal(e.a.MemberID)
	_, _, err := e.svc.ListEvents(t.Context(), p, audit.Filter{})
	assert.ErrorIs(t, err, apperr.ErrPermissionDenied)
	_, err = e.svc.GetEvent(t.Context(), p, id)
	assert.ErrorIs(t, err, apperr.ErrPermissionDenied)
	p.Role = ""
	_, _, err = e.svc.ListEvents(t.Context(), p, audit.Filter{})
	assert.ErrorIs(t, err, apperr.ErrPermissionDenied)
}

func TestListEventsTieBreak(t *testing.T) {
	e := newEnv(t)
	at := time.Now().Add(-time.Hour).Truncate(time.Second)
	want := make([]uuid.UUID, 5)
	for i := range want {
		want[i] = e.write(t, e.a, nil, "x.tie", "", "", at)
	}
	slices.SortFunc(want, func(a, b uuid.UUID) int { return bytes.Compare(b[:], a[:]) }) // id DESC

	var got []uuid.UUID
	for page := 1; page != 0; {
		evs, next, err := e.svc.ListEvents(t.Context(), e.a.Principal(e.a.AdminID), audit.Filter{Page: page, PageSize: 2})
		require.NoError(t, err)
		for _, ev := range evs {
			got = append(got, ev.ID)
		}
		page = next
	}
	assert.Equal(t, want, got, "equal created_at: id DESC, no repeat, no gap")
}
