package dbproxy_test

import (
	"context"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/gateway"
	"github.com/werbot/werbot/internal/gateway/dbproxy"
	"github.com/werbot/werbot/internal/platform/metrics"
)

// spySink records querylog entries; a non-nil gate blocks every Event until it is closed.
type spySink struct {
	gateway.RecordingSink // the terminal methods are never called here
	gate                  chan struct{}
	mu                    sync.Mutex
	entries               []dbproxy.Entry
	calls                 atomic.Int64
}

func (s *spySink) Event(v any) error {
	s.calls.Add(1)
	if s.gate != nil {
		<-s.gate
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.entries = append(s.entries, v.(dbproxy.Entry))
	return nil
}

func (s *spySink) texts() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]string, len(s.entries))
	for i, e := range s.entries {
		out[i] = e.Text
	}
	return out
}

func newGuard(sink *spySink, reg *metrics.Registry) *dbproxy.Guard {
	return dbproxy.NewGuard(&gateway.Session{Recording: sink}, dbproxy.GuardConfig{Type: "postgres", Metrics: reg})
}

func scrape(r *metrics.Registry) string {
	rec := httptest.NewRecorder()
	r.Handler().ServeHTTP(rec, httptest.NewRequest("GET", "/metrics", nil))
	return rec.Body.String()
}

func TestGuardCheckAllows(t *testing.T) {
	reg := metrics.NewRegistry()
	g := newGuard(&spySink{}, reg)
	defer g.Close()
	v, err := g.Check(context.Background(), dbproxy.Op{Kind: "query", Text: "select 1", DB: "app"})
	require.NoError(t, err)
	assert.True(t, v.Allow)
	assert.Empty(t, v.Classes)
	assert.Contains(t, scrape(reg), `werbot_gateway_query_overhead_seconds_count{type="postgres"} 1`)
	assert.Contains(t, scrape(reg), `werbot_gateway_query_overhead_seconds_bucket{type="postgres",le="5e-05"}`)
}

func TestGuardLogOrder(t *testing.T) {
	sink := &spySink{}
	g := newGuard(sink, metrics.NewRegistry())
	want := make([]string, 1000)
	for i := range want {
		want[i] = strconv.Itoa(i)
		g.Log(dbproxy.Entry{Kind: "query", Text: want[i]})
	}
	g.Close()
	assert.Equal(t, want, sink.texts())
}

func TestGuardLogBackpressure(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		reg := metrics.NewRegistry()
		sink := &spySink{gate: make(chan struct{})}
		g := newGuard(sink, reg)
		g.Log(dbproxy.Entry{Kind: "query", Text: "0"})
		synctest.Wait() // the writer holds entry 0 inside the blocked Event
		for i := 1; i <= 1024; i++ {
			g.Log(dbproxy.Entry{Kind: "query", Text: strconv.Itoa(i)}) // fills the buffer without blocking
		}
		assert.NotContains(t, scrape(reg), `werbot_gateway_querylog_blocked_total{type="postgres"} 1`)
		var done atomic.Bool
		go func() {
			g.Log(dbproxy.Entry{Kind: "query", Text: "1025"})
			done.Store(true)
		}()
		synctest.Wait()
		assert.False(t, done.Load(), "Log on a full buffer must block")
		assert.Contains(t, scrape(reg), `werbot_gateway_querylog_blocked_total{type="postgres"} 1`)
		close(sink.gate)
		synctest.Wait()
		assert.True(t, done.Load())
		g.Close()
		texts := sink.texts()
		require.Len(t, texts, 1026, "no entry may be lost")
		for i, s := range texts {
			assert.Equal(t, strconv.Itoa(i), s)
		}
		assert.Contains(t, scrape(reg), `werbot_gateway_querylog_blocked_total{type="postgres"} 1`)
	})
}

func TestGuardLogAfterClose(t *testing.T) {
	sink := &spySink{}
	g := newGuard(sink, metrics.NewRegistry())
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			for range 100 {
				g.Log(dbproxy.Entry{Kind: "query"})
			}
		})
	}
	wg.Go(g.Close)
	wg.Wait()
	g.Close() // repeated Close is a no-op
	n := sink.calls.Load()
	g.Log(dbproxy.Entry{Kind: "query"})
	assert.Equal(t, n, sink.calls.Load(), "Log after Close must not reach the recording")
}

func TestCheckNoAlloc(t *testing.T) {
	g := dbproxy.NewGuard(&gateway.Session{Recording: &spySink{}}, dbproxy.GuardConfig{Type: "postgres"}) // nil Metrics: a private registry
	defer g.Close()
	ctx, op := context.Background(), dbproxy.Op{Kind: "query", Text: strings.Repeat("x", 200)}
	assert.Zero(t, testing.AllocsPerRun(1000, func() { _, _ = g.Check(ctx, op) }))
}

// BenchmarkGuardCheck: ≤ 200 ns/op, 0 allocs (values table, Ruling C8).
func BenchmarkGuardCheck(b *testing.B) {
	g := newGuard(&spySink{}, metrics.NewRegistry())
	defer g.Close()
	ctx, op := context.Background(), dbproxy.Op{Kind: "query", Text: strings.Repeat("x", 200)}
	b.ReportAllocs()
	for b.Loop() {
		_, _ = g.Check(ctx, op)
	}
}

// BenchmarkGuardLog measures the caller side of Log with a fast sink; no threshold in the plan.
func BenchmarkGuardLog(b *testing.B) {
	s := &gateway.Session{Recording: gateway.NopRecording(uuid.Nil, 0, 0, "querylog")}
	g := dbproxy.NewGuard(s, dbproxy.GuardConfig{Type: "postgres"})
	e := dbproxy.Entry{Kind: "query", Text: "select 1", Rows: 1}
	b.ReportAllocs()
	for b.Loop() {
		g.Log(e)
	}
	g.Close()
}
