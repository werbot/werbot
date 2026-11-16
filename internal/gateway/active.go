package gateway

import (
	"context"
	"errors"
	"net"
	"sync"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"

	"github.com/werbot/werbot/internal/platform/metrics"
)

// closedCh is returned by idle when no slot is taken.
var closedCh = func() chan struct{} { c := make(chan struct{}); close(c); return c }()

// activeSessions holds the MaxSessions slots, the cancel of every opened session
// and the sessions whose EndSession is not delivered yet; heartbeats report both.
type activeSessions struct {
	mu       sync.Mutex
	gauge    *metrics.Gauge
	slots    int
	idleCh   chan struct{}                         // closed when slots drops to 0; nil while 0
	cancels  map[uuid.UUID]context.CancelCauseFunc // opened sessions; nil until Run binds
	pending  map[uuid.UUID]struct{}                // EndSession not delivered yet
	shutdown error                                 // set by cancelAll: a later bind cancels at once
}

func newActiveSessions(gauge *metrics.Gauge) *activeSessions {
	return &activeSessions{gauge: gauge, cancels: map[uuid.UUID]context.CancelCauseFunc{}, pending: map[uuid.UUID]struct{}{}}
}

// reserve takes a slot unless limit are taken.
func (a *activeSessions) reserve(limit int) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.slots >= limit {
		return false
	}
	if a.slots == 0 {
		a.idleCh = make(chan struct{})
	}
	a.slots++
	a.gauge.Set(int64(a.slots))
	return true
}

func (a *activeSessions) release() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.slots--
	a.gauge.Set(int64(a.slots))
	if a.slots == 0 {
		close(a.idleCh)
		a.idleCh = nil
	}
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
// server closes any other session of this instance as lost.
func (a *activeSessions) ids() []string {
	a.mu.Lock()
	defer a.mu.Unlock()
	out := make([]string, 0, len(a.cancels)+len(a.pending))
	for id := range a.cancels {
		out = append(out, id.String())
	}
	for id := range a.pending {
		out = append(out, id.String())
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

// endSession sends EndSession once; on an outage it keeps the id in the
// heartbeat and retries in the background until delivery, a refusal or stop.
func (p *Pipeline) endSession(id uuid.UUID, reason string) {
	err := p.ctl.EndSession(p.stop, id, reason)
	retry := unreachable(err)
	p.active.done(id, retry)
	if err != nil && !retry {
		p.log.Warn("session: end not accepted", "session_id", id, "reason", reason, "error", err)
	}
	if retry {
		p.retries.Go(func() { p.retryEnd(id, reason) })
	}
}

func (p *Pipeline) retryEnd(id uuid.UUID, reason string) {
	defer p.active.delivered(id)
	for n := 0; ; n++ {
		t := time.NewTimer(backoff(n, p.backoffMin, p.backoffMax))
		select {
		case <-p.stop.Done():
			t.Stop()
			p.log.Warn("session: end not delivered before stop", "session_id", id, "reason", reason)
			return
		case <-t.C:
		}
		err := p.ctl.EndSession(p.stop, id, reason)
		if !unreachable(err) {
			if err != nil && p.stop.Err() == nil {
				p.log.Warn("session: end not accepted", "session_id", id, "reason", reason, "error", err)
			}
			return
		}
	}
}

// drain refuses new sessions, waits up to timeout for the running ones, cancels the
// rest with gateway_shutdown and waits for their Run; then it gives undelivered
// EndSession calls up to flushTimeout and stops the retries.
func (p *Pipeline) drain(timeout time.Duration) {
	p.draining.Store(true)
	t := time.NewTimer(timeout)
	select {
	case <-p.active.idle():
	case <-t.C:
		p.active.cancelAll(causeShutdown)
		<-p.active.idle()
	}
	t.Stop()
	flushed := make(chan struct{})
	go func() { p.retries.Wait(); close(flushed) }()
	select {
	case <-flushed:
	case <-time.After(p.flushTimeout):
	}
	p.stopNow()
	<-flushed
}
