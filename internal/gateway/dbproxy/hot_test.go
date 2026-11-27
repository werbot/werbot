package dbproxy_test

import (
	"bufio"
	"context"
	"os"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/werbot/werbot/internal/gateway"
	"github.com/werbot/werbot/internal/gateway/dbproxy"
	"github.com/werbot/werbot/internal/platform/sqlclass"
	"github.com/werbot/werbot/internal/platform/sqlclass/pg"
)

// Hot-path budgets of the Guard (spec 13 §4.1): a repeated text or prepared
// execute, and a new text.
const (
	budgetCachedP50 = 100 * time.Microsecond
	budgetCachedP99 = 500 * time.Microsecond
	budgetNewP99    = 2 * time.Millisecond

	hotRepeats  = 10000
	hotNew      = 2000
	hotAttempts = 3 // ponytail: noise of shared CI runners; one attempt of three must pass
)

// realGuard is a Guard on the PostgreSQL parser with a PDP that allows,
// for an agent when agent is set (always classified).
func realGuard(tb testing.TB, agent bool) *dbproxy.Guard {
	tb.Helper()
	if err := pg.Warmup(); err != nil {
		tb.Fatal(err)
	}
	h := gateway.SessionHooks{PDP: &fakePDP{version: "v1", profile: rules}}
	if agent {
		h.Limits = &gateway.AgentLimits{MaxQueries: 1 << 30}
	}
	g := dbproxy.NewGuard(&gateway.Session{Recording: &spySink{}}, h,
		dbproxy.GuardConfig{Type: "postgres", Dialect: dbproxy.SQLDialect(sqlclass.Postgres)})
	tb.Cleanup(g.Close)
	return g
}

// corpusTexts are n distinct texts: corpus lines of sqlclass/pg, each with its own comment.
func corpusTexts(tb testing.TB, n int) []string {
	tb.Helper()
	f, err := os.Open("../../platform/sqlclass/pg/testdata/corpus.txt")
	if err != nil {
		tb.Fatal(err)
	}
	defer f.Close()
	var rows []string
	for sc := bufio.NewScanner(f); sc.Scan(); {
		line := sc.Text()
		if cols := strings.SplitN(line, " | ", 4); len(cols) == 4 && !strings.HasPrefix(line, "#") {
			rows = append(rows, strings.ReplaceAll(cols[3], `\n`, "\n"))
		}
	}
	out := make([]string, n)
	for i := range out {
		out[i] = rows[i%len(rows)] + "\n-- " + strconv.Itoa(i)
	}
	return out
}

// timeChecks checks each op once and returns the sorted durations.
func timeChecks(g *dbproxy.Guard, ops []dbproxy.Op) []time.Duration {
	d := make([]time.Duration, len(ops))
	for i, op := range ops {
		start := time.Now()
		_, _ = g.Check(context.Background(), op)
		d[i] = time.Since(start)
	}
	slices.Sort(d)
	return d
}

func pct(d []time.Duration, p int) time.Duration { return d[(len(d)-1)*p/100] }

// BenchmarkGuardCheckCached repeats a text and a prepared execute: class
// cache and decision cache hits. It reports only; TestHotPathGuard holds the budget.
func BenchmarkGuardCheckCached(b *testing.B) {
	g := realGuard(b, false)
	ctx := b.Context()
	ops := []dbproxy.Op{query("select id, name from users where id = 42"), {Kind: "execute", Text: "select $1", Statement: "s1"}}
	_, _ = g.Check(ctx, dbproxy.Op{Kind: "parse", Text: "select $1", Statement: "s1"})
	for _, op := range ops {
		_, _ = g.Check(ctx, op)
	}
	b.ReportAllocs()
	for i := 0; b.Loop(); i++ {
		_, _ = g.Check(ctx, ops[i%2])
	}
}

// BenchmarkGuardCheckNew checks distinct corpus texts of an agent: every one is parsed.
func BenchmarkGuardCheckNew(b *testing.B) {
	g := realGuard(b, true)
	texts := corpusTexts(b, b.N+1)
	ctx := b.Context()
	b.ReportAllocs()
	for i := 0; b.Loop(); i++ {
		_, _ = g.Check(ctx, query(texts[i%len(texts)]+" "+strconv.Itoa(i)))
	}
}

// TestHotPathGuard holds the Guard budgets: hotRepeats repeated checks at
// p50 ≤ budgetCachedP50 and p99 ≤ budgetCachedP99, hotNew new texts at p99 ≤
// budgetNewP99, each in one of hotAttempts attempts. It runs only with
// HOT_BUDGET=1 (make bench-hot) and alone.
func TestHotPathGuard(t *testing.T) {
	if os.Getenv("HOT_BUDGET") != "1" {
		t.Skip("set HOT_BUDGET=1 to measure the hot-path budgets (make bench-hot)")
	}
	t.Run("cached", func(t *testing.T) {
		g := realGuard(t, false)
		ops := make([]dbproxy.Op, hotRepeats)
		for i := range ops {
			ops[i] = query("select id, name from users where id = 42")
		}
		bestOf(t, func() (bool, string) {
			d := timeChecks(g, ops)
			return pct(d, 50) <= budgetCachedP50 && pct(d, 99) <= budgetCachedP99,
				"cached p50 " + pct(d, 50).String() + " p99 " + pct(d, 99).String()
		})
	})
	t.Run("new", func(t *testing.T) {
		g := realGuard(t, true)
		texts := corpusTexts(t, hotNew*hotAttempts)
		attempt := 0
		bestOf(t, func() (bool, string) {
			ops := make([]dbproxy.Op, hotNew)
			for i := range ops {
				ops[i] = query(texts[attempt*hotNew+i])
			}
			attempt++
			d := timeChecks(g, ops)
			return pct(d, 99) <= budgetNewP99, "new text p50 " + pct(d, 50).String() + " p99 " + pct(d, 99).String()
		})
	})
}

// bestOf passes when one of hotAttempts attempts passes.
func bestOf(t *testing.T, attempt func() (bool, string)) {
	t.Helper()
	var results []string
	for range hotAttempts {
		ok, result := attempt()
		t.Log(result)
		if ok {
			return
		}
		results = append(results, result)
	}
	t.Fatalf("no attempt met the budget: %v", results)
}
