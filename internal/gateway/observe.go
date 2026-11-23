package gateway

import (
	"bytes"
	"context"
	"errors"
	"sync"

	"github.com/google/uuid"

	"github.com/werbot/werbot/internal/platform/metrics"
)

// observerBuffer is the events an observer may fall behind before it loses
// some; maxObservers bounds the observers of one session.
const (
	observerBuffer = 64
	maxObservers   = 16
)

// errObserverLimit refuses an observer beyond maxObservers.
var errObserverLimit = errors.New("observer limit")

// closedEvents is what Observe returns when there is nothing to watch.
var closedEvents = func() chan ObserveEvent { c := make(chan ObserveEvent); close(c); return c }()

// ObserveEvent is output of a watched terminal session, or its window when
// Window.Cols > 0.
type ObserveEvent struct {
	Data   []byte
	Window Window
}

// Observe watches the terminal session sessionID until it ends or cancel is
// called: the first event is the current window, then its output and window
// changes. A session that is unknown, ended or not a terminal gives a closed
// channel. An observer that falls behind loses events; the session never waits
// for it. The only error is errObserverLimit.
func (a *activeSessions) Observe(sessionID uuid.UUID) (<-chan ObserveEvent, func(), error) {
	a.mu.Lock()
	t := a.sessions[sessionID]
	a.mu.Unlock()
	if t == nil {
		return closedEvents, func() {}, nil
	}
	o, ok := t.rec.(*observedSink)
	if !ok {
		return closedEvents, func() {}, nil
	}
	return o.observe()
}

// recording is the recording of session id opened for r: the configured one
// when the kind is recorded, else NopRecording. A terminal's is observed
// either way.
func (p *Pipeline) recording(id uuid.UUID, r OpenRequest, recorded bool) RecordingSink {
	rec := NopRecording(id, 0, 0)
	if recorded {
		rec = p.cfg.Recording(id, r.Window.Cols, r.Window.Rows)
	}
	if r.Kind != KindTerminal {
		return rec
	}
	return &observedSink{RecordingSink: rec, drops: p.dropped, win: r.Window, obs: map[chan ObserveEvent]struct{}{}}
}

// observedSink records like its sink and copies output and window changes to
// the observers without blocking; input and markers go to the sink only.
type observedSink struct {
	RecordingSink
	drops *metrics.Counter

	mu  sync.Mutex
	win Window                         // the current window
	obs map[chan ObserveEvent]struct{} // nil once closed
}

func (o *observedSink) observe() (<-chan ObserveEvent, func(), error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.obs == nil {
		return closedEvents, func() {}, nil
	}
	if len(o.obs) >= maxObservers {
		return nil, nil, errObserverLimit
	}
	ch := make(chan ObserveEvent, observerBuffer)
	ch <- ObserveEvent{Window: o.win}
	o.obs[ch] = struct{}{}
	return ch, func() { o.leave(ch) }, nil
}

// leave ends observer ch once.
func (o *observedSink) leave(ch chan ObserveEvent) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if _, ok := o.obs[ch]; ok {
		delete(o.obs, ch)
		close(ch)
	}
}

// Output records p, then hands one copy to every observer with room for it.
func (o *observedSink) Output(p []byte) {
	o.RecordingSink.Output(p)
	if len(p) == 0 {
		return
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	var c []byte // copied once and only for an observer with room: a full one costs nothing
	for ch := range o.obs {
		if len(ch) == cap(ch) { // only Output and Resize send, under o.mu: a free slot stays free until the send
			o.drops.Inc()
			continue
		}
		if c == nil {
			c = bytes.Clone(p)
		}
		ch <- ObserveEvent{Data: c}
	}
}

// Resize records the window and tells every observer with room for it.
func (o *observedSink) Resize(cols, rows int) {
	o.RecordingSink.Resize(cols, rows)
	o.mu.Lock()
	defer o.mu.Unlock()
	o.win = Window{Cols: cols, Rows: rows}
	for ch := range o.obs {
		select {
		case ch <- ObserveEvent{Window: o.win}:
		default:
			o.drops.Inc()
		}
	}
}

// Close ends every observer, then closes the recording.
func (o *observedSink) Close(ctx context.Context) error {
	o.mu.Lock()
	for ch := range o.obs {
		close(ch)
	}
	o.obs = nil
	o.mu.Unlock()
	return o.RecordingSink.Close(ctx)
}
