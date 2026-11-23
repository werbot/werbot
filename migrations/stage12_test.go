package migrations_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/platform/db/dbtest"
	"github.com/werbot/werbot/migrations"
)

const insertAgent = `INSERT INTO agent (org_id, sponsor_user_id, name, status) VALUES ($1, $2, $3, $4)`

func TestMigrations12UpDown(t *testing.T) {
	pool, _ := dbtest.New(t)
	ctx := context.Background()
	sqlDB := stdlib.OpenDBFromPool(pool)
	defer func() { _ = sqlDB.Close() }()
	p, err := goose.NewProvider(goose.DialectPostgres, sqlDB, migrations.FS)
	require.NoError(t, err)

	_, err = p.DownTo(ctx, 152)
	require.NoError(t, err)
	_, err = p.Up(ctx)
	require.NoError(t, err)
}

func TestAgentNameAndStatusChecks(t *testing.T) {
	pool, _ := dbtest.New(t)
	ctx := context.Background()
	f := dbtest.Seed(t, pool)

	for _, name := range []string{"Claude", "", strings.Repeat("a", 64), "-a"} {
		_, err := pool.Exec(ctx, insertAgent, f.OrgID, f.MemberID, name, "active")
		require.Error(t, err, name)
		pgCode(t, err, "23514")
	}
	_, err := pool.Exec(ctx, insertAgent, f.OrgID, f.MemberID, "ok", "paused")
	pgCode(t, err, "23514")

	_, err = pool.Exec(ctx, insertAgent, f.OrgID, f.MemberID, strings.Repeat("a", 63), "active")
	require.NoError(t, err)
	_, err = pool.Exec(ctx, insertAgent, f.OrgID, f.MemberID, strings.Repeat("a", 63), "active")
	pgCode(t, err, "23505")
}

func TestAgentRunEndedCheck(t *testing.T) {
	pool, _ := dbtest.New(t)
	ctx := context.Background()
	f := dbtest.Seed(t, pool)
	agent := dbtest.Agent(t, pool, f.OrgID, f.MemberID, "a")

	const q = `INSERT INTO agent_run (org_id, agent_id, sponsor_user_id, client, status, max_actions, max_concurrent, expires_at, ended_at)
		VALUES ($1, $2, $3, 'c', $4, 1, 1, now(), $5)`
	_, err := pool.Exec(ctx, q, f.OrgID, agent, f.MemberID, "ended", nil)
	pgCode(t, err, "23514")
	_, err = pool.Exec(ctx, q, f.OrgID, agent, f.MemberID, "running", time.Now())
	pgCode(t, err, "23514")
	_, err = pool.Exec(ctx, q, f.OrgID, agent, f.MemberID, "ended", time.Now())
	require.NoError(t, err)
}

func TestAgentCrossOrgRejected(t *testing.T) {
	pool, _ := dbtest.New(t)
	ctx := context.Background()
	f := dbtest.Seed(t, pool)
	other := dbtest.Seed(t, pool)
	agent := dbtest.Agent(t, pool, f.OrgID, f.MemberID, "a")
	run := dbtest.AgentRun(t, pool, f.OrgID, agent, 10, 2)

	_, err := pool.Exec(ctx, `INSERT INTO agent_run (org_id, agent_id, sponsor_user_id, client, status, max_actions, max_concurrent, expires_at)
		VALUES ($1, $2, $3, 'c', 'running', 1, 1, now())`, other.OrgID, agent, other.MemberID)
	pgCode(t, err, "23503")

	_, err = pool.Exec(ctx, `INSERT INTO agent (org_id, sponsor_user_id, name) VALUES ($1, $2, 'x')`, other.OrgID, f.MemberID)
	pgCode(t, err, "23503")

	_, err = pool.Exec(ctx, `INSERT INTO agent_tool_call (org_id, run_id, tool, status) VALUES ($1, $2, 't', 'running')`, other.OrgID, run)
	pgCode(t, err, "23503")

	_, err = pool.Exec(ctx, `INSERT INTO session (org_id, client_ip, kind, agent_run_id) VALUES ($1, '10.0.0.1', 'terminal', $2)`, other.OrgID, run)
	pgCode(t, err, "23503")

	_, err = pool.Exec(ctx, `INSERT INTO session (org_id, client_ip, kind, output_sha256) VALUES ($1, '10.0.0.1', 'terminal', '\x00')`, f.OrgID)
	pgCode(t, err, "23514")
}

func TestAgentCascades(t *testing.T) {
	pool, _ := dbtest.New(t)
	ctx := context.Background()
	f := dbtest.Seed(t, pool)
	agent := dbtest.Agent(t, pool, f.OrgID, f.MemberID, "a")
	run := dbtest.AgentRun(t, pool, f.OrgID, agent, 10, 2)

	var call, sess, appr uuid.UUID
	require.NoError(t, pool.QueryRow(ctx, `INSERT INTO agent_tool_call (org_id, run_id, tool, status) VALUES ($1, $2, 't', 'running') RETURNING id`, f.OrgID, run).Scan(&call))
	require.NoError(t, pool.QueryRow(ctx, `INSERT INTO session (org_id, user_id, client_ip, kind, agent_run_id, tool_call_id, exit_code, output_sha256)
		VALUES ($1, $2, '10.0.0.1', 'terminal', $3, $4, 0, decode(repeat('00', 32), 'hex')) RETURNING id`, f.OrgID, f.MemberID, run, call).Scan(&sess))
	require.NoError(t, pool.QueryRow(ctx, `INSERT INTO approval (org_id, kind, requester_user_id, agent_run_id, status, expires_at)
		VALUES ($1, 'command', $2, $3, 'pending', now() + interval '1 hour') RETURNING id`, f.OrgID, f.OtherMemberID, run).Scan(&appr))

	_, err := pool.Exec(ctx, `DELETE FROM membership WHERE org_id = $1 AND user_id = $2`, f.OrgID, f.MemberID)
	require.NoError(t, err)

	for _, tbl := range []string{"agent", "agent_run", "agent_tool_call"} {
		var n int
		require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM `+tbl).Scan(&n))
		assert.Zero(t, n, tbl)
	}
	var runID, callID *uuid.UUID
	var org uuid.UUID
	require.NoError(t, pool.QueryRow(ctx, `SELECT org_id, agent_run_id, tool_call_id FROM session WHERE id = $1`, sess).Scan(&org, &runID, &callID))
	assert.Equal(t, f.OrgID, org)
	assert.Nil(t, runID)
	assert.Nil(t, callID)
	var apprRun *uuid.UUID
	require.NoError(t, pool.QueryRow(ctx, `SELECT agent_run_id FROM approval WHERE id = $1`, appr).Scan(&apprRun))
	assert.Nil(t, apprRun)
}

func TestAgentNotifyOnlyOnStatusChange(t *testing.T) {
	pool, _ := dbtest.New(t)
	ctx := context.Background()
	f := dbtest.Seed(t, pool)
	conn, err := pool.Acquire(ctx)
	require.NoError(t, err)
	defer conn.Release()
	_, err = conn.Exec(ctx, "LISTEN werbot_org_changed")
	require.NoError(t, err)

	agent := dbtest.Agent(t, pool, f.OrgID, f.MemberID, "a") // notifies
	run := dbtest.AgentRun(t, pool, f.OrgID, agent, 10, 2)   // no
	for _, q := range []string{
		`UPDATE agent SET limits = '{"a":1}' WHERE id = $1`, // no
		`UPDATE agent SET status = 'frozen' WHERE id = $1`,  // notifies
	} {
		_, err = pool.Exec(ctx, q, agent)
		require.NoError(t, err)
	}
	for _, q := range []string{
		`UPDATE agent_run SET actions = 1, denials_in_row = 1 WHERE id = $1`, // no
		`UPDATE agent_run SET status = 'paused' WHERE id = $1`,               // notifies
	} {
		_, err = pool.Exec(ctx, q, run)
		require.NoError(t, err)
	}

	other := dbtest.Org(t, pool) // marks the end: notifications arrive in commit order
	got := 0
	for {
		wctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		n, err := conn.Conn().WaitForNotification(wctx)
		cancel()
		require.NoError(t, err)
		if n.Payload == other.String() {
			break
		}
		assert.Equal(t, f.OrgID.String(), n.Payload)
		got++
	}
	assert.Equal(t, 3, got)
}
