package session

import (
	"context"
	"net/netip"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/platform/db/dbtest"
)

// sqlCount counts the statements a pool sends.
type sqlCount struct {
	mu sync.Mutex
	n  int
}

func (c *sqlCount) TraceQueryStart(ctx context.Context, _ *pgx.Conn, _ pgx.TraceQueryStartData) context.Context {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.n++
	return ctx
}

func (*sqlCount) TraceQueryEnd(context.Context, *pgx.Conn, pgx.TraceQueryEndData) {}

func (c *sqlCount) take() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	n := c.n
	c.n = 0
	return n
}

// Start writes the session and its audit row in one statement: BEGIN with the
// context, the break-glass check, the insert, COMMIT. The audit row keeps every
// column of auditlog.Write.
func TestStartWritesSessionAndAuditInOneStatement(t *testing.T) {
	setupPool, url := dbtest.New(t)
	cfg, err := pgxpool.ParseConfig(url)
	require.NoError(t, err)
	trace := &sqlCount{}
	cfg.ConnConfig.Tracer = trace
	pool, err := pgxpool.NewWithConfig(bg, cfg)
	require.NoError(t, err)
	t.Cleanup(pool.Close)

	org := dbtest.Org(t, setupPool)
	gw := dbtest.Gateway(t, setupPool, org, "edge")
	user, res, inst := dbtest.User(t, setupPool, org, "member"), dbtest.Resource(t, setupPool, org, "web", nil), dbtest.Instance(t, setupPool, org, gw)
	trace.take()

	id, err := New(pool).Start(bg, Start{OrgID: org, UserID: user, ResourceID: res, GatewayID: gw, InstanceID: inst,
		ClientIP: netip.MustParseAddr("10.0.0.7"), Kind: "stream", Target: "db:5432"})
	require.NoError(t, err)
	assert.Equal(t, 4, trace.take(), "BEGIN, the break-glass check, the insert, COMMIT")

	var a struct {
		org, actor                          uuid.UUID
		kind, action, typ, target, ip, data string
	}
	require.NoError(t, setupPool.QueryRow(bg, `SELECT org_id, actor_user_id, actor_kind, action, target_type, target_id, host(ip), data::text
		FROM audit_event WHERE action = 'session.start'`).Scan(&a.org, &a.actor, &a.kind, &a.action, &a.typ, &a.target, &a.ip, &a.data))
	assert.Equal(t, org, a.org)
	assert.Equal(t, user, a.actor)
	assert.Equal(t, "user", a.kind)
	assert.Equal(t, "resource", a.typ)
	assert.Equal(t, res.String(), a.target)
	assert.Equal(t, "10.0.0.7", a.ip)
	assert.JSONEq(t, `{"session_id":"`+id.String()+`","kind":"stream","target":"db:5432","transport":"tcp"}`, a.data)

	var n int
	require.NoError(t, setupPool.QueryRow(bg, `SELECT count(*) FROM session WHERE id = $1 AND org_id = $2 AND user_id = $3
		AND resource_id = $4 AND gateway_id = $5 AND gateway_instance_id = $6 AND client_ip = '10.0.0.7' AND kind = 'stream'
		AND target = 'db:5432' AND ended_at IS NULL`, id, org, user, res, gw, inst).Scan(&n))
	assert.Equal(t, 1, n)
}
