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

	"github.com/werbot/werbot/internal/gateway/recorder"
	"github.com/werbot/werbot/internal/platform/metrics"
)

// maxSessionIDs is the max_items of Heartbeat.active_session_ids; the server
// rejects a longer list. Config.MaxSessions is capped by it.
const maxSessionIDs = 10000

// closedCh is returned by idle when no slot is taken.
var closedCh = func() chan struct{} { c := make(chan struct{}); close(c); return c }()

// activeSessions holds the MaxSessions slots and the cancel of every opened session.
type activeSessions struct {
	mu          sync.Mutex
	gauge       *metrics.Gauge
	log         *slog.Logger
	draining    bool
	slots       int
	idleCh      chan struct{}                         // closed when slots drops to 0; nil while 0
	cancels     map[uuid.UUID]context.CancelCauseFunc // opened sessions; nil until Run binds
	shutdown    error                                 // set by cancelAll: a later bind cancels at once
	lastOmitted int                                   // ids left out of the last heartbeat; the cap warning fires when it changes
}

func newActiveSessions(gauge *metrics.Gauge, logger *slog.Logger) *activeSessions {
	return &activeSessions{gauge: gauge, log: logger, cancels: map[uuid.UUID]context.CancelCauseFunc{}}
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

// startDrain makes every later reserve fail with ErrNotReady.
func (a *activeSessions) startDrain() {
	a.mu.Lock()
	defer a.mu.Unlock()
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

func (a *activeSessions) add(id uuid.UUID) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.cancels[id] = nil
}

// bind stores the cancel of a running session; after cancelAll it cancels at once.
func (a *activeSessions) bind(id uuid.UUID, cancel context.CancelCauseFunc) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.cancels[id] = cancel
	if a.shutdown != nil {
		cancel(a.shutdown)
	}
}

// cancelAll cancels every running session and every one bound later with cause.
func (a *activeSessions) cancelAll(cause error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.shutdown = cause
	for _, cancel := range a.cancels {
		if cancel != nil {
			cancel(cause)
		}
	}
}

// Terminate ends a running session with reason (recorder.Options.Terminate); an
// unknown or not yet running session is left alone.
func (a *activeSessions) Terminate(sessionID uuid.UUID, reason string) {
	a.mu.Lock()
	cancel := a.cancels[sessionID]
	a.mu.Unlock()
	if cancel != nil {
		cancel(endCause(reason))
	}
}

// done drops a finished session.
func (a *activeSessions) done(id uuid.UUID) {
	a.mu.Lock()
	defer a.mu.Unlock()
	delete(a.cancels, id)
}

// ids lists the opened sessions.
func (a *activeSessions) ids() []uuid.UUID {
	a.mu.Lock()
	defer a.mu.Unlock()
	return slices.Collect(maps.Keys(a.cancels))
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

// heartbeatSessionIDs lists the opened sessions, then those whose end waits in the
// spool, without repeats; the server closes any other session of this instance as
// lost. Opened sessions are read first: a session moves from them to the spool, so
// it cannot slip between the two reads. Ids beyond maxSessionIDs (pending ends only,
// as MaxSessions is capped by it) wait for a later heartbeat.
func heartbeatSessionIDs(a *activeSessions, sp *recorder.Spool) []string {
	ids := a.ids()
	seen := make(map[uuid.UUID]bool, len(ids))
	for _, id := range ids {
		seen[id] = true
	}
	for _, id := range sp.PendingEnds() {
		if !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	a.noteOmitted(max(len(ids)-maxSessionIDs, 0))
	out := make([]string, min(len(ids), maxSessionIDs))
	for i := range out {
		out[i] = ids[i].String()
	}
	return out
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
