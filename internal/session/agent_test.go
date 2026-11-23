package session

import (
	"bytes"
	"context"
	"errors"
	"net/netip"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/platform/db/dbtest"
)

// agentRun inserts an agent of e.user with a running run.
func (e env) agentRun(t *testing.T, name string) uuid.UUID {
	t.Helper()
	return dbtest.AgentRun(t, e.pool, e.org, dbtest.Agent(t, e.pool, e.org, e.user, name), 10, 4)
}

// toolCall inserts a running tool call of run.
func (e env) toolCall(t *testing.T, run uuid.UUID) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	require.NoError(t, e.pool.QueryRow(bg, `INSERT INTO agent_tool_call (org_id, run_id, tool, status)
		VALUES ($1, $2, 'ssh_exec', 'running') RETURNING id`, e.org, run).Scan(&id))
	return id
}

func (e env) startAgent(run, toolCall *uuid.UUID, guard func(context.Context, pgx.Tx) error) (uuid.UUID, error) {
	return e.svc.Start(bg, Start{OrgID: e.org, UserID: e.user, ResourceID: e.res, GatewayID: e.gw, InstanceID: e.a,
		ClientIP: netip.MustParseAddr("10.0.0.1"), Kind: "exec", Command: "ls", AgentRunID: run, ToolCallID: toolCall, Guard: guard})
}

// The guard runs in the start transaction before the insert: its error leaves
// no session row, and its writes commit with the session.
func TestStartGuardRunsBeforeInsert(t *testing.T) {
	e := setup(t)
	run := e.agentRun(t, "a1")
	refused := errors.New("refused")
	_, err := e.startAgent(&run, nil, func(ctx context.Context, tx pgx.Tx) error {
		var n int
		require.NoError(t, tx.QueryRow(ctx, "SELECT count(*) FROM session WHERE agent_run_id = $1", run).Scan(&n))
		assert.Zero(t, n, "the guard sees no row of this start")
		return refused
	})
	require.ErrorIs(t, err, refused)
	var n int
	require.NoError(t, e.pool.QueryRow(bg, "SELECT count(*) FROM session").Scan(&n))
	assert.Zero(t, n, "a refused guard leaves no session")

	id, err := e.startAgent(&run, nil, func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, "UPDATE agent_run SET actions = actions + 1 WHERE id = $1", run)
		return err
	})
	require.NoError(t, err)
	var stored *uuid.UUID
	var actions int
	require.NoError(t, e.pool.QueryRow(bg, `SELECT s.agent_run_id, r.actions FROM session s, agent_run r
		WHERE s.id = $1 AND r.id = $2`, id, run).Scan(&stored, &actions))
	assert.Equal(t, &run, stored)
	assert.Equal(t, 1, actions, "the guard's write commits with the session")
}

// A tool call of the session's run is stored; one of another run is stored as NULL.
func TestStartSessionToolCall(t *testing.T) {
	e := setup(t)
	run, other := e.agentRun(t, "a1"), e.agentRun(t, "a2")
	own, foreign := e.toolCall(t, run), e.toolCall(t, other)
	toolCallOf := func(id uuid.UUID) *uuid.UUID {
		var tc *uuid.UUID
		require.NoError(t, e.pool.QueryRow(bg, "SELECT tool_call_id FROM session WHERE id = $1", id).Scan(&tc))
		return tc
	}
	id, err := e.startAgent(&run, &own, nil)
	require.NoError(t, err)
	assert.Equal(t, &own, toolCallOf(id))
	id, err = e.startAgent(&run, &foreign, nil)
	require.NoError(t, err)
	assert.Nil(t, toolCallOf(id), "another run's tool call")
	id, err = e.startAgent(nil, &own, nil)
	require.NoError(t, err)
	assert.Nil(t, toolCallOf(id), "no run")
}

// An exec result stores its exit code and output hash in the first close
// only, with or without a command; a console result is unchanged.
func TestEndSessionExecResult(t *testing.T) {
	e := setup(t)
	result := func(id uuid.UUID) (exit *int32, sum []byte, command *string) {
		require.NoError(t, e.pool.QueryRow(bg, "SELECT exit_code, output_sha256, command FROM session WHERE id = $1", id).
			Scan(&exit, &sum, &command))
		return exit, sum, command
	}
	three, zero, sha := int32(3), int32(0), bytes.Repeat([]byte{7}, 32)

	id := e.open(t, &e.a, 0)
	require.NoError(t, e.svc.End(bg, e.org, e.a, id, "closed", nil, &Result{ExitCode: &three, OutputSHA256: sha}))
	require.NoError(t, e.svc.End(bg, e.org, e.a, id, "closed", nil, &Result{ExitCode: &zero}))
	exit, sum, command := result(id)
	assert.Equal(t, &three, exit, "the first close wins")
	assert.Equal(t, sha, sum)
	assert.Nil(t, command)

	id = e.open(t, &e.a, 0)
	require.NoError(t, e.svc.End(bg, e.org, e.a, id, "closed", nil, &Result{Command: "ls", Classes: []string{"read"}}))
	exit, sum, command = result(id)
	assert.Nil(t, exit, "no exit code")
	assert.Nil(t, sum)
	assert.Equal(t, "ls", *command)
}

// A heartbeat denial of an agent run audits session.deny with data.agent_run_id.
func TestDenyAgentRunID(t *testing.T) {
	e := setup(t)
	run := e.agentRun(t, "a1")
	require.NoError(t, e.svc.Deny(bg, e.org, []Denial{
		{UserID: &e.user, Resource: "web", Kind: "exec", Target: "rm -rf /", Reason: "denied", AgentRunID: &run},
		{UserID: &e.user, Resource: "web", Kind: "exec", Target: "ls", Reason: "denied"},
	}))
	rows, err := e.pool.Query(bg, `SELECT data->>'agent_run_id' FROM audit_event WHERE org_id = $1 AND action = 'session.deny' ORDER BY id`, e.org)
	require.NoError(t, err)
	got, err := pgx.CollectRows(rows, pgx.RowTo[*string])
	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.Equal(t, run.String(), *got[0])
	assert.Nil(t, got[1], "a human denial has no agent_run_id")
}
