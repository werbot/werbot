package app

import (
	"context"
	"errors"
	"strconv"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
	"github.com/werbot/werbot/internal/mcpserver"
	"github.com/werbot/werbot/internal/mcpserver/dbquery"
	"github.com/werbot/werbot/internal/platform/sqlclass/pg"
)

// Criteria 07 of phase 3 in process: the server, two gateway instances of
// one pool, PostgreSQL targets of the test cluster and werbot mcp. The
// classifier-failure criterion is TestAgentClassifierFailureStillReadOnly
// in pgconnector (Ruling C24). The agent tests run in parallel: each waits up
// to two health passes for the agent_readonly status of its database.

// TestPhase3AgentSelect: a verified agent_readonly credential turns ok on
// the first probe; the agent's SELECT returns the rows and is a read line of
// the session's querylog.
func TestPhase3AgentSelect(t *testing.T) {
	t.Parallel()
	s, db, tg, p := dbAgentStack(t)
	s.readonlyStatus(t, db, "ok")
	s.denyDestructiveSQL(t, db, tg)
	m := s.startMCP(t, mcpserver.Options{Profile: p, Agent: "e2e"})

	out := m.query(t, "SELECT count(*) FROM orders")
	assert.Equal(t, dbquery.StatusOK, out.Status)
	assert.Equal(t, [][]any{{"3"}}, out.Rows)

	run := s.agentRun(t)
	m.close(t)
	var sid uuid.UUID
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		require.NoError(c, s.pool.QueryRow(context.Background(), `SELECT id FROM session
			WHERE agent_run_id = $1 AND resource_id = $2 AND ended_at IS NOT NULL`, run, db).Scan(&sid))
	}, 20*time.Second, 100*time.Millisecond)
	line, ok := queryLine(s.querylog(t, s.aliceHTTP(t), sid, tg.roPass), "SELECT count(*) FROM orders")
	require.True(t, ok, "the query is in the querylog")
	assert.Equal(t, []string{"read"}, line.Classes)
}

// TestPhase3AgentDropTableDenied: the agent baseline denies DROP TABLE; the
// table stays, the denial is counted and audited with the run.
func TestPhase3AgentDropTableDenied(t *testing.T) {
	t.Parallel()
	s, db, tg, p := dbAgentStack(t)
	s.readonlyStatus(t, db, "ok")
	s.denyDestructiveSQL(t, db, tg)
	m := s.startMCP(t, mcpserver.Options{Profile: p, Agent: "e2e"})

	out := m.query(t, "DROP TABLE orders")
	assert.Equal(t, dbquery.StatusDenied, out.Status)
	assert.Equal(t, "a", tg.note(t, 1), "the table exists")

	ctx, run := context.Background(), s.agentRun(t)
	require.EventuallyWithT(t, func(c *assert.CollectT) { // agent denials reach the server in the background
		var denials, audits int
		require.NoError(c, s.pool.QueryRow(ctx, "SELECT denials_in_row FROM agent_run WHERE id = $1", run).Scan(&denials))
		require.NoError(c, s.pool.QueryRow(ctx, `SELECT count(*) FROM audit_event WHERE action = 'session.deny'
			AND data->>'agent_run_id' = $1`, run.String()).Scan(&audits))
		assert.Equal(c, [2]int{1, 1}, [2]int{denials, audits})
	}, 10*time.Second, 100*time.Millisecond)
}

// TestPhase3AgentUpdateWaitsForApproval: under an approval rule for agents
// on env=prod an UPDATE waits for a person; approved, it runs as the
// primary role; the run reuses the approval for 10 minutes, then waits again.
func TestPhase3AgentUpdateWaitsForApproval(t *testing.T) {
	t.Parallel()
	s, db, tg, p := dbAgentStack(t)
	carol := s.addAdmin(t, "carol")
	_, err := s.api.pol.CreateCommandRule(context.Background(), connect.NewRequest(&werbotv1.CreateCommandRuleRequest{
		Name: "prod-writes", Effect: "approval", Classes: []string{"write"}, Subjects: &werbotv1.Subjects{AllAgents: true},
		ResourceSelector: &werbotv1.ResourceSelector{Labels: map[string]string{"env": "prod"}}}))
	require.NoError(t, err)
	requestPolicy(t, signIn(t, s.url, "alice@example.com"), "db-queries", false, db)
	s.readonlyStatus(t, db, "ok")
	s.denyDestructiveSQL(t, db, tg)
	m := s.startMCP(t, mcpserver.Options{Profile: p, Agent: "e2e"})

	const update = "UPDATE orders SET note = current_user WHERE id = 1"
	approve := func(id string) error {
		_, err := carol.req.Approve(context.Background(), connect.NewRequest(&werbotv1.ApproveRequest{Id: id}))
		return err
	}
	deny := func(id string) error {
		_, err := carol.req.Deny(context.Background(), connect.NewRequest(&werbotv1.DenyRequest{Id: id}))
		return err
	}
	id := s.decideQuery(t, m, update, approve, dbquery.StatusOK)
	assert.Equal(t, tg.cfg.GetUsername(), tg.note(t, 1), "the approved write runs as the primary role")
	assert.Equal(t, dbquery.StatusOK, m.query(t, update).Status, "the run reuses its approval")

	_, err = s.pool.Exec(context.Background(), "UPDATE approval SET decided_at = decided_at - interval '11 minutes' WHERE id = $1", id)
	require.NoError(t, err)
	next := s.decideQuery(t, m, update, deny, dbquery.StatusDenied)
	assert.NotEqual(t, id, next, "an approval older than 10 minutes is not reused")
}

// decideQuery runs sql through db_query while decide settles the approval it
// waits for; the call must end with want. It returns the approval id.
func (s *stack) decideQuery(t *testing.T, m *mcpClient, sql string, decide func(id string) error, want string) string {
	t.Helper()
	done := make(chan dbquery.Result, 1)
	go func() { done <- m.query(t, sql) }()
	id := s.pendingQuery(t, sql)
	require.NoError(t, decide(id))
	select {
	case out := <-done:
		assert.Equal(t, want, out.Status)
	case <-time.After(10 * time.Second):
		t.Fatal("the held query did not end within 10s of the decision")
	}
	return id
}

// TestPhase3AgentWithoutVerifiedCredential: a credential that can write is
// has_write; the agent does not see the database and is denied.
func TestPhase3AgentWithoutVerifiedCredential(t *testing.T) {
	t.Parallel()
	s, db, _, p := dbAgentStack(t, "GRANT INSERT ON orders TO %s")
	r := s.readonlyStatus(t, db, "has_write")
	assert.Contains(t, r.GetAgentReadonlyDetail(), "INSERT on")
	m := s.startMCP(t, mcpserver.Options{Profile: p, Agent: "e2e"})

	list := m.call(t, "list_resources", map[string]any{})
	require.False(t, list.IsError, toolText(list))
	assert.NotContains(t, toolText(list), `"db"`, "the agent does not see the database")
	q := m.call(t, "db_query", map[string]any{"resource": "db", "query": "SELECT 1"})
	assert.True(t, q.IsError)
	assert.Equal(t, "resource db not found", toolText(q))

	var agent string
	require.NoError(t, s.pool.QueryRow(context.Background(), "SELECT id::text FROM agent WHERE name = 'e2e'").Scan(&agent))
	res, err := s.api.pol.TestCommand(context.Background(), connect.NewRequest(&werbotv1.TestCommandRequest{
		Subject: &werbotv1.TestCommandRequest_AgentId{AgentId: agent}, ResourceId: db.String(), Command: "SELECT 1"}))
	require.NoError(t, err)
	assert.False(t, res.Msg.GetAllow())
	assert.Contains(t, res.Msg.GetReasons(), "agent:baseline:db_readonly")
}

// TestPhase3HumanQueryRule: without query rules a person's queries take the
// fast path and the querylog still has their classes; a deny rule on
// destructive refuses DROP TABLE with 42501 and keeps the session.
func TestPhase3HumanQueryRule(t *testing.T) {
	s := startStack(t)
	tg := newPGTarget(t)
	db := s.addDB(t, tg)
	s.resourceReady(t, "db")
	// the gateway warms the parser in the background once its snapshot holds
	// a database; until then every query is opaque
	require.NoError(t, pg.Warmup())
	ctx := context.Background()

	conn := s.pgConnect(t, "db:"+strconv.Itoa(int(tg.cfg.GetPort())), tg.cfg.GetDatabase())
	var n int
	require.NoError(t, conn.QueryRow(ctx, "SELECT count(*) FROM orders").Scan(&n))
	assert.Equal(t, 3, n)
	require.NoError(t, conn.Close(ctx))
	line, ok := queryLine(s.querylog(t, s.aliceHTTP(t), endedSession(t, s, db), tg.password), "SELECT count(*) FROM orders")
	require.True(t, ok, "the query is in the querylog")
	assert.Equal(t, []string{"read"}, line.Classes)

	s.denyDestructiveSQL(t, db, tg)
	conn, err := s.pgDial(t, tg)
	require.NoError(t, err)
	defer conn.Close(ctx) //nolint:errcheck // test client
	_, err = conn.Exec(ctx, "DROP TABLE orders")
	pe, ok := errors.AsType[*pgconn.PgError](err)
	require.True(t, ok, "not a server error: %v", err)
	assert.Equal(t, [2]string{"42501", "werbot: query denied: " + denySQLMessage}, [2]string{pe.Code, pe.Message})
	require.NoError(t, conn.QueryRow(ctx, "SELECT count(*) FROM orders").Scan(&n), "the session goes on")
	assert.Equal(t, 3, n)
}
