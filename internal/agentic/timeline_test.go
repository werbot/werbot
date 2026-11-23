package agentic_test

import (
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
	"github.com/werbot/werbot/internal/agentic"
	"github.com/werbot/werbot/internal/platform/db/dbtest"
)

// TestGetRunTimeline: GetRun returns the run with its tool calls, sessions,
// approvals and denials, each oldest first; a member never reads another
// sponsor's run, an admin reads any.
func TestGetRunTimeline(t *testing.T) {
	svc, pool, f := newTestService(t)
	h := agentic.NewHandler(svc)
	agent := dbtest.Agent(t, pool, f.OrgID, f.MemberID, "claude")
	run := dbtest.AgentRun(t, pool, f.OrgID, agent, 10, 4)
	p := agentPrincipal(f, agent, run)
	first, err := svc.RecordToolCall(bg, p, agentic.ToolCallInput{Tool: "ssh_exec", Arguments: map[string]any{"command": "uptime"}})
	require.NoError(t, err)
	second, err := svc.RecordToolCall(bg, p, agentic.ToolCallInput{Tool: "list_resources"})
	require.NoError(t, err)
	session := f.StartSession(t, pool, f.MemberID)
	exec(t, pool, `UPDATE session SET agent_run_id = $2, tool_call_id = $3, command = 'uptime', classes = '{read}', exit_code = 0,
		output_sha256 = sha256('out'), ended_at = now(), end_reason = 'closed' WHERE id = $1`, session, run, first)
	var approval uuid.UUID
	require.NoError(t, pool.QueryRow(bg, `INSERT INTO approval (org_id, kind, requester_user_id, agent_run_id, resource_id, command,
		classes, status, decided_by, decided_at, expires_at) VALUES ($1, 'command', $2, $3, $4, 'rm -rf /tmp/x', '{destructive}',
		'denied', $5, now(), now() + interval '5 minutes') RETURNING id`, f.OrgID, f.MemberID, run, f.ResourceID, f.AdminID).Scan(&approval))
	_, err = svc.RecordDenial(bg, f.OrgID, run, agentic.Denial{Resource: "web", Command: "cat /etc/shadow",
		Classes: []string{"secrets"}, Reasons: []string{"agent:baseline:dangerous"}})
	require.NoError(t, err)
	other := dbtest.AgentRun(t, pool, f.OrgID, agent, 10, 4)
	_, err = svc.RecordToolCall(bg, agentPrincipal(f, agent, other), agentic.ToolCallInput{Tool: "ssh_exec"})
	require.NoError(t, err)

	res, err := h.GetRun(as(f, f.MemberID), req(&werbotv1.GetRunRequest{Id: run.String()}))
	require.NoError(t, err)
	m := res.Msg
	assert.Equal(t, run.String(), m.GetRun().GetId())
	assert.Equal(t, "claude", m.GetRun().GetAgentName())
	require.Len(t, m.GetToolCalls(), 2, "only the run's own calls")
	assert.Equal(t, []string{first.String(), second.String()}, []string{m.GetToolCalls()[0].GetId(), m.GetToolCalls()[1].GetId()})
	assert.Equal(t, map[string]any{"command": "uptime"}, m.GetToolCalls()[0].GetArguments().AsMap())
	require.Len(t, m.GetSessions(), 1)
	s := m.GetSessions()[0]
	assert.Equal(t, "web", s.GetResourceName())
	assert.Equal(t, first.String(), s.GetToolCallId())
	assert.Equal(t, "uptime", s.GetCommand())
	assert.Equal(t, []string{"read"}, s.GetClasses())
	assert.Equal(t, int32(0), s.GetExitCode())
	assert.NotNil(t, s.ExitCode, "exit code 0 is set")
	assert.Len(t, s.GetOutputSha256(), 32)
	require.Len(t, m.GetApprovals(), 1)
	a := m.GetApprovals()[0]
	assert.Equal(t, approval.String(), a.GetId())
	assert.Equal(t, "denied", a.GetStatus())
	assert.Equal(t, []string{"destructive"}, a.GetClasses())
	assert.NotEmpty(t, a.GetDecidedByLogin())
	require.Len(t, m.GetDenials(), 1)
	d := m.GetDenials()[0]
	assert.Equal(t, "web", d.GetResource())
	assert.Equal(t, "cat /etc/shadow", d.GetCommand())
	assert.Equal(t, []string{"secrets"}, d.GetClasses())
	assert.Equal(t, []string{"agent:baseline:dangerous"}, d.GetReasons())
	assert.WithinDuration(t, time.Now(), d.GetAt().AsTime(), time.Minute)

	_, err = h.GetRun(as(f, f.OtherMemberID), req(&werbotv1.GetRunRequest{Id: run.String()}))
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err), "another sponsor's run")
	_, err = h.GetRun(as(f, f.AdminID), req(&werbotv1.GetRunRequest{Id: run.String()}))
	require.NoError(t, err, "an admin reads any run")
	o := dbtest.Seed(t, pool)
	_, err = h.GetRun(as(o, o.AdminID), req(&werbotv1.GetRunRequest{Id: run.String()}))
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err), "a run of another organization")
}

// TestListRunsRPC: runs newest first, filtered by agent and status, paged
// (default 50, next_page 0 at the end); a member lists only own runs.
func TestListRunsRPC(t *testing.T) {
	svc, pool, f := newTestService(t)
	h := agentic.NewHandler(svc)
	claude := dbtest.Agent(t, pool, f.OrgID, f.MemberID, "claude")
	codex := dbtest.Agent(t, pool, f.OrgID, f.MemberID, "codex")
	foreign := dbtest.Agent(t, pool, f.OrgID, f.OtherMemberID, "claude")
	var runs []uuid.UUID
	for range 3 {
		runs = append(runs, dbtest.AgentRun(t, pool, f.OrgID, claude, 10, 4))
	}
	ended := dbtest.AgentRun(t, pool, f.OrgID, codex, 10, 4)
	end(t, pool, ended, "1 minute")
	dbtest.AgentRun(t, pool, f.OrgID, foreign, 10, 4)

	ids := func(m *werbotv1.ListRunsResponse) (out []string) {
		for _, r := range m.GetRuns() {
			out = append(out, r.GetId())
		}
		return out
	}
	all, err := h.ListRuns(as(f, f.MemberID), req(&werbotv1.ListRunsRequest{}))
	require.NoError(t, err)
	assert.Equal(t, []string{ended.String(), runs[2].String(), runs[1].String(), runs[0].String()}, ids(all.Msg), "own runs, newest first")
	assert.Zero(t, all.Msg.GetNextPage())
	page, err := h.ListRuns(as(f, f.MemberID), req(&werbotv1.ListRunsRequest{AgentId: claude.String(), PageSize: 2}))
	require.NoError(t, err)
	assert.Equal(t, []string{runs[2].String(), runs[1].String()}, ids(page.Msg))
	assert.Equal(t, int32(2), page.Msg.GetNextPage())
	page, err = h.ListRuns(as(f, f.MemberID), req(&werbotv1.ListRunsRequest{AgentId: claude.String(), PageSize: 2, Page: 2}))
	require.NoError(t, err)
	assert.Equal(t, []string{runs[0].String()}, ids(page.Msg))
	assert.Zero(t, page.Msg.GetNextPage())
	byStatus, err := h.ListRuns(as(f, f.MemberID), req(&werbotv1.ListRunsRequest{Status: "ended"}))
	require.NoError(t, err)
	assert.Equal(t, []string{ended.String()}, ids(byStatus.Msg))
	admin, err := h.ListRuns(as(f, f.AdminID), req(&werbotv1.ListRunsRequest{}))
	require.NoError(t, err)
	assert.Len(t, admin.Msg.GetRuns(), 5, "an admin lists every run")
}
