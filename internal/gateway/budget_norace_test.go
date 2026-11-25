//go:build !race

package gateway

import (
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestExecDecisionP99 holds the exec decision (analyse, match paths and
// decide) of the BenchmarkExecDecision snapshot to the design §12 budget, for
// benchCommand and worstCommand of a person and of an agent (two passes):
// p99 ≤ 5 ms after 100 warm-up runs, over 2 000 runs. go test runs packages in parallel, so a
// busy CI runner can stretch one round's tail: the best of three rounds must
// meet the budget (a real slowdown fails every round). The race detector slows
// it many times, so the test runs only without it.
func TestExecDecisionP99(t *testing.T) {
	p, human, agent := execBench(t, scaleBudget)
	for _, r := range []AccessRequest{human, agent} {
		for _, command := range []string{benchCommand, worstCommand} {
			r.Command = command
			requireP99(t, p, r)
		}
	}
}

// BenchmarkExecDecisionAtL2Scale measures the same decision at the L2 scale of
// the load test (10 000 bindings, 200 command rules, 100 agents; O1). The p99
// budget holds at 1 000 bindings only: the cost grows with the bindings, and
// splitting the policies per resource is phase 3. It reports p50-ms and p99-ms
// of one round of 2 000 decisions and never fails on latency; it fails only
// when the decision is not the measured one. go test runs no benchmark by
// default, so it never slows the unit tests. Run it alone:
//
//	go test -run '^$' -bench '^BenchmarkExecDecisionAtL2Scale$' -benchtime 1x ./internal/gateway/
func BenchmarkExecDecisionAtL2Scale(b *testing.B) {
	p, human, agent := execBench(b, scaleL2)
	for who, r := range map[string]AccessRequest{"human": human, "agent": agent} {
		for name, command := range map[string]string{"bench": benchCommand, "worst": worstCommand} {
			r.Command = command
			require.Equal(b, []string{ObligationApproval}, decideExec(p, r).Obligations, "%s/%s", who, name)
			b.Run(who+"/"+name, func(b *testing.B) {
				warmUp(p, r)
				var p50, p99 time.Duration
				for b.Loop() {
					p50, p99 = latencyRound(p, r)
				}
				b.ReportMetric(0, "ns/op") // one op is a whole round
				b.ReportMetric(float64(p50)/float64(time.Millisecond), "p50-ms")
				b.ReportMetric(float64(p99)/float64(time.Millisecond), "p99-ms")
			})
		}
	}
}

func warmUp(p *Policy, r AccessRequest) {
	for range 100 {
		decideExec(p, r)
	}
}

// latencyRound times 2 000 decisions of r and returns their p50 and p99.
func latencyRound(p *Policy, r AccessRequest) (p50, p99 time.Duration) {
	runs := make([]time.Duration, 2000)
	for i := range runs {
		start := time.Now()
		decideExec(p, r)
		runs[i] = time.Since(start)
	}
	slices.Sort(runs)
	return runs[len(runs)/2], runs[len(runs)*99/100]
}

func requireP99(t *testing.T, p *Policy, r AccessRequest) {
	t.Helper()
	warmUp(p, r)
	best := time.Duration(1<<63 - 1)
	for range 3 {
		p50, p99 := latencyRound(p, r)
		t.Logf("exec decision p50 %v, p99 %v", p50, p99)
		best = min(best, p99)
	}
	require.LessOrEqual(t, best, 5*time.Millisecond)
}
