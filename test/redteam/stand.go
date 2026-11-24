//go:build redteam

// Package redteam is the red-team suite of stage 16 (12 §12): agent
// scenarios against the cluster stand of test/cluster under the compose
// project werbot-redteam (`make redteam`). Every test starts with begin,
// runs alone and restores in t.Cleanup what it changes; refusals are
// asserted by the agent's answer, SQL counts and audit rows, never by logs.
package redteam

import (
	"context"
	"sync"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
	"github.com/werbot/werbot/internal/mcpserver"
	"github.com/werbot/werbot/test/cluster"
)

const (
	target         = "target" // SSH resource of the stand bound to alice; the agents run as alice's agents
	aliceEmail     = "alice@example.com"
	bobEmail       = "bob@example.com"
	denialsToPause = 5                // refusals in a row that pause a run (contract §3.4)
	alertWait      = 10 * time.Second // agent_alert jobs commit with the refusal; margin for the gateway's report
	pollEvery      = 200 * time.Millisecond
)

var (
	st    *cluster.Stand // the stand of this process; tests never run in parallel
	home  string         // alice's WERBOT_HOME: werbot login --device runs in the first begin
	setup sync.Once
	ready bool // setup finished
)

// begin restores the stand (Healthy) and, on the first call, adds bob as an
// admin (approver and agent_alert recipient) and logs alice's CLI in to
// Edge with home.
func begin(t *testing.T) {
	t.Helper()
	st.Healthy(t)
	setup.Do(func() {
		st.AddMember(t, bobEmail, werbotv1.Role_ROLE_ADMIN)
		st.Login(t, st.Edge, home, st.Edge.BaseURL)
		ready = true
	})
	if !ready {
		t.Fatal("red-team setup failed in an earlier test")
	}
}

// execOn is the ssh_exec input of command on target.
func execOn(command string) mcpserver.ExecInput {
	return mcpserver.ExecInput{Resource: target, Command: command}
}

// requireAgentHealthy requires "id" on target to run for a: a refusal checked
// after it comes from policy, not from a broken stand or run (Review Focus 1).
func requireAgentHealthy(t *testing.T, a *cluster.Agent) {
	t.Helper()
	out, err := a.Exec(t.Context(), execOn("id"))
	require.NoError(t, err, "health check")
	require.Equal(t, "ok", out.Status, "health check: %s", out.Message)
}

// requireRefused requires the agent baseline refusal for command on target (ruling C6).
func requireRefused(t *testing.T, a *cluster.Agent, command string, msg ...any) {
	t.Helper()
	out, err := a.Exec(t.Context(), execOn(command))
	require.NoError(t, err, msg...)
	require.Equal(t, "denied", out.Status, msg...)
	require.Equal(t, "Access denied", out.Message, msg...)
}

// denial is one session.deny audit row.
type denial struct {
	ActorKind, Command, Reason string
	Classes                    []string
}

// denials returns the session.deny rows of run, oldest first.
func denials(t *testing.T, run uuid.UUID) []denial {
	t.Helper()
	rows, err := st.DB.Query(context.Background(), `SELECT actor_kind, data->>'target', data->>'reason',
		ARRAY(SELECT jsonb_array_elements_text(data->'classes')) FROM audit_event
		WHERE action = 'session.deny' AND data->>'agent_run_id' = $1 ORDER BY id`, run.String())
	require.NoError(t, err)
	defer rows.Close()
	var out []denial
	for rows.Next() {
		var d denial
		require.NoError(t, rows.Scan(&d.ActorKind, &d.Command, &d.Reason, &d.Classes))
		out = append(out, d)
	}
	require.NoError(t, rows.Err())
	return out
}

// sessions returns the commands of the sessions of run.
func sessions(t *testing.T, run uuid.UUID) []string {
	t.Helper()
	rows, err := st.DB.Query(context.Background(), "SELECT coalesce(command, '') FROM session WHERE agent_run_id = $1", run)
	require.NoError(t, err)
	defer rows.Close()
	var out []string
	for rows.Next() {
		var c string
		require.NoError(t, rows.Scan(&c))
		out = append(out, c)
	}
	require.NoError(t, rows.Err())
	return out
}

// transcript maps the command of every ssh_exec call of run to its status
// (GetRun as alice); a repeated command keeps its last status.
func transcript(t *testing.T, run uuid.UUID) map[string]string {
	t.Helper()
	res, err := st.Edge.Agents.GetRun(context.Background(), connect.NewRequest(&werbotv1.GetRunRequest{Id: run.String()}))
	require.NoError(t, err)
	out := map[string]string{}
	for _, c := range res.Msg.GetToolCalls() {
		if c.GetTool() == "ssh_exec" {
			out[c.GetArguments().GetFields()["command"].GetStringValue()] = c.GetStatus()
		}
	}
	return out
}

// runState is the status and the refusals in a row of run.
func runState(t *testing.T, run uuid.UUID) (status string, denialsInRow int) {
	t.Helper()
	require.NoError(t, st.DB.QueryRow(context.Background(),
		"SELECT status, denials_in_row FROM agent_run WHERE id = $1", run).Scan(&status, &denialsInRow))
	return status, denialsInRow
}

// audits counts the audit events action on the target id.
func audits(t *testing.T, action string, id uuid.UUID) int {
	t.Helper()
	var n int
	require.NoError(t, st.DB.QueryRow(context.Background(),
		"SELECT count(*) FROM audit_event WHERE action = $1 AND target_id = $2", action, id.String()).Scan(&n))
	return n
}

// mails counts the send_email jobs of template to the address (ruling C22);
// t may be the CollectT of EventuallyWithT.
func mails(t require.TestingT, template, to string) int {
	var n int
	require.NoError(t, st.DB.QueryRow(context.Background(), `SELECT count(*) FROM river_job
		WHERE kind = 'send_email' AND args->>'template' = $1 AND args->>'to' = $2`, template, to).Scan(&n))
	return n
}
