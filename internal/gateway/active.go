package gateway

import (
	"context"
	"errors"
	"log/slog"
	"maps"
	"net"
	"slices"
	"sync"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"

	"github.com/werbot/werbot/internal/platform/metrics"
)

// maxSessionIDs is the max_items of Heartbeat.active_session_ids; the server
// rejects a longer list. Config.MaxSessions is capped by it.
const maxSessionIDs = 10000

// closedCh is returned by idle when no slot is taken.
var closedCh = func() chan struct{} { c := make(chan struct{}); close(c); return c }()

// activeSessions holds the MaxSessions slots and the state of every opened session.
type activeSessions struct {
	mu          sync.Mutex
	gauge       *metrics.Gauge
	log         *slog.Logger
	draining    bool
	drained     chan struct{} // closed by startDrain: approval waits end
	slots       int
	idleCh      chan struct{}          // closed when slots drops to 0; nil while 0
	sessions    map[uuid.UUID]*tracked // opened sessions
	shutdown    error                  // set by cancelAll: a later bind cancels at once
	lastOmitted int                    // ids left out of the last heartbeat; the cap warning fires when it changes
}

// tracked is an opened session as the sweeper and the heartbeat see it.
type tracked struct {
	cancel    context.CancelCauseFunc // nil until Run binds
	cause     error                   // the first cancel before bind; bind applies it
	req       AccessRequest           // the PDP input, re-evaluated by the sweeper
	kind      Kind
	host      string // the target host of a stream or datagram
	version   string // the policy version req was last allowed on
	started   time.Time
	idle, max time.Duration // 0: no limit
	held      int           // console checks running (hold): the idle limit waits
	c         *counters
	sent      [4]int64 // the counters in the last heartbeat
}

func newActiveSessions(gauge *metrics.Gauge, logger *slog.Logger) *activeSessions {
	return &activeSessions{gauge: gauge, log: logger, sessions: map[uuid.UUID]*tracked{}, drained: make(chan struct{})}
}

// reserve takes a slot: ErrNotReady while draining, ErrBusy when limit are taken.
func (a *activeSessions) reserve(limit int) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.draining {
		return ErrNotReady
	}
	if a.slots >= limit {
		return ErrBusy
	}
	if a.slots == 0 {
		a.idleCh = make(chan struct{})
	}
	a.slots++
	a.gauge.Set(int64(a.slots))
	return nil
}

func (a *activeSessions) release() {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.slots == 0 {
		a.log.Error("session: slot released twice")
		return
	}
	a.slots--
	a.gauge.Set(int64(a.slots))
	if a.slots == 0 {
		close(a.idleCh)
		a.idleCh = nil
	}
}

// startDrain makes every later reserve fail with ErrNotReady and closes drained.
func (a *activeSessions) startDrain() {
	a.mu.Lock()
	defer a.mu.Unlock()
	if !a.draining {
		close(a.drained)
	}
	a.draining = true
}

func (a *activeSessions) isDraining() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.draining
}

func (a *activeSessions) used() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.slots
}

// idle is closed once every slot is free.
func (a *activeSessions) idle() <-chan struct{} {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.idleCh == nil {
		return closedCh
	}
	return a.idleCh
}

// add tracks an opened session; t.c is set when nil.
func (a *activeSessions) add(id uuid.UUID, t *tracked) {
	if t.c == nil {
		t.c = newCounters(t.started)
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	a.sessions[id] = t
}

// bind stores the cancel of a running session and returns its counters. A cancel
// that came before (cancel, cancelAll) is applied at once.
func (a *activeSessions) bind(id uuid.UUID, cancel context.CancelCauseFunc) *counters {
	a.mu.Lock()
	defer a.mu.Unlock()
	t := a.sessions[id] // added by Open
	t.cancel = cancel
	if t.cause != nil {
		cancel(t.cause)
	}
	if a.shutdown != nil {
		cancel(a.shutdown)
	}
	return t.c
}

// cancel ends a session with cause; before bind the cause waits for it. The
// first cause wins.
func (a *activeSessions) cancel(id uuid.UUID, cause error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.cancelLocked(id, cause)
}

func (a *activeSessions) cancelLocked(id uuid.UUID, cause error) {
	t := a.sessions[id]
	switch {
	case t == nil:
	case t.cancel != nil:
		t.cancel(cause)
	case t.cause == nil:
		t.cause = cause
	}
}

// cancelAll cancels every running session and every one bound later with cause.
func (a *activeSessions) cancelAll(cause error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.shutdown = cause
	for _, t := range a.sessions {
		if t.cancel != nil {
			t.cancel(cause)
		}
	}
}

// Terminate ends a session with reason (recorder.Options.Terminate); an unknown
// session is left alone.
func (a *activeSessions) Terminate(sessionID uuid.UUID, reason string) {
	a.cancel(sessionID, endCause(reason))
}

// countersOf returns the counters of a session; nil when it is unknown.
func (a *activeSessions) countersOf(id uuid.UUID) *counters {
	a.mu.Lock()
	defer a.mu.Unlock()
	if t := a.sessions[id]; t != nil {
		return t.c
	}
	return nil
}

// hold marks a console check of session id running until release: expire
// skips its idle limit (not its max duration) meanwhile, and the release
// counts as activity. An unknown session is left alone.
func (a *activeSessions) hold(id uuid.UUID) (release func()) {
	a.mu.Lock()
	defer a.mu.Unlock()
	t := a.sessions[id]
	if t == nil {
		return func() {}
	}
	t.held++
	return func() {
		a.mu.Lock()
		defer a.mu.Unlock()
		t.held--
		t.c.touch()
	}
}

// done drops a finished session.
func (a *activeSessions) done(id uuid.UUID) {
	a.mu.Lock()
	defer a.mu.Unlock()
	delete(a.sessions, id)
}

// ids lists the opened sessions.
func (a *activeSessions) ids() []uuid.UUID {
	a.mu.Lock()
	defer a.mu.Unlock()
	return slices.Collect(maps.Keys(a.sessions))
}

// noteOmitted warns when the number of ids left out of the heartbeat changes.
func (a *activeSessions) noteOmitted(n int) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if n != a.lastOmitted && n > 0 {
		a.log.Warn("session: heartbeat full, undelivered ends left out", "omitted", n)
	}
	a.lastOmitted = n
}

// unreachable reports a control plane outage, as opposed to a refusal.
func unreachable(err error) bool {
	if err == nil {
		return false
	}
	var ne net.Error
	code := connect.CodeOf(err)
	return code == connect.CodeUnavailable || code == connect.CodeDeadlineExceeded || errors.As(err, &ne)
}

// drain refuses new sessions, waits up to timeout for the running ones, cancels the
// rest with gateway_shutdown and waits up to flushTimeout for their Run; then it
// stops the pipeline. Their ends are in the spool by then; serve waits for delivery.
func (p *Pipeline) drain(timeout time.Duration) {
	p.active.startDrain()
	if !waitFor(p.active.idle(), timeout) {
		p.active.cancelAll(causeShutdown)
		if !waitFor(p.active.idle(), p.flushTimeout) {
			p.log.Error("drain: sessions did not end after cancel", "session_ids", p.active.ids())
		}
	}
	p.stopNow()
}

// waitFor reports whether ch closed within d.
func waitFor(ch <-chan struct{}, d time.Duration) bool {
	select { // an already closed ch wins over a zero d
	case <-ch:
		return true
	default:
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ch:
		return true
	case <-t.C:
		return false
	}
}
