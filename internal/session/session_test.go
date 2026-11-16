package session

import (
	"context"
	"errors"
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/platform/apperr"
	"github.com/werbot/werbot/internal/platform/db/dbtest"
	"github.com/werbot/werbot/internal/platform/jobs"
	"github.com/werbot/werbot/internal/session/dbq"
)

var bg = context.Background()

type env struct {
	svc            *Service
	pool           *pgxpool.Pool
	org, user, res uuid.UUID
	gw, a, b       uuid.UUID // pool and two of its instances
}

func setup(t *testing.T) env {
	t.Helper()
	pool, _ := dbtest.New(t)
	org := dbtest.Org(t, pool)
	gw := dbtest.Gateway(t, pool, org, "edge")
	return env{
		svc: New(pool), pool: pool, org: org,
		user: dbtest.User(t, pool, org, "member"),
		res:  dbtest.Resource(t, pool, org, "web", nil),
		gw:   gw, a: dbtest.Instance(t, pool, org, gw), b: dbtest.Instance(t, pool, org, gw),
	}
}

func (e env) exec(t *testing.T, sql string, args ...any) {
	t.Helper()
	_, err := e.pool.Exec(bg, sql, args...)
	require.NoError(t, err)
}

// open inserts an open session of instance started age ago; a nil instance models a deleted one.
func (e env) open(t *testing.T, instance *uuid.UUID, age time.Duration) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	require.NoError(t, e.pool.QueryRow(bg,
		`INSERT INTO session (org_id, user_id, resource_id, gateway_id, gateway_instance_id, client_ip, started_at)
		 VALUES ($1, $2, $3, $4, $5, '10.0.0.1', now() - $6::float8 * interval '1 second') RETURNING id`,
		e.org, e.user, e.res, e.gw, instance, age.Seconds()).Scan(&id))
	return id
}

func (e env) ended(t *testing.T, id uuid.UUID) (reason *string) {
	t.Helper()
	require.NoError(t, e.pool.QueryRow(bg, "SELECT end_reason FROM session WHERE id = $1", id).Scan(&reason))
	return reason
}

func (e env) endAudits(t *testing.T, kind string) int {
	t.Helper()
	var n int
	require.NoError(t, e.pool.QueryRow(bg,
		"SELECT count(*) FROM audit_event WHERE action = 'session.end' AND actor_kind = $1", kind).Scan(&n))
	return n
}

func TestStartAndEnd(t *testing.T) {
	e := setup(t)
	id, err := e.svc.Start(bg, Start{OrgID: e.org, UserID: e.user, ResourceID: e.res, GatewayID: e.gw, InstanceID: e.a,
		ClientIP: netip.MustParseAddr("203.0.113.7")})
	require.NoError(t, err)
	assert.Equal(t, uuid.Version(7), id.Version())

	var gw, inst uuid.UUID
	var ip netip.Addr
	var endedNull bool
	require.NoError(t, e.pool.QueryRow(bg,
		"SELECT gateway_id, gateway_instance_id, client_ip, ended_at IS NULL FROM session WHERE id = $1", id).
		Scan(&gw, &inst, &ip, &endedNull))
	assert.Equal(t, [4]any{e.gw, e.a, netip.MustParseAddr("203.0.113.7"), true}, [4]any{gw, inst, ip, endedNull})

	var targetType, targetID, sid string
	require.NoError(t, e.pool.QueryRow(bg,
		"SELECT target_type, target_id, data->>'session_id' FROM audit_event WHERE action = 'session.start' AND actor_kind = 'user'").
		Scan(&targetType, &targetID, &sid))
	assert.Equal(t, [3]string{"resource", e.res.String(), id.String()}, [3]string{targetType, targetID, sid})

	require.NoError(t, e.svc.End(bg, e.org, e.a, id, "closed"))
	require.NotNil(t, e.ended(t, id))
	assert.Equal(t, "closed", *e.ended(t, id))
	assert.Equal(t, 1, e.endAudits(t, "gateway"))

	require.NoError(t, e.svc.End(bg, e.org, e.a, id, "again"))
	assert.Equal(t, "closed", *e.ended(t, id))
	assert.Equal(t, 1, e.endAudits(t, "gateway"), "repeat End writes no event")
}

func TestEndForeign(t *testing.T) {
	e := setup(t)
	id := e.open(t, &e.a, 0)
	assert.ErrorIs(t, e.svc.End(bg, e.org, e.b, id, "closed"), apperr.ErrNotFound)
	assert.ErrorIs(t, e.svc.End(bg, uuid.New(), e.a, id, "closed"), apperr.ErrNotFound)
	assert.Nil(t, e.ended(t, id))
}

func TestEndMissingScopedToInstance(t *testing.T) {
	e := setup(t)
	a1, a2, a3 := e.open(t, &e.a, 2*time.Minute), e.open(t, &e.a, 2*time.Minute), e.open(t, &e.a, 0)
	b1 := e.open(t, &e.b, 2*time.Minute)
	n, err := e.svc.EndMissing(bg, e.org, e.a, []uuid.UUID{a1}, "gateway_lost")
	require.NoError(t, err)
	assert.EqualValues(t, 1, n)
	assert.Nil(t, e.ended(t, a1))
	require.NotNil(t, e.ended(t, a2))
	assert.Equal(t, "gateway_lost", *e.ended(t, a2))
	assert.Nil(t, e.ended(t, a3), "started under a minute ago")
	assert.Nil(t, e.ended(t, b1), "other instance")
	assert.Equal(t, 1, e.endAudits(t, "gateway"))

	n, err = e.svc.EndMissing(bg, e.org, e.b, nil, "gateway_lost")
	require.NoError(t, err)
	assert.EqualValues(t, 1, n, "nil active closes every stale session")
	assert.NotNil(t, e.ended(t, b1))
}

// sqlOf records the text of the first query instead of running it.
type sqlOf struct {
	dbq.DBTX
	sql string
}

func (s *sqlOf) Query(_ context.Context, sql string, _ ...any) (pgx.Rows, error) {
	s.sql = sql
	return nil, errors.New("recorded")
}

// The active-set check must stay O(rows + ids) under a generic plan too.
func TestEndMissingPlanIsHashed(t *testing.T) {
	e := setup(t)
	rec := &sqlOf{}
	_, _ = dbq.New(rec).EndMissingSessions(bg, dbq.EndMissingSessionsParams{})
	require.NotEmpty(t, rec.sql)
	conn, err := e.pool.Acquire(bg)
	require.NoError(t, err)
	defer conn.Release()
	// The simple protocol sends $n unbound, as GENERIC_PLAN needs.
	res, err := conn.Conn().PgConn().Exec(bg, "EXPLAIN (GENERIC_PLAN, COSTS OFF) "+rec.sql).ReadAll()
	require.NoError(t, err)
	var plan strings.Builder
	for _, row := range res[0].Rows {
		plan.Write(row[0])
		plan.WriteByte('\n')
	}
	assert.Contains(t, plan.String(), "hashed SubPlan", plan.String())
}

func TestEndMissingLargeActiveSet(t *testing.T) {
	e := setup(t)
	rows, err := e.pool.Query(bg,
		`INSERT INTO session (org_id, user_id, resource_id, gateway_id, gateway_instance_id, client_ip, started_at)
		 SELECT $1, $2, $3, $4, $5, '10.0.0.1', now() - interval '2 minutes' FROM generate_series(1, 600)
		 RETURNING id`, e.org, e.user, e.res, e.gw, e.a)
	require.NoError(t, err)
	ids, err := pgx.CollectRows(rows, pgx.RowTo[uuid.UUID])
	require.NoError(t, err)
	active := append([]uuid.UUID{}, ids[:500]...)
	for range 9500 {
		active = append(active, uuid.New())
	}
	absent := ids[500:]

	n, err := e.svc.EndMissing(bg, e.org, e.a, active, "gateway_lost")
	require.NoError(t, err)
	assert.EqualValues(t, 100, n)
	var closed int
	require.NoError(t, e.pool.QueryRow(bg,
		"SELECT count(*) FROM session WHERE id = ANY($1) AND ended_at IS NOT NULL", absent).Scan(&closed))
	assert.Equal(t, 100, closed, "exactly the absent ones")
}

func TestEndLost(t *testing.T) {
	e := setup(t)
	e.exec(t, "UPDATE gateway_instance SET last_seen_at = now() - interval '3 minutes' WHERE id = $1", e.a)
	e.exec(t, "UPDATE gateway_instance SET last_seen_at = now() WHERE id = $1", e.b)
	silent := dbtest.Instance(t, e.pool, e.org, e.gw)
	e.exec(t, "UPDATE gateway_instance SET created_at = now() - interval '3 minutes' WHERE id = $1", silent)
	s1, s2, s3, s4 := e.open(t, &e.a, time.Hour), e.open(t, nil, time.Hour), e.open(t, &e.b, time.Hour), e.open(t, &silent, time.Hour)

	n, err := e.svc.EndLost(bg, 2*time.Minute)
	require.NoError(t, err)
	assert.EqualValues(t, 3, n)
	for _, id := range []uuid.UUID{s1, s2, s4} {
		require.NotNil(t, e.ended(t, id))
		assert.Equal(t, "gateway_lost", *e.ended(t, id))
	}
	assert.Nil(t, e.ended(t, s3))
	assert.Equal(t, 3, e.endAudits(t, "system"))
}

func TestEndLostJobStartupGrace(t *testing.T) {
	e := setup(t)
	e.exec(t, "UPDATE gateway_instance SET last_seen_at = now() - interval '3 minutes' WHERE id = $1", e.a)
	id := e.open(t, &e.a, time.Hour)

	workers := river.NewWorkers()
	assert.Len(t, RegisterJobs(workers, e.svc, time.Now()), 1)
	w := &endLostWorker{s: e.svc, started: time.Now()}
	require.NoError(t, w.Work(bg, &river.Job[EndLostArgs]{}))
	assert.Nil(t, e.ended(t, id), "inside startup grace")

	w.started = time.Now().Add(-3 * time.Minute)
	require.NoError(t, w.Work(bg, &river.Job[EndLostArgs]{}))
	assert.NotNil(t, e.ended(t, id))

	assert.Equal(t, "session.end_lost", EndLostArgs{}.Kind())
	assert.Equal(t, jobs.QueueMaintenance, EndLostArgs{}.InsertOpts().Queue)
}

func TestEndLostPeriodicJob(t *testing.T) {
	e := setup(t)
	workers := river.NewWorkers()
	c, err := jobs.NewClient(e.pool, workers, RegisterJobs(workers, e.svc, time.Now()), nil)
	require.NoError(t, err)
	assert.True(t, c.PeriodicJobs().RemoveByID("session.end_lost"))
	t0 := time.Date(2026, 11, 16, 10, 0, 0, 0, time.UTC)
	assert.Equal(t, t0.Add(30*time.Second), endLostSchedule.Next(t0))
}
