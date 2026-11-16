package gateway

import (
	"context"
	"errors"
	"log/slog"
	"net"
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

// activeSessions holds the MaxSessions slots, the cancel of every opened session
// and the sessions whose EndSession is not delivered yet; heartbeats report both.
type activeSessions struct {
	mu       sync.Mutex
	gauge    *metrics.Gauge
	log      *slog.Logger
	draining bool
	slots    int
	idleCh   chan struct{}                         // closed when slots drops to 0; nil while 0
	cancels  map[uuid.UUID]context.CancelCauseFunc // opened sessions; nil until Run binds
	pending  map[uuid.UUID]struct{}                // EndSession not delivered yet
	shutdown error                                 // set by cancelAll: a later bind cancels at once
	// retriesClosed is set by drain before it waits on the retries: no retry starts after
	retriesClosed bool
	lastOmitted   int // pending ids left out of the last heartbeat; the cap warning fires when it changes
}

func newActiveSessions(gauge *metrics.Gauge, logger *slog.Logger) *activeSessions {
	return &activeSessions{gauge: gauge, log: logger, cancels: map[uuid.UUID]context.CancelCauseFunc{}, pending: map[uuid.UUID]struct{}{}}
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

// done drops a finished session; pending keeps its id until EndSession is delivered.
func (a *activeSessions) done(id uuid.UUID, pending bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	delete(a.cancels, id)
	if pending {
		a.pending[id] = struct{}{}
	}
}

func (a *activeSessions) delivered(id uuid.UUID) {
	a.mu.Lock()
	defer a.mu.Unlock()
	delete(a.pending, id)
}

// ids lists the opened sessions and those with an undelivered EndSession; the
// server closes any other session of this instance as lost. Opened sessions go
// first; pending ones beyond maxSessionIDs are left out of this heartbeat.
func (a *activeSessions) ids() []string {
	a.mu.Lock()
	defer a.mu.Unlock()
	out := make([]string, 0, min(len(a.cancels)+len(a.pending), maxSessionIDs))
	for id := range a.cancels {
		out = append(out, id.String())
	}
	for id := range a.pending {
		if len(out) == maxSessionIDs {
			break
		}
		out = append(out, id.String())
	}
	if omitted := len(a.cancels) + len(a.pending) - len(out); omitted != a.lastOmitted {
		a.lastOmitted = omitted
		if omitted > 0 {
			a.log.Warn("session: heartbeat full, undelivered ends left out", "omitted", omitted)
		}
	}
	return out
}

// startRetry adds a retry to wg unless drain closed the gate; the check and the Add
// share a.mu with closeRetries, so wg.Wait never races a late Add.
func (a *activeSessions) startRetry(wg *sync.WaitGroup) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.retriesClosed {
		return false
	}
	wg.Add(1)
	return true
}

// closeRetries makes every later startRetry fail.
func (a *activeSessions) closeRetries() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.retriesClosed = true
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

// endSession sends EndSession once; on an outage it keeps the id in the
// heartbeat and retries in the background until delivery, a refusal or stop.
// The id leaves the heartbeat only once the server accepted or refused the end;
// after stop, or once drain stopped taking retries, it stays pending.
func (p *Pipeline) endSession(id uuid.UUID, reason string) {
	err := p.ctl.EndSession(p.stop, id, reason)
	stopped := err != nil && p.stop.Err() != nil
	retry := unreachable(err) || stopped
	p.active.done(id, retry)
	switch {
	case retry && !stopped && p.active.startRetry(&p.retries):
		go func() {
			defer p.retries.Done()
			p.retryEnd(id, reason)
		}()
	case retry:
		p.log.Warn("session: end not delivered before stop", "session_id", id, "reason", reason)
	case err != nil:
		p.log.Warn("session: end not accepted", "session_id", id, "reason", reason, "error", err)
	}
}

func (p *Pipeline) retryEnd(id uuid.UUID, reason string) {
	for n := 0; ; n++ {
		t := time.NewTimer(backoff(n, p.backoffMin, p.backoffMax))
		select {
		case <-p.stop.Done():
			t.Stop()
		case <-t.C:
		}
		var err error
		if p.stop.Err() == nil {
			if err = p.ctl.EndSession(p.stop, id, reason); err == nil {
				p.active.delivered(id) // accepted, even if stop came during the call
				return
			}
		}
		if p.stop.Err() != nil {
			p.log.Warn("session: end not delivered before stop", "session_id", id, "reason", reason)
			return
		}
		if unreachable(err) {
			continue
		}
		p.active.delivered(id)
		if err != nil {
			p.log.Warn("session: end not accepted", "session_id", id, "reason", reason, "error", err)
		}
		return
	}
}

// drain refuses new sessions, waits up to timeout for the running ones, cancels the
// rest with gateway_shutdown and waits up to flushTimeout for their Run; then it
// gives undelivered EndSession calls up to flushTimeout and stops the retries.
func (p *Pipeline) drain(timeout time.Duration) {
	p.active.startDrain()
	if !waitFor(p.active.idle(), timeout) {
		p.active.cancelAll(causeShutdown)
		if !waitFor(p.active.idle(), p.flushTimeout) {
			p.log.Error("drain: sessions did not end after cancel", "session_ids", p.active.ids())
		}
	}
	p.active.closeRetries() // before Wait: a later Add would race it
	flushed := make(chan struct{})
	go func() { p.retries.Wait(); close(flushed) }()
	waitFor(flushed, p.flushTimeout)
	p.stopNow()
	<-flushed
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
