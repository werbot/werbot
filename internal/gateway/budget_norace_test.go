//go:build !race

package gateway

import (
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestExecDecisionP99 holds the exec decision (classify and decide) of the
// BenchmarkExecDecision snapshot to the design §12 budget: p99 ≤ 5 ms after
// 100 warm-up runs, over 2 000 runs. go test runs packages in parallel, so a
// busy CI runner can stretch one round's tail: the best of three rounds must
// meet the budget (a real slowdown fails every round). The race detector slows
// it many times, so the test runs only without it.
func TestExecDecisionP99(t *testing.T) {
	p, r := execBench(t)
	for range 100 {
		decideExec(p, r)
	}
	best := time.Duration(1<<63 - 1)
	for range 3 {
		runs := make([]time.Duration, 2000)
		for i := range runs {
			start := time.Now()
			decideExec(p, r)
			runs[i] = time.Since(start)
		}
		slices.Sort(runs)
		p99 := runs[len(runs)*99/100]
		t.Logf("exec decision p50 %v, p99 %v", runs[len(runs)/2], p99)
		best = min(best, p99)
	}
	require.LessOrEqual(t, best, 5*time.Millisecond)
}
