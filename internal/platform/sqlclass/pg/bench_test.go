package pg

import (
	"os"
	"slices"
	"strconv"
	"testing"
	"time"
)

const (
	budgetNewText = 2 * time.Millisecond // spec 13 §4.1: a new text, added p99
	hotVariants   = 2000
	// ponytail: noise of shared CI runners; one attempt of three must pass.
	// Tighten to one when a dedicated runner measures the budgets.
	hotAttempts = 3
)

// variants returns n distinct texts: corpus lines, each with its own trailing comment.
func variants(tb testing.TB, n int) []string {
	rows := loadCorpus(tb)
	out := make([]string, n)
	for i := range out {
		out[i] = rows[i%len(rows)].sql + "\n-- " + strconv.Itoa(i)
	}
	return out
}

// latencies classifies each text once and returns the sorted durations.
func latencies(texts []string) []time.Duration {
	d := make([]time.Duration, len(texts))
	for i, sql := range texts {
		start := time.Now()
		classify(sql)
		d[i] = time.Since(start)
	}
	slices.Sort(d)
	return d
}

func pct(d []time.Duration, p int) time.Duration { return d[(len(d)-1)*p/100] }

// BenchmarkPGClassify cycles through the corpus variants (the parser has no
// cache); it reports only, TestHotPathPGClassify holds the budget.
func BenchmarkPGClassify(b *testing.B) {
	texts := variants(b, hotVariants)
	var d []time.Duration
	for i := 0; b.Loop(); i++ {
		start := time.Now()
		classify(texts[i%len(texts)])
		d = append(d, time.Since(start))
	}
	slices.Sort(d)
	b.ReportMetric(float64(pct(d, 50).Nanoseconds()), "p50-ns")
	b.ReportMetric(float64(pct(d, 99).Nanoseconds()), "p99-ns")
}

// TestHotPathPGClassify holds the new-text budget: the p99 of classifying
// hotVariants corpus variants is at most budgetNewText in one of hotAttempts
// attempts. It runs only with HOT_BUDGET=1 (make bench-hot) and alone.
func TestHotPathPGClassify(t *testing.T) {
	if os.Getenv("HOT_BUDGET") != "1" {
		t.Skip("set HOT_BUDGET=1 to measure the hot-path budgets (make bench-hot)")
	}
	texts := variants(t, hotVariants)
	var results []string
	for range hotAttempts {
		d := latencies(texts)
		result := "new text p50 " + pct(d, 50).String() + " p99 " + pct(d, 99).String() + " (budget p99 " + budgetNewText.String() + ")"
		t.Log(result)
		if pct(d, 99) <= budgetNewText {
			return
		}
		results = append(results, result)
	}
	t.Fatalf("no attempt met the budget: %v", results)
}
