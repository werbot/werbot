package session_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/platform/apperr"
	"github.com/werbot/werbot/internal/platform/clientip"
	"github.com/werbot/werbot/internal/platform/db/dbtest"
	"github.com/werbot/werbot/internal/platform/principal"
	"github.com/werbot/werbot/internal/session"
)

// ticketEnv is org A with an online instance serving ssh, sessions of member
// and otherMember, and org B.
type ticketEnv struct {
	pool          *pgxpool.Pool
	svc           *session.Service
	a, b          dbtest.Fixture
	mine, foreign uuid.UUID // sessions of a.MemberID and a.OtherMemberID
}

func newTicketEnv(t *testing.T) ticketEnv {
	t.Helper()
	pool, _ := dbtest.New(t)
	e := ticketEnv{pool: pool, svc: session.New(pool), a: dbtest.Seed(t, pool), b: dbtest.Seed(t, pool)}
	e.mine, e.foreign = e.a.StartSession(t, pool, e.a.MemberID), e.a.StartSession(t, pool, e.a.OtherMemberID)
	e.a.OnlineInstance(t, pool, e.a.InstanceID, "ssh")
	return e
}

// ipCtx is a request context whose client address is 192.0.2.7.
func ipCtx(t *testing.T) context.Context {
	t.Helper()
	ctx := t.Context()
	r := httptest.NewRequestWithContext(ctx, http.MethodGet, "/", nil)
	r.RemoteAddr = "192.0.2.7:1234"
	clientip.Middleware(nil)(http.HandlerFunc(func(_ http.ResponseWriter, req *http.Request) { ctx = req.Context() })).ServeHTTP(nil, r)
	return ctx
}

func (e ticketEnv) count(t *testing.T, sql string, args ...any) int {
	t.Helper()
	var n int
	require.NoError(t, e.pool.QueryRow(t.Context(), sql, args...).Scan(&n))
	return n
}

func (e ticketEnv) tickets(t *testing.T) int { return e.count(t, "SELECT count(*) FROM stream_ticket") }

func (e ticketEnv) observeAudits(t *testing.T) int {
	return e.count(t, "SELECT count(*) FROM audit_event WHERE action = 'session.observe'")
}

func (e ticketEnv) expire(t *testing.T, id uuid.UUID, ago string) {
	t.Helper()
	_, err := e.pool.Exec(t.Context(), "UPDATE stream_ticket SET expires_at = now() - $2::interval WHERE id = $1", id, ago)
	require.NoError(t, err)
}

func (e ticketEnv) terminal(t *testing.T, userID uuid.UUID) session.StreamTicket {
	t.Helper()
	tk, err := e.svc.CreateTerminalTicket(t.Context(), e.a.Principal(userID), "web", 120, 40, uuid.New())
	require.NoError(t, err)
	return tk
}

// Owner and admin get an observe ticket for any live session of the
// organization, a member only for their own; it lives 30s, is bound to the
// session's instance and audited as session.observe. A session the caller
// may not see — another member's, another organization's, unknown — is
// ErrNotFound with no ticket and no audit.
func TestCreateObserveTicket(t *testing.T) {
	e := newTicketEnv(t)
	ctx, target := ipCtx(t), uuid.New()
	tk, err := e.svc.CreateObserveTicket(ctx, e.a.Principal(e.a.OwnerID), e.foreign, target)
	require.NoError(t, err)
	assert.Equal(t, "observe", tk.Mode)
	assert.Equal(t, []uuid.UUID{e.a.OrgID, e.a.OwnerID, e.a.InstanceID, target}, []uuid.UUID{tk.OrgID, tk.UserID, tk.InstanceID, tk.TargetServerID})
	assert.Equal(t, &e.foreign, tk.SessionID)
	assert.Nil(t, tk.ResourceID)
	assert.Equal(t, netip.MustParseAddr("192.0.2.7"), tk.ClientIP)

	var (
		mode     string
		inst, tg uuid.UUID
		ip       netip.Addr
		ttl      float64
	)
	require.NoError(t, e.pool.QueryRow(t.Context(), `SELECT mode, gateway_instance_id, target_server_id, client_ip,
		extract(epoch FROM expires_at - now())::float8 FROM stream_ticket WHERE id = $1`, tk.ID).Scan(&mode, &inst, &tg, &ip, &ttl))
	assert.Equal(t, "observe", mode)
	assert.Equal(t, []uuid.UUID{e.a.InstanceID, target}, []uuid.UUID{inst, tg})
	assert.Equal(t, netip.MustParseAddr("192.0.2.7"), ip)
	assert.Greater(t, ttl, 29.0)
	assert.LessOrEqual(t, ttl, 31.0)

	var data map[string]any
	require.NoError(t, e.pool.QueryRow(t.Context(), `SELECT data FROM audit_event WHERE action = 'session.observe'
		AND target_type = 'session' AND target_id = $1 AND actor_user_id = $2`, e.foreign.String(), e.a.OwnerID).Scan(&data))
	assert.Equal(t, map[string]any{"session_id": e.foreign.String()}, data)

	_, err = e.svc.CreateObserveTicket(ctx, e.a.Principal(e.a.AdminID), e.mine, target)
	require.NoError(t, err, "admin, a member's session")
	_, err = e.svc.CreateObserveTicket(ctx, e.a.Principal(e.a.MemberID), e.mine, target)
	require.NoError(t, err, "member, own session")

	for name, c := range map[string]struct {
		p  principal.Principal
		id uuid.UUID
	}{
		"member, another member's session": {e.a.Principal(e.a.MemberID), e.foreign},
		"another organization's owner":     {e.b.Principal(e.b.OwnerID), e.mine},
		"unknown session":                  {e.a.Principal(e.a.OwnerID), uuid.New()},
	} {
		_, err := e.svc.CreateObserveTicket(ctx, c.p, c.id, target)
		require.ErrorIs(t, err, apperr.ErrNotFound, name)
	}
	assert.Equal(t, 3, e.tickets(t))
	assert.Equal(t, 3, e.observeAudits(t))
}

// Only a live terminal session can be observed: an ended session, an exec
// session and one without a gateway instance are ErrNotFound.
func TestCreateObserveTicketNotLive(t *testing.T) {
	e := newTicketEnv(t)
	ctx, owner := t.Context(), e.a.Principal(e.a.OwnerID)
	require.NoError(t, e.svc.End(ctx, e.a.OrgID, e.a.InstanceID, e.mine, "closed", nil, nil))
	_, err := e.pool.Exec(ctx, "UPDATE session SET kind = 'exec', command = 'id' WHERE id = $1", e.foreign)
	require.NoError(t, err)
	orphan := e.a.StartSession(t, e.pool, e.a.AdminID)
	_, err = e.pool.Exec(ctx, "UPDATE session SET gateway_instance_id = NULL WHERE id = $1", orphan)
	require.NoError(t, err)

	for name, id := range map[string]uuid.UUID{"ended": e.mine, "exec": e.foreign, "no instance": orphan} {
		_, err := e.svc.CreateObserveTicket(ctx, owner, id, uuid.New())
		require.ErrorIs(t, err, apperr.ErrNotFound, name)
	}
	assert.Zero(t, e.tickets(t))
	assert.Zero(t, e.observeAudits(t))
}

// An agent token never gets a stream ticket, not even for its sponsor's own
// session or a resource of the organization.
func TestCreateTicketRefusesAgent(t *testing.T) {
	e := newTicketEnv(t)
	p := e.a.Principal(e.a.MemberID)
	run := uuid.New()
	p.AgentID, p.RunID = &run, &run
	_, err := e.svc.CreateObserveTicket(t.Context(), p, e.mine, uuid.New())
	require.ErrorIs(t, err, apperr.ErrPermissionDenied)
	_, err = e.svc.CreateTerminalTicket(t.Context(), p, "web", 80, 24, uuid.New())
	require.ErrorIs(t, err, apperr.ErrPermissionDenied)
	assert.Zero(t, e.tickets(t))
	assert.Zero(t, e.observeAudits(t))
}

// A terminal ticket goes to a random online instance of the resource's pool
// that serves the resource type, keeps the window and writes no audit. An
// unknown resource is ErrNotFound; an existing one without an online instance
// or without a pool is ErrUnavailable.
func TestCreateTerminalTicket(t *testing.T) {
	e := newTicketEnv(t)
	ctx, owner := t.Context(), e.a.Principal(e.a.OwnerID)
	second, noSSH := dbtest.Instance(t, e.pool, e.a.OrgID, e.a.GatewayID), dbtest.Instance(t, e.pool, e.a.OrgID, e.a.GatewayID)
	e.a.OnlineInstance(t, e.pool, second, "telnet", "ssh")
	e.a.OnlineInstance(t, e.pool, noSSH, "telnet")
	e.a.OnlineInstance(t, e.pool, e.a.OtherInstanceID, "ssh") // another pool
	audits := e.count(t, "SELECT count(*) FROM audit_event")

	seen := map[uuid.UUID]bool{}
	for range 40 {
		tk := e.terminal(t, e.a.MemberID)
		seen[tk.InstanceID] = true
		assert.Equal(t, "web", tk.Mode)
		assert.Equal(t, &e.a.ResourceID, tk.ResourceID)
		assert.Nil(t, tk.SessionID)
		assert.Equal(t, []int{120, 40}, []int{tk.Cols, tk.Rows})
	}
	assert.Equal(t, map[uuid.UUID]bool{e.a.InstanceID: true, second: true}, seen)
	assert.Equal(t, 40, e.count(t, "SELECT count(*) FROM stream_ticket WHERE cols = 120 AND rows = 40 AND mode = 'web'"))
	assert.Equal(t, audits, e.count(t, "SELECT count(*) FROM audit_event"), "no audit")

	_, err := e.svc.CreateTerminalTicket(ctx, owner, "missing", 80, 24, uuid.New())
	require.ErrorIs(t, err, apperr.ErrNotFound)
	_, err = e.svc.CreateTerminalTicket(ctx, e.b.Principal(e.b.OwnerID), "missing", 80, 24, uuid.New())
	require.ErrorIs(t, err, apperr.ErrNotFound, "another organization")

	for _, w := range [][2]int{{0, 24}, {501, 24}, {80, 0}, {80, 201}} {
		_, err = e.svc.CreateTerminalTicket(ctx, owner, "web", w[0], w[1], uuid.New())
		require.ErrorIs(t, err, apperr.ErrInvalidArgument, "window %v", w)
	}

	dbtest.Resource(t, e.pool, e.a.OrgID, "nopool", nil)
	_, err = e.svc.CreateTerminalTicket(ctx, owner, "nopool", 80, 24, uuid.New())
	require.ErrorIs(t, err, apperr.ErrUnavailable, "resource without a pool")

	_, err = e.pool.Exec(ctx, "UPDATE gateway_instance SET last_seen_at = now() - interval '46 seconds' WHERE gateway_id = $1", e.a.GatewayID)
	require.NoError(t, err)
	_, err = e.svc.CreateTerminalTicket(ctx, owner, "web", 80, 24, uuid.New())
	require.ErrorIs(t, err, apperr.ErrUnavailable, "pool offline")
}

// A ticket is spent once: of 10 concurrent Consume calls exactly one gets the
// ticket; an expired or unknown ticket is ErrNotFound.
func TestConsumeStreamTicketOnce(t *testing.T) {
	e := newTicketEnv(t)
	tk := e.terminal(t, e.a.MemberID)
	var (
		wg   sync.WaitGroup
		mu   sync.Mutex
		won  []session.StreamTicket
		lost int
	)
	for range 10 {
		wg.Go(func() {
			got, err := e.svc.ConsumeStreamTicket(context.Background(), tk.ID)
			mu.Lock()
			defer mu.Unlock()
			if err == nil {
				won = append(won, got)
				return
			}
			assert.ErrorIs(t, err, apperr.ErrNotFound)
			lost++
		})
	}
	wg.Wait()
	require.Len(t, won, 1)
	assert.Equal(t, 9, lost)
	assert.Equal(t, []uuid.UUID{tk.ID, tk.OrgID, tk.UserID, tk.InstanceID, tk.TargetServerID},
		[]uuid.UUID{won[0].ID, won[0].OrgID, won[0].UserID, won[0].InstanceID, won[0].TargetServerID})
	assert.Equal(t, tk.ResourceID, won[0].ResourceID)

	expired := e.terminal(t, e.a.MemberID)
	e.expire(t, expired.ID, "1 second")
	for name, id := range map[string]uuid.UUID{"expired": expired.ID, "unknown": uuid.New()} {
		_, err := e.svc.ConsumeStreamTicket(t.Context(), id)
		require.ErrorIs(t, err, apperr.ErrNotFound, name)
	}
}

// Lookup is a repeatable read bound to the ticket's organization and
// instance: it never spends the ticket and refuses an expired one.
func TestLookupStreamTicket(t *testing.T) {
	e := newTicketEnv(t)
	ctx := t.Context()
	tk := e.terminal(t, e.a.MemberID)
	var login string
	require.NoError(t, e.pool.QueryRow(ctx, "SELECT login FROM users WHERE id = $1", e.a.MemberID).Scan(&login))

	for range 2 {
		got, err := e.svc.LookupStreamTicket(ctx, e.a.OrgID, e.a.InstanceID, tk.ID)
		require.NoError(t, err)
		assert.Equal(t, login, got.Login)
		assert.Equal(t, "web", got.ResourceName)
		assert.Equal(t, []int{120, 40}, []int{got.Cols, got.Rows})
		assert.Equal(t, tk.ID, got.ID)
	}

	for name, c := range map[string][2]uuid.UUID{
		"another instance":     {e.a.OrgID, e.a.OtherInstanceID},
		"another organization": {e.b.OrgID, e.a.InstanceID},
	} {
		_, err := e.svc.LookupStreamTicket(ctx, c[0], c[1], tk.ID)
		require.ErrorIs(t, err, apperr.ErrNotFound, name)
	}

	_, err := e.svc.ConsumeStreamTicket(ctx, tk.ID)
	require.NoError(t, err)
	_, err = e.svc.LookupStreamTicket(ctx, e.a.OrgID, e.a.InstanceID, tk.ID)
	require.NoError(t, err, "a spent ticket is still found")

	e.expire(t, tk.ID, "1 second")
	_, err = e.svc.LookupStreamTicket(ctx, e.a.OrgID, e.a.InstanceID, tk.ID)
	require.ErrorIs(t, err, apperr.ErrNotFound, "expired")
}
