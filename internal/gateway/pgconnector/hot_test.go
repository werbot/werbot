package pgconnector

import (
	"fmt"
	"os"
	"runtime"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
)

// Hot-path budgets (spec 13 §4.1): what the connector may add to a direct
// session with the database.
const (
	budgetP50     = 100 * time.Microsecond // simple and prepared, added p50
	budgetP99     = 500 * time.Microsecond // simple and prepared, added p99
	budgetStream  = 0.9                    // stream: proxy bytes/s ≥ 0.9 × direct
	budgetConnect = 30 * time.Millisecond  // connect, added p99

	hotWarmup   = 200
	hotSamples  = 5000
	hotConnects = 200
	// ponytail: noise of shared CI runners; one attempt of three must pass.
	// Tighten to one when a dedicated runner measures the budgets.
	hotAttempts = 3

	// A stream sample is 64 MiB of 1 KiB rows; an attempt compares the median
	// of 5 interleaved samples each way.
	hotStreamRows    = 64 << 10
	hotStreamSamples = 5
)

// TestHotPathBudget measures, against the test database, what the connector
// adds to a direct session: each scenario interleaves direct and proxied
// operations, and passes when one of hotAttempts attempts meets its budget.
// It runs only with HOT_BUDGET=1 (make bench-hot) and alone: other tests
// would share its CPU.
func TestHotPathBudget(t *testing.T) {
	if os.Getenv("HOT_BUDGET") != "1" {
		t.Skip("set HOT_BUDGET=1 to measure the hot-path budgets (make bench-hot)")
	}
	cfg, secret := realPG(t)
	p := serveProxy(t.Context(), t, testResource(t, cfg), secret)
	conns := pair(t, cfg, secret, p)

	t.Run("simple", func(t *testing.T) { latencyBudget(t, conns, simpleOp(t.Context())) })
	t.Run("prepared", func(t *testing.T) { latencyBudget(t, conns, preparedOp(t.Context())) })
	t.Run("stream", func(t *testing.T) {
		if runtime.GOOS != "linux" {
			t.Skip("the stream budget is measured on Linux only: elsewhere a loopback relay is bound by syscalls, a plain io.Copy relay included")
		}
		bestOf(t, func() (bool, string) { return streamAttempt(t, pair(t, cfg, secret, p)) }) // new backends each attempt
	})
	t.Run("connect", func(t *testing.T) {
		direct, proxied := directURL(cfg, secret), p.connectURL(cfg.GetDatabase())
		bestOf(t, func() (bool, string) {
			var d, v []time.Duration
			for range hotConnects {
				d = append(d, connectTime(t, direct))
				v = append(v, connectTime(t, proxied))
			}
			added := pct(v, 99) - pct(d, 99)
			return added <= budgetConnect, fmt.Sprintf("added p99 %v (budget %v; direct p99 %v)", added, budgetConnect, pct(d, 99))
		})
	})
}

// TestHotPathPostgresHumanQuery is TestHotPathBudget's simple and prepared
// budgets for a person whose policy has query rules (every operation
// checked). It runs only with HOT_BUDGET=1 and alone.
func TestHotPathPostgresHumanQuery(t *testing.T) {
	if os.Getenv("HOT_BUDGET") != "1" {
		t.Skip("set HOT_BUDGET=1 to measure the hot-path budgets (make bench-hot)")
	}
	warm(t)
	cfg, secret := realPG(t)
	conns := pair(t, cfg, secret, ruledProxy(t, cfg, secret))
	t.Run("simple", func(t *testing.T) { latencyBudget(t, conns, simpleOp(t.Context())) })
	t.Run("prepared", func(t *testing.T) { latencyBudget(t, conns, preparedOp(t.Context())) })
}

// bestOf runs attempt up to hotAttempts times and stops at the first that
// meets its budget; it fails the test with the result of every attempt.
func bestOf(t *testing.T, attempt func() (ok bool, result string)) {
	t.Helper()
	var results []string
	for i := range hotAttempts {
		ok, result := attempt()
		t.Logf("attempt %d: %s", i+1, result)
		if ok {
			return
		}
		results = append(results, result)
	}
	t.Errorf("over budget in %d attempts: %v", hotAttempts, results)
}

// latencyBudget checks the added p50 and p99 of op over hotSamples
// interleaved operations, after hotWarmup of each.
func latencyBudget(t *testing.T, conns [2]*pgconn.PgConn, op func(*pgconn.PgConn) error) {
	t.Helper()
	bestOf(t, func() (bool, string) {
		for range hotWarmup {
			timed(t, conns[0], op)
			timed(t, conns[1], op)
		}
		d, v := make([]time.Duration, 0, hotSamples), make([]time.Duration, 0, hotSamples)
		for range hotSamples {
			d = append(d, timed(t, conns[0], op))
			v = append(v, timed(t, conns[1], op))
		}
		p50, p99 := pct(v, 50)-pct(d, 50), pct(v, 99)-pct(d, 99)
		return p50 <= budgetP50 && p99 <= budgetP99, fmt.Sprintf("added p50 %v p99 %v (budget %v, %v; direct p50 %v p99 %v)",
			p50, p99, budgetP50, budgetP99, pct(d, 50), pct(d, 99))
	})
}

// streamAttempt reads hotStreamSamples payloads each way, interleaved, after
// one warm-up each, and compares the median durations. Each attempt takes new
// connections: where the scheduler puts a database backend sticks with it.
func streamAttempt(t *testing.T, conns [2]*pgconn.PgConn) (bool, string) {
	op := streamOp(t.Context(), hotStreamRows, streamRowBytes)
	timed(t, conns[0], op)
	timed(t, conns[1], op)
	var d, v []time.Duration
	for range hotStreamSamples {
		d = append(d, timed(t, conns[0], op))
		v = append(v, timed(t, conns[1], op))
	}
	md, mv := pct(d, 50), pct(v, 50)
	ratio := md.Seconds() / mv.Seconds()
	mb := float64(hotStreamRows*streamRowBytes) / 1e6
	return ratio >= budgetStream, fmt.Sprintf("proxy/direct %.3f (budget %.2f; direct %.0f MB/s, proxy %.0f MB/s)",
		ratio, budgetStream, mb/md.Seconds(), mb/mv.Seconds())
}
