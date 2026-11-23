package session_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/platform/apperr"
	"github.com/werbot/werbot/internal/platform/db/dbtest"
	"github.com/werbot/werbot/internal/platform/pgnotify"
	"github.com/werbot/werbot/internal/session"
)

const sentinel = "sentinel"

// termEnv is org A with a session of member and one of otherMember, org B, and
// a listener on the terminate channel.
type termEnv struct {
	pool          *pgxpool.Pool
	svc           *session.Service
	a, b          dbtest.Fixture
	mine, foreign uuid.UUID // sessions of a.MemberID and a.OtherMemberID
	notes         <-chan string
}

func newTermEnv(t *testing.T) termEnv {
	t.Helper()
	pool, _ := dbtest.New(t)
	e := termEnv{pool: pool, svc: session.New(pool), a: dbtest.Seed(t, pool), b: dbtest.Seed(t, pool)}
	e.mine, e.foreign = e.a.StartSession(t, pool, e.a.MemberID), e.a.StartSession(t, pool, e.a.OtherMemberID)
	e.notes = listenAll(t, pool, pgnotify.SessionTerminate)
	return e
}

// listenAll returns every payload sent on channel until the test ends.
func listenAll(t *testing.T, pool *pgxpool.Pool, channel string) <-chan string {
	t.Helper()
	conn, err := pool.Acquire(t.Context())
	require.NoError(t, err)
	_, err = conn.Exec(t.Context(), "LISTEN "+channel)
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	out, done := make(chan string, 16), make(chan struct{})
	go func() {
		defer close(done)
		for {
			n, err := conn.Conn().WaitForNotification(ctx)
			if err != nil {
				return
			}
			out <- n.Payload
		}
	}()
	t.Cleanup(func() { cancel(); <-done; conn.Release() })
	return out
}

// sent returns the payloads sent before now: it sends a sentinel last and
// collects up to it (notifications arrive in commit order).
func (e termEnv) sent(t *testing.T) []string {
	t.Helper()
	_, err := e.pool.Exec(t.Context(), "SELECT pg_notify($1, $2)", pgnotify.SessionTerminate, sentinel)
	require.NoError(t, err)
	var got []string
	for {
		select {
		case p := <-e.notes:
			if p == sentinel {
				return got
			}
			got = append(got, p)
		case <-time.After(2 * time.Second):
			require.FailNow(t, "no notification within 2s")
		}
	}
}

func (e termEnv) note(id uuid.UUID) string { return e.a.InstanceID.String() + ":" + id.String() }

// requested returns the stored terminate request of id.
func (e termEnv) requested(t *testing.T, id uuid.UUID) (at *time.Time, reason *string) {
	t.Helper()
	require.NoError(t, e.pool.QueryRow(t.Context(),
		"SELECT terminate_requested_at, terminate_reason FROM session WHERE id = $1", id).Scan(&at, &reason))
	return at, reason
}

// audits returns the reasons of the session.terminate events of id.
func (e termEnv) audits(t *testing.T, id uuid.UUID) []string {
	t.Helper()
	rows, err := e.pool.Query(t.Context(), `SELECT data->>'reason' FROM audit_event
		WHERE action = 'session.terminate' AND target_type = 'session' AND target_id = $1 ORDER BY created_at`, id.String())
	require.NoError(t, err)
	var out []string
	for rows.Next() {
		var r string
		require.NoError(t, rows.Scan(&r))
		out = append(out, r)
	}
	require.NoError(t, rows.Err())
	return out
}

// Owner and admin terminate any session of their organization, a member only
// their own; a session the caller may not see — another member's, another
// organization's or an unknown id — is ErrNotFound and changes nothing.
func TestTerminateSession(t *testing.T) {
	e := newTermEnv(t)
	ctx := t.Context()
	admins := e.a.StartSession(t, e.pool, e.a.AdminID)

	for name, c := range map[string]struct {
		p  uuid.UUID
		id uuid.UUID
	}{"member foreign": {e.a.MemberID, e.foreign}, "unknown": {e.a.OwnerID, uuid.New()}} {
		require.ErrorIs(t, e.svc.TerminateSession(ctx, e.a.Principal(c.p), c.id, "maintenance"), apperr.ErrNotFound, name)
	}
	require.ErrorIs(t, e.svc.TerminateSession(ctx, e.b.Principal(e.b.OwnerID), e.mine, "maintenance"), apperr.ErrNotFound,
		"another organization's owner")
	assert.Empty(t, e.sent(t))
	at, _ := e.requested(t, e.foreign)
	assert.Nil(t, at)

	require.NoError(t, e.svc.TerminateSession(ctx, e.a.Principal(e.a.OwnerID), e.foreign, "maintenance"))
	assert.Equal(t, []string{e.note(e.foreign)}, e.sent(t))
	assert.Equal(t, []string{"maintenance"}, e.audits(t, e.foreign))
	at, reason := e.requested(t, e.foreign)
	require.NotNil(t, at)
	assert.Equal(t, "maintenance", *reason)

	require.NoError(t, e.svc.TerminateSession(ctx, e.a.Principal(e.a.AdminID), admins, "audit"))
	require.NoError(t, e.svc.TerminateSession(ctx, e.a.Principal(e.a.MemberID), e.mine, "done"))
	assert.Equal(t, []string{e.note(admins), e.note(e.mine)}, e.sent(t))
	assert.Equal(t, []string{"done"}, e.audits(t, e.mine))
}

// An agent token never terminates a session, not even its sponsor's own.
func TestTerminateSessionRefusesAgent(t *testing.T) {
	e := newTermEnv(t)
	p := e.a.Principal(e.a.MemberID)
	run := uuid.New()
	p.AgentID, p.RunID = &run, &run
	require.ErrorIs(t, e.svc.TerminateSession(t.Context(), p, e.mine, "x"), apperr.ErrPermissionDenied)
	assert.Empty(t, e.sent(t))
	assert.Empty(t, e.audits(t, e.mine))
	at, _ := e.requested(t, e.mine)
	assert.Nil(t, at)
}

// An empty reason or one over 256 characters is refused; the limit counts characters.
func TestTerminateSessionReason(t *testing.T) {
	e := newTermEnv(t)
	p := e.a.Principal(e.a.OwnerID)
	for _, r := range []string{"", strings.Repeat("é", 257)} {
		require.ErrorIs(t, e.svc.TerminateSession(t.Context(), p, e.mine, r), apperr.ErrInvalidArgument)
	}
	require.NoError(t, e.svc.TerminateSession(t.Context(), p, e.mine, strings.Repeat("é", 256)), "256 characters, 512 bytes")
}

// An ended session is ErrSessionEnded: no NOTIFY, no audit, no request.
func TestTerminateEndedSession(t *testing.T) {
	e := newTermEnv(t)
	require.NoError(t, e.svc.End(t.Context(), e.a.OrgID, e.a.InstanceID, e.mine, "closed", nil, nil))
	err := e.svc.TerminateSession(t.Context(), e.a.Principal(e.a.OwnerID), e.mine, "maintenance")
	require.ErrorIs(t, err, session.ErrSessionEnded)
	require.ErrorIs(t, err, apperr.ErrFailedPrecondition)
	assert.EqualError(t, err, "session already ended: failed precondition")
	assert.Empty(t, e.sent(t))
	assert.Empty(t, e.audits(t, e.mine))
	at, _ := e.requested(t, e.mine)
	assert.Nil(t, at)
}

// A repeat on an open session sends the NOTIFY again but keeps the first
// reason and its one audit event.
func TestTerminateTwice(t *testing.T) {
	e := newTermEnv(t)
	p := e.a.Principal(e.a.OwnerID)
	require.NoError(t, e.svc.TerminateSession(t.Context(), p, e.mine, "first"))
	require.NoError(t, e.svc.TerminateSession(t.Context(), p, e.mine, "second"))
	assert.Equal(t, []string{e.note(e.mine), e.note(e.mine)}, e.sent(t))
	assert.Equal(t, []string{"first"}, e.audits(t, e.mine))
	_, reason := e.requested(t, e.mine)
	assert.Equal(t, "first", *reason)
}

// A session whose instance is gone gets the request but no NOTIFY.
func TestTerminateWithoutInstance(t *testing.T) {
	e := newTermEnv(t)
	_, err := e.pool.Exec(t.Context(), "UPDATE session SET gateway_instance_id = NULL WHERE id = $1", e.mine)
	require.NoError(t, err)
	require.NoError(t, e.svc.TerminateSession(t.Context(), e.a.Principal(e.a.OwnerID), e.mine, "maintenance"))
	assert.Empty(t, e.sent(t))
	at, _ := e.requested(t, e.mine)
	assert.NotNil(t, at)
}

// PendingTerminations returns, by id, the open sessions of the instance with
// a terminate request: never an ended one, another instance's or another
// organization's.
func TestPendingTerminations(t *testing.T) {
	e := newTermEnv(t)
	ctx := t.Context()
	owner := e.a.Principal(e.a.OwnerID)
	ended := e.a.StartSession(t, e.pool, e.a.AdminID)
	other := e.a.StartSession(t, e.pool, e.a.OwnerID)
	_, err := e.pool.Exec(ctx, "UPDATE session SET gateway_id = $2, gateway_instance_id = $3 WHERE id = $1",
		other, e.a.OtherGatewayID, e.a.OtherInstanceID)
	require.NoError(t, err)
	idB := e.b.StartSession(t, e.pool, e.b.MemberID)
	for _, id := range []uuid.UUID{e.foreign, e.mine, ended, other} {
		require.NoError(t, e.svc.TerminateSession(ctx, owner, id, "maintenance"))
	}
	require.NoError(t, e.svc.TerminateSession(ctx, e.b.Principal(e.b.OwnerID), idB, "maintenance"))
	require.NoError(t, e.svc.End(ctx, e.a.OrgID, e.a.InstanceID, ended, "terminated", nil, nil))

	got, err := e.svc.PendingTerminations(ctx, e.a.OrgID, e.a.InstanceID)
	require.NoError(t, err)
	want := []uuid.UUID{e.mine, e.foreign}
	if want[1].String() < want[0].String() {
		want[0], want[1] = want[1], want[0]
	}
	assert.Equal(t, want, got)

	got, err = e.svc.PendingTerminations(ctx, e.a.OrgID, e.b.InstanceID)
	require.NoError(t, err)
	assert.Empty(t, got, "another organization's instance")
}
