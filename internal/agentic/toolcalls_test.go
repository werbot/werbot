package agentic_test

import (
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/agentic"
	"github.com/werbot/werbot/internal/platform/apperr"
	"github.com/werbot/werbot/internal/platform/db/dbtest"
	"github.com/werbot/werbot/internal/platform/principal"
)

// agentPrincipal is the run token principal of run of agent in f's organization.
func agentPrincipal(f dbtest.Fixture, agent, run uuid.UUID) principal.Principal {
	p := f.Principal(f.MemberID)
	p.AgentID, p.RunID = &agent, &run
	return p
}

// toolCall is one stored agent_tool_call row.
type toolCall struct {
	Tool, Traceparent, Status string
	Arguments                 map[string]any
	Ended                     bool
}

func toolCallOf(t *testing.T, pool *pgxpool.Pool, id uuid.UUID) toolCall {
	t.Helper()
	var c toolCall
	require.NoError(t, pool.QueryRow(bg,
		"SELECT tool, traceparent, status, arguments, ended_at IS NOT NULL FROM agent_tool_call WHERE id = $1", id).
		Scan(&c.Tool, &c.Traceparent, &c.Status, &c.Arguments, &c.Ended))
	return c
}

func TestToolCalls(t *testing.T) {
	svc, pool, f := newTestService(t)
	agent := dbtest.Agent(t, pool, f.OrgID, f.MemberID, "claude")
	run := dbtest.AgentRun(t, pool, f.OrgID, agent, 10, 4)
	p := agentPrincipal(f, agent, run)
	const trace = "00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01"

	id, err := svc.RecordToolCall(bg, p, agentic.ToolCallInput{Tool: "ssh_exec",
		Arguments: map[string]any{"resource": "web", "command": "uptime"}, Traceparent: trace})
	require.NoError(t, err)
	assert.Equal(t, toolCall{Tool: "ssh_exec", Traceparent: trace, Status: "running",
		Arguments: map[string]any{"resource": "web", "command": "uptime"}}, toolCallOf(t, pool, id))

	require.NoError(t, svc.FinishToolCall(bg, p, id, "ok"))
	got := toolCallOf(t, pool, id)
	assert.Equal(t, "ok", got.Status)
	assert.True(t, got.Ended)
	assert.ErrorIs(t, svc.FinishToolCall(bg, p, id, "error"), apperr.ErrFailedPrecondition, "a finished call")
	assert.ErrorIs(t, svc.FinishToolCall(bg, p, id, "bogus"), apperr.ErrInvalidArgument, "unknown status")

	id, err = svc.RecordToolCall(bg, p, agentic.ToolCallInput{Tool: "list_resources", Traceparent: "x"})
	require.NoError(t, err)
	assert.Equal(t, toolCall{Tool: "list_resources", Status: "running", Arguments: map[string]any{}}, toolCallOf(t, pool, id),
		"a traceparent outside W3C is stored empty")

	otherRun := dbtest.AgentRun(t, pool, f.OrgID, agent, 10, 4)
	assert.ErrorIs(t, svc.FinishToolCall(bg, agentPrincipal(f, agent, otherRun), id, "ok"), apperr.ErrNotFound, "call of another run")
	human := f.Principal(f.MemberID)
	_, err = svc.RecordToolCall(bg, human, agentic.ToolCallInput{Tool: "ssh_exec"})
	assert.ErrorIs(t, err, apperr.ErrPermissionDenied, "principal without a run")
	assert.ErrorIs(t, svc.FinishToolCall(bg, human, id, "ok"), apperr.ErrPermissionDenied, "principal without a run")
	other := dbtest.Seed(t, pool)
	foreign := agentPrincipal(other, agent, run)
	_, err = svc.RecordToolCall(bg, foreign, agentic.ToolCallInput{Tool: "ssh_exec"})
	assert.ErrorIs(t, err, apperr.ErrNotFound, "run of another organization")
	assert.ErrorIs(t, svc.FinishToolCall(bg, foreign, id, "ok"), apperr.ErrNotFound, "run of another organization")
	assert.Equal(t, "running", toolCallOf(t, pool, id).Status)

	for name, in := range map[string]agentic.ToolCallInput{
		"tool with capitals":  {Tool: "SSH"},
		"empty tool":          {Tool: ""},
		"tool of 65":          {Tool: "a" + strings.Repeat("b", 64)},
		"arguments of 17 KiB": {Tool: "ssh_exec", Arguments: map[string]any{"command": strings.Repeat("x", 17<<10)}},
	} {
		_, err := svc.RecordToolCall(bg, p, in)
		assert.ErrorIs(t, err, apperr.ErrInvalidArgument, name)
	}

	exec(t, pool, `INSERT INTO agent_tool_call (org_id, run_id, tool, status)
		SELECT $1, $2, 'ssh_exec', 'ok' FROM generate_series(1, $3::int - 2)`, f.OrgID, run, agentic.MaxToolCallsPerRun)
	_, err = svc.RecordToolCall(bg, p, agentic.ToolCallInput{Tool: "ssh_exec"})
	require.ErrorIs(t, err, agentic.ErrLimitExceeded, "call 1001 of a run")

	end(t, pool, otherRun, "1 minute")
	_, err = svc.RecordToolCall(bg, agentPrincipal(f, agent, otherRun), agentic.ToolCallInput{Tool: "ssh_exec"})
	require.ErrorIs(t, err, agentic.ErrRunNotActive, "ended run")
}
