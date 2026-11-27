package dbproxy

import (
	"context"
	"sync"
	"time"

	"github.com/werbot/werbot/internal/gateway"
	"github.com/werbot/werbot/internal/platform/metrics"
	"github.com/werbot/werbot/internal/platform/sqlclass"
)

// logBuffer is how many querylog entries wait for the recording before Log blocks.
const logBuffer = 1024

// GuardConfig configures a Guard; Type labels its metrics and names its
// action ("<type>.query"). Dialect classifies the texts, Options.SecretTables
// mark secrets; Broken reports the dialect's broken parser latch (nil: none).
type GuardConfig struct {
	Type    string
	Dialect Dialect
	Options sqlclass.Options
	Broken  func() bool
	Metrics *metrics.Registry // the session's registry; nil means a private one
}

// Guard checks one session's database operations and writes their querylog
// lines to the session's recording in order, off the caller's goroutine.
// Check and Approve run on one goroutine of the connector; Log and Close are
// safe for concurrent use.
type Guard struct {
	h            gateway.SessionHooks
	action       string
	dialect      Dialect
	secretTables []string
	classes      *classCache
	brokenFn     func() bool

	overhead *metrics.Histogram
	blocked  *metrics.Counter
	hits     *metrics.Counter
	broken   *metrics.Gauge

	state // of Check and Approve

	mu     sync.RWMutex // held for reading by Log, for writing by Close
	closed bool
	ch     chan Entry
	done   chan struct{} // closed when the writer has written every entry
}

// NewGuard returns the Guard of s with its hooks h and starts its querylog
// writer; Close stops it. Zero hooks (no PDP) check nothing: every operation
// is allowed.
func NewGuard(s *gateway.Session, h gateway.SessionHooks, cfg GuardConfig) *Guard {
	reg := cfg.Metrics
	if reg == nil {
		reg = metrics.NewRegistry()
	}
	g := &Guard{
		h: h, action: cfg.Type + ".query", dialect: cfg.Dialect, secretTables: cfg.Options.SecretTables,
		classes: classCacheOf(cfg.Type), brokenFn: cfg.Broken,
		overhead: reg.Histogram("werbot_gateway_query_overhead_seconds",
			"Time the gateway spends checking one database operation.", OverheadBuckets, "type", cfg.Type),
		blocked: reg.Counter("werbot_gateway_querylog_blocked_total",
			"Querylog writes that waited for a full buffer.", "type", cfg.Type),
		hits: reg.Counter("werbot_gateway_query_cache_hits_total",
			"Operation texts whose classes came from the shared class cache.", "type", cfg.Type),
		ch:   make(chan Entry, logBuffer),
		done: make(chan struct{}),
	}
	if cfg.Broken != nil {
		g.broken = reg.Gauge("werbot_gateway_sql_parser_broken", "1 while the SQL parser is broken until restart.", "type", cfg.Type)
		g.noteBroken()
	}
	g.start(g)
	go func() {
		defer close(g.done)
		for e := range g.ch {
			g.logClasses(&e)
			_ = s.Recording.Event(e) // a lost line is the recording's failure; it ends the recording
		}
	}()
	return g
}

// Check judges op and measures the overhead. Without a PDP it allows; a
// person whose policy has no rule for the operations takes the fast path
// (allowed, classified later for the querylog only). It does not request an
// approval: a Verdict with Obligations ["approval"] goes to Approve. A
// repeated text or prepared execute does not allocate.
func (g *Guard) Check(_ context.Context, op Op) (Verdict, error) {
	t0 := time.Now()
	v := g.check(op, t0)
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
