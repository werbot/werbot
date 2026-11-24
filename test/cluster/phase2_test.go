//go:build cluster

package cluster

import (
	"context"
	"fmt"
	"os"
	"slices"
	"sync"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/coder/websocket"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
	"github.com/werbot/werbot/internal/mcpserver"
)

// Limits of the phase 2 scenarios (spec 12 §12 and §15).
const (
	ApprovalPropagation = 5 * time.Second  // #17: a decision on one replica releases a command waiting on the other
	relayAwaitTimeout   = 10 * time.Second // relay.AwaitTimeout: a browser stream waits this long for its gateway stream
	onlineWindow        = 45 * time.Second // a gateway instance is online while last_seen_at is this fresh (TerminalTarget)
	approvalAppears     = 15 * time.Second // ssh_exec reaches the gateway and its approval row is written
	maxActions          = 10               // #19: max_actions of the run; twice as many calls

	// unreachableInternalURL is a server2 internal URL with no listener behind it (Ruling C18).
	unreachableInternalURL = "https://server2:1"
	approvalCommand        = "systemctl restart werbot-demo" // class write
)

// cliHome logs alice in with werbot login --device --server server, approved
// through api, into a new WERBOT_HOME under the system temp directory; the
// home is removed with the test.
func cliHome(t testing.TB, api API, server string) string {
	t.Helper()
	home, err := os.MkdirTemp("", "werbot-cli-")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(home) })
	st.Login(t, api, home, server)
	return home
}

// onlineInstances are the hostnames of the pool-a instances a ticket may pick.
func onlineInstances(ctx context.Context) ([]string, error) {
	rows, err := st.DB.Query(ctx, `SELECT hostname FROM gateway_instance
		WHERE gateway_id = $1 AND last_seen_at > now() - interval '45 seconds' ORDER BY hostname`, st.PoolID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowTo[string])
}

// TestScenario15RelayUnavailableWhenInternalListenerDown: server2 advertises an
// internal URL with no listener and only gw1 (tunnel on server1) is online, so
// a browser on server2 never gets its stream: it closes 1011 within
// AwaitTimeout plus a margin and server2 counts a timeout. SSH sessions and a
// browser on server1, which needs no bridge, keep working (12 §15).
func TestScenario15RelayUnavailableWhenInternalListenerDown(t *testing.T) {
	st.Healthy(t)
	ctx := context.Background()
	t.Cleanup(func() { // server2 with the default internal URL is ready before gw2 starts: else gw2's tunnel falls back to server1
		_, err := st.Compose(ctx, "up", "-d", "server2")
		assert.NoError(t, err)
		assert.NoError(t, poll(ctx, time.Minute, func(ctx context.Context) error { return st.ready(ctx, "server2") }))
		st.Healthy(t)
	})
	keep := OpenShell(t, st.Dial(t, st.Addr(t, "nginx", 2201), "alice+target"))
	keep.Mark(t, "ssh-before")

	_, err := st.Compose(ctx, "stop", "gw2")
	require.NoError(t, err)
	_, err = st.ComposeEnv(ctx, map[string]string{"SERVER_INTERNAL_URL": unreachableInternalURL}, "up", "-d", "server2")
	require.NoError(t, err)
	require.NoError(t, poll(ctx, time.Minute, func(ctx context.Context) error { return st.ready(ctx, "server2") }))
	require.NoError(t, poll(ctx, onlineWindow+15*time.Second, func(ctx context.Context) error {
		got, err := onlineInstances(ctx)
		if err == nil && !slices.Equal(got, []string{"gw1"}) {
			err = fmt.Errorf("online instances %v, want [gw1]: the ticket must go to the tunnel on server1", got)
		}
		return err
	}))

	c := st.DialWS(t, st.B, terminalPath)
	t0 := time.Now()
	readCtx, cancel := context.WithTimeout(ctx, relayAwaitTimeout+5*time.Second)
	defer cancel()
	var rerr error
	for rerr == nil {
		_, _, rerr = c.Read(readCtx)
	}
	require.Equal(t, websocket.StatusInternalError, websocket.CloseStatus(rerr), "close of the browser on server2: %v", rerr)
	t.Logf("browser closed %s after the dial", time.Since(t0))

	var gw, target string
	require.NoError(t, st.DB.QueryRow(ctx, `SELECT gi.hostname, si.internal_url FROM stream_ticket t
		JOIN gateway_instance gi ON gi.id = t.gateway_instance_id JOIN server_instance si ON si.id = t.target_server_id
		WHERE t.mode = 'web' ORDER BY t.created_at DESC LIMIT 1`).Scan(&gw, &target))
	assert.Equal(t, "gw1", gw, "gateway instance of the ticket")
	assert.Equal(t, unreachableInternalURL, target, "internal URL of the browser replica")
	assert.GreaterOrEqual(t, Metric(t, st.Addr(t, "server2", 9100), "werbot_relay_rejected_total", "reason", "timeout"), int64(1))
	keep.Mark(t, "ssh-after")

	a := st.DialWS(t, st.A, terminalPath) // browser and tunnel on server1: no bridge
	typeMark(t, a, "local-mark")
	ReadUntil(t, a, "mark:local-mark\r", readMax)
}

// execResult is one ssh_exec answer of an agent.
type execResult struct {
	out mcpserver.ExecOutput
	err error
}

// pendingApproval waits for the pending approval of run and returns it with
// the hostname of the gateway instance that waits for it; an ssh_exec that
// answers first fails the test with its answer.
func pendingApproval(t testing.TB, run uuid.UUID, done <-chan execResult) (id uuid.UUID, gw string) {
	t.Helper()
	err := poll(context.Background(), approvalAppears, func(ctx context.Context) error {
		select {
		case r := <-done:
			t.Fatalf("ssh_exec answered before any approval: status %q, message %q, err %v", r.out.Status, r.out.Message, r.err)
		default:
		}
		return st.DB.QueryRow(ctx, `SELECT a.id, coalesce(gi.hostname, '') FROM approval a
			LEFT JOIN gateway_instance gi ON gi.id = a.gateway_instance_id
			WHERE a.agent_run_id = $1 AND a.status = 'pending'`, run).Scan(&id, &gw)
	})
	require.NoError(t, err, "pending approval of run %s", run)
	return id, gw
}

// awaitSnapshot waits until gw1 and gw2 hold every policy change made so far:
// a snapshot is whole, so once a binding made now opens target2 on both, the
// earlier changes are there too.
func awaitSnapshot(t testing.TB) {
	t.Helper()
	bind := st.createBinding(t, st.Edge, "target2")
	defer st.deleteBinding(t, bind)
	for _, port := range []int{2201, 2202} {
		c := st.Dial(t, st.Addr(t, "nginx", port), "alice+target2")
		opened := func() bool {
			out, code, err := run(c, "hostname")
			return err == nil && out == "target\n" && code == 0
		}
		require.Eventually(t, opened, BindingPropagation, 50*time.Millisecond, "snapshot on nginx :%d", port)
	}
}

// TestScenario17ApprovalOnAnotherReplicaReleasesCommand: an agent's write
// command waits for approval on a gateway; approving it on the replica that
// does not hold that gateway's tunnel releases the command within
// ApprovalPropagation (12 §12 #17).
func TestScenario17ApprovalOnAnotherReplicaReleasesCommand(t *testing.T) {
	st.Healthy(t)
	ctx := context.Background()
	home := cliHome(t, st.Edge, st.Edge.BaseURL)
	rule, err := st.Edge.Policy.CreateCommandRule(ctx, connect.NewRequest(&werbotv1.CreateCommandRuleRequest{
		Name: "cluster-agents-write", Subjects: &werbotv1.Subjects{AllAgents: true}, Classes: []string{"write"}, Effect: "approval",
		ResourceSelector: &werbotv1.ResourceSelector{ResourceIds: []string{st.Resources["target"].String()}},
	}))
	require.NoError(t, err)
	t.Cleanup(func() {
		_, err := st.Edge.Policy.DeleteCommandRule(context.Background(), connect.NewRequest(&werbotv1.DeleteCommandRuleRequest{Id: rule.Msg.GetRule().GetId()}))
		assert.NoError(t, err)
	})
	awaitSnapshot(t)
	a := st.StartAgent(t, home, "claude", "cluster")

	done := make(chan execResult, 1)
	go func() {
		out, err := a.Exec(ctx, mcpserver.ExecInput{Resource: "target", Command: approvalCommand})
		done <- execResult{out, err}
	}()
	id, gw := pendingApproval(t, a.RunID, done)
	other, ok := map[string]API{"gw1": st.B, "gw2": st.A}[gw] // canonical placement: gw1 -> server1, gw2 -> server2
	require.True(t, ok, "approval waits on gateway instance %q", gw)

	t0 := time.Now()
	_, err = other.AccessRequests.Approve(ctx, connect.NewRequest(&werbotv1.ApproveRequest{Id: id.String()}))
	require.NoError(t, err)
	select {
	case r := <-done:
		require.NoError(t, r.err)
		require.Equal(t, "ok", r.out.Status, "ssh_exec answer: %s", r.out.Message)
		require.NotNil(t, r.out.ExitCode)
	case <-time.After(ApprovalPropagation):
		t.Fatalf("command still waits %s after the approval on the replica without the tunnel of %s", ApprovalPropagation, gw)
	}
	t.Logf("approval reached gateway in %s", time.Since(t0))

	var host string
	require.NoError(t, poll(ctx, approvalAppears, func(ctx context.Context) error {
		return st.DB.QueryRow(ctx, `SELECT gi.hostname FROM session s JOIN gateway_instance gi ON gi.id = s.gateway_instance_id
			WHERE s.agent_run_id = $1 AND s.command = $2 AND s.exit_code IS NOT NULL`, a.RunID, approvalCommand).Scan(&host)
	}), "session of the approved command")
	assert.Equal(t, gw, host, "the command ran on the gateway that waited for the approval")
}

// TestScenario19AgentActionLimitAcrossReplicas: 20 concurrent ssh_exec calls of
// one run with max_actions = 10 spread over both gateways (pool front :2222)
// and both replicas; Postgres counts them, so exactly 10 run and the others
// get Agent limit exceeded (12 §12 #19, Ruling G17).
func TestScenario19AgentActionLimitAcrossReplicas(t *testing.T) {
	st.Healthy(t)
	ctx := context.Background()
	home := cliHome(t, st.Edge, st.Edge.BaseURL)
	_, err := st.Edge.Agents.CreateAgent(ctx, connect.NewRequest(&werbotv1.CreateAgentRequest{
		Name: "limits", Limits: &werbotv1.AgentLimits{MaxActions: maxActions, MaxConcurrent: 32},
	}))
	require.NoError(t, err)
	a := st.StartAgent(t, home, "limits", "cluster")

	res := make([]execResult, 2*maxActions)
	var wg sync.WaitGroup
	for i := range res {
		wg.Go(func() {
			out, err := a.Exec(ctx, mcpserver.ExecInput{Resource: "target", Command: "id"})
			res[i] = execResult{out, err}
		})
	}
	wg.Wait()
	var ok, limited int
	for _, r := range res {
		switch {
		case r.err != nil:
			t.Errorf("tool error: %v", r.err)
		case r.out.Status == "ok":
			ok++
		case r.out.Status == "error" && r.out.Message == "Agent limit exceeded":
			limited++
		default:
			t.Errorf("unexpected answer: status %q, message %q", r.out.Status, r.out.Message)
		}
	}
	assert.Equal(t, maxActions, ok, "calls that ran")
	assert.Equal(t, maxActions, limited, "calls refused with Agent limit exceeded")

	var actions, sessions, instances int
	require.NoError(t, st.DB.QueryRow(ctx, `SELECT r.actions, count(s.id), count(DISTINCT s.gateway_instance_id)
		FROM agent_run r LEFT JOIN session s ON s.agent_run_id = r.id WHERE r.id = $1 GROUP BY r.actions`, a.RunID).
		Scan(&actions, &sessions, &instances))
	assert.Equal(t, maxActions, actions, "agent_run.actions")
	assert.Equal(t, maxActions, sessions, "sessions of the run")
	if instances != 2 {
		t.Fatalf("sessions of the run landed on %d gateway instance(s), want 2: the limit was not tested across replicas", instances)
	}
}
