//go:build load

package load

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/durationpb"

	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
	"github.com/werbot/werbot/internal/mcpserver"
	"github.com/werbot/werbot/test/cluster"
)

// maxRunCalls is agentic.MaxToolCallsPerRun, the tool calls of one run (list_resources included).
const maxRunCalls = 1000

// TestL5 is the agent load of stage 16 (10 §8.3 L5): AgentExec, then Relays on the same stand.
// The classifier and PDP latency has no stand test: TestExecDecisionP99 and
// BenchmarkExecDecisionAtL2Scale of internal/gateway measure it (see README).
func TestL5(t *testing.T) {
	needStand(t)
	t.Run("AgentExec", l5AgentExec)
	t.Run("Relays", l5Relays)
}

// agentPlan spreads rate ssh_exec calls a minute over runs runs for d: the
// gap between the calls of one run and their number. A run holds at most
// maxRunCalls tool calls, its list_resources included.
func agentPlan(runs, rate int, d time.Duration) (every time.Duration, calls int, err error) {
	if runs < 1 || rate < 1 || d < time.Minute || d > time.Hour {
		return 0, 0, fmt.Errorf("agent load: runs %d, rate %d, duration %s: want runs, rate > 0 and 1m <= duration <= 1h", runs, rate, d)
	}
	every = time.Duration(runs) * time.Minute / time.Duration(rate)
	calls = int(d / every)
	if calls+1 > maxRunCalls {
		return 0, 0, fmt.Errorf("agent load: %d calls per run exceed %d (rate/runs x minutes): raise -load.agent-runs", calls+1, maxRunCalls)
	}
	return every, calls, nil
}

func TestAgentPlan(t *testing.T) {
	every, calls, err := agentPlan(20, 1000, 10*time.Minute) // the default profile
	require.NoError(t, err)
	assert.Equal(t, 1200*time.Millisecond, every)
	assert.Equal(t, 500, calls)
	_, _, err = agentPlan(10, 1000, 10*time.Minute) // 1 000 calls and list_resources
	require.Error(t, err)
	_, _, err = agentPlan(1, 1000, 10*time.Minute) // 10 000 calls in one run
	require.Error(t, err)
	_, _, err = agentPlan(1, 100, 10*time.Minute) // 1 000 calls and list_resources
	require.Error(t, err)
	_, calls, err = agentPlan(1, 99, 10*time.Minute)
	require.NoError(t, err)
	assert.Equal(t, 990, calls)
	for _, d := range []time.Duration{30 * time.Second, 2 * time.Hour} {
		_, _, err = agentPlan(10, 1000, d)
		require.Error(t, err, d)
	}
}

// execStat is the ssh_exec calls of one run: sent and failed ones, their latencies and the first error.
type execStat struct {
	sent, failed int
	lat          []time.Duration
	first        error
}

// runExec sends calls "echo load" through a, one every gap after wait, and
// returns when the last answered.
func runExec(ctx context.Context, a *cluster.Agent, wait, gap time.Duration, calls int) (s execStat) {
	start := time.Now().Add(wait)
	for k := range calls {
		select {
		case <-ctx.Done():
			return s
		case <-time.After(time.Until(start.Add(time.Duration(k) * gap))):
		}
		t0 := time.Now()
		out, err := a.Exec(ctx, mcpserver.ExecInput{Resource: "target", Command: "echo load"})
		if err == nil && (out.Status != "ok" || !strings.Contains(out.Stdout, "load")) {
			err = fmt.Errorf("status %q: %s", out.Status, out.Message)
		}
		s.sent++
		s.lat = append(s.lat, time.Since(t0))
		if err != nil {
			s.failed++
			if s.first == nil {
				s.first = err
			}
		}
	}
	return s
}

// l5AgentExec: *agentRuns werbot mcp runs (agents load-<i>) send *agentRate
// ssh_exec calls a minute in total for *agentFor through the pool. No call
// fails, every call has its session row, RSS within the §10.1 budgets (12 §12).
func l5AgentExec(t *testing.T) {
	every, calls, err := agentPlan(*agentRuns, *agentRate, *agentFor)
	if err != nil {
		t.Fatal(err)
	}
	home := t.TempDir()
	st.Login(t, st.Edge, home, st.Edge.BaseURL)
	agents := make([]*cluster.Agent, *agentRuns)
	for i := range agents {
		name := fmt.Sprintf("load-%d", i)
		_, err := st.Edge.Agents.CreateAgent(t.Context(), connect.NewRequest(&werbotv1.CreateAgentRequest{Name: name,
			Limits: &werbotv1.AgentLimits{MaxActions: maxRunCalls, MaxDuration: durationpb.New(time.Hour), MaxConcurrent: 32}}))
		require.NoError(t, err)
		agents[i] = st.StartAgent(t, home, name, "L5 load")
	}

	ctx, stop := context.WithCancel(t.Context())
	samples := sampleEvery(ctx, t, st)
	stats := make([]execStat, len(agents))
	var wg sync.WaitGroup
	from := time.Now()
	for i, a := range agents { // runs start evenly inside one gap
		wg.Go(func() {
			stats[i] = runExec(t.Context(), a, time.Duration(i)*every/time.Duration(len(agents)), every, calls)
		})
	}
	wg.Wait()
	took := time.Since(from)
	stop()
	all := <-samples
	reportExec(t, agents, stats, calls, took)
	reportRSS(t, "L5", all, "gw1", gatewayBudget)
	reportRSS(t, "L5", all, "gw2", gatewayBudget)
	reportRSS(t, "L5", all, "server1", serverBudget)
	reportRSS(t, "L5", all, "server2", serverBudget)
}

// reportExec reports the errors, the calls with their session rows and the rate and p99 of the calls.
func reportExec(t *testing.T, agents []*cluster.Agent, stats []execStat, calls int, took time.Duration) {
	t.Helper()
	var sent, failed, rows int
	var lat []time.Duration
	var first error
	for i, s := range stats {
		sent, failed, lat = sent+s.sent, failed+s.failed, append(lat, s.lat...)
		if first == nil {
			first = s.first
		}
		var n int
		require.NoError(t, st.DB.QueryRow(context.Background(), `SELECT count(*) FROM session WHERE agent_run_id = $1`, agents[i].RunID).Scan(&n))
		rows += n
	}
	want := len(agents) * calls
	report(t, "L5", "agent ssh_exec errors", "0", fmt.Sprintf("%d of %d (first: %v)", failed, sent, first), failed == 0)
	report(t, "L5", "agent ssh_exec answered", fmt.Sprint(want), fmt.Sprint(sent), sent == want)
	report(t, "L5", "agent sessions recorded", "= calls", fmt.Sprintf("%d of %d", rows, sent), rows == sent)
	report(t, "L5", "agent ssh_exec rate", "reference", fmt.Sprintf("%.0f/min over %d runs", float64(sent)/took.Minutes(), len(agents)), true)
	report(t, "L5", "agent ssh_exec p99", "reference", p99(lat).Round(time.Millisecond).String(), true)
}
