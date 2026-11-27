package dbproxy

import (
	"context"
	"sync"
	"time"

	"github.com/werbot/werbot/internal/gateway"
	"github.com/werbot/werbot/internal/platform/metrics"
)

// logBuffer is how many querylog entries wait for the recording before Log blocks.
const logBuffer = 1024

// GuardConfig configures a Guard; Type labels its metrics.
type GuardConfig struct {
	Type    string
	Metrics *metrics.Registry // the session's registry; nil means a private one
}

// Guard checks one session's database operations and writes their querylog
// lines to the session's recording in order, off the caller's goroutine.
type Guard struct {
	overhead *metrics.Histogram
	blocked  *metrics.Counter
	mu       sync.RWMutex // held for reading by Log, for writing by Close
	closed   bool
	ch       chan Entry
	done     chan struct{} // closed when the writer has written every entry
}

// NewGuard returns the Guard of s and starts its querylog writer; Close stops it.
func NewGuard(s *gateway.Session, cfg GuardConfig) *Guard {
	reg := cfg.Metrics
	if reg == nil {
		reg = metrics.NewRegistry()
	}
	g := &Guard{
		overhead: reg.Histogram("werbot_gateway_query_overhead_seconds",
			"Time the gateway spends checking one database operation.", OverheadBuckets, "type", cfg.Type),
		blocked: reg.Counter("werbot_gateway_querylog_blocked_total",
			"Querylog writes that waited for a full buffer.", "type", cfg.Type),
		ch:   make(chan Entry, logBuffer),
		done: make(chan struct{}),
	}
	go func() {
		defer close(g.done)
		for e := range g.ch {
			_ = s.Recording.Event(e) // a lost line is the recording's failure; it ends the recording
		}
	}()
	return g
}

// Check judges op; stage 17 allows every operation and only measures the
// overhead. It does not allocate.
func (g *Guard) Check(_ context.Context, _ Op) (Verdict, error) {
	t0 := time.Now()
	v := Verdict{Allow: true}
	g.overhead.Observe(time.Since(t0).Seconds())
	return v, nil
}

// Log queues e for the querylog. A full buffer blocks the caller rather than
// lose the line. After Close, Log does nothing.
func (g *Guard) Log(e Entry) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	if g.closed {
		return
	}
	select {
	case g.ch <- e:
	default:
		g.blocked.Inc()
		g.ch <- e
	}
}

// Close waits until every queued entry is written; a repeated Close only waits.
func (g *Guard) Close() {
	g.mu.Lock()
	if !g.closed {
		g.closed = true
		close(g.ch)
	}
	g.mu.Unlock()
	<-g.done
}
