package gateway

import (
	"fmt"
	"log/slog"
	"runtime/debug"
	"sync"
)

// Guard runs the relay goroutines of one session for a connector. The pipeline
// recovers only the Serve goroutine, so a relay recovers its own panic: the
// first panic or error stops the session; Wait re-panics a panic on the calling
// (Serve) goroutine, so the pipeline ends the session as "internal_error".
type Guard struct {
	stop     func()
	secret   Secret // redacted from the log
	wg       sync.WaitGroup
	mu       sync.Mutex
	err      error
	panicked *relayPanic
	finished bool // Wait has returned: later failures have no session to end
}

// NewGuard returns a guard that calls stop on the first failure; stop must
// unblock the goroutines Wait waits for.
func NewGuard(stop func(), secret Secret) *Guard {
	return &Guard{stop: stop, secret: secret}
}

// relayPanic is a recovered relay panic with the relay goroutine's stack.
type relayPanic struct {
	value any
	stack []byte
}

func (p *relayPanic) String() string {
	return fmt.Sprintf("relay panic: %v\n\nrelay goroutine stack:\n%s", p.value, p.stack)
}

// Go runs fn on a goroutine that Wait waits for.
func (g *Guard) Go(fn func() error) {
	g.wg.Go(func() { g.call(fn) })
}

// Detach runs fn on a goroutine that Wait does not wait for: fn may block on
// a reader the session cannot interrupt, until the front closes it.
func (g *Guard) Detach(fn func() error) {
	go g.call(fn)
}

func (g *Guard) call(fn func() error) {
	defer func() {
		if r := recover(); r != nil {
			g.fail(nil, &relayPanic{value: r, stack: debug.Stack()})
		}
	}()
	if err := fn(); err != nil {
		g.fail(err, nil)
	}
}

// fail keeps the first failure and stops the session. A panic after Wait
// returned is only logged (the stack, never relayed data), the secret redacted.
func (g *Guard) fail(err error, p *relayPanic) {
	g.mu.Lock()
	switch {
	case g.finished:
		if p != nil {
			slog.Error("connector: relay panic after the session ended",
				"panic", g.secret.Redact(fmt.Sprint(p.value)), "stack", g.secret.Redact(string(p.stack)))
		}
	case g.err == nil && g.panicked == nil:
		g.err, g.panicked = err, p
	}
	g.mu.Unlock()
	g.stop()
}

// Wait waits for the Go goroutines and returns the first error, or re-panics
// the first relay panic once the session is stopped.
func (g *Guard) Wait() error {
	g.wg.Wait()
	g.mu.Lock()
	g.finished = true
	err, p := g.err, g.panicked
	g.mu.Unlock()
	if p != nil {
		panic(p)
	}
	return err
}

// LiveSink passes the recorder calls of detached relays only until End: an
// input relay may still read from the front after the connector returned, when
// the pipeline already closes the recording.
type LiveSink struct {
	mu    sync.Mutex
	ended bool
	rec   RecordingSink
}

// NewLiveSink wraps rec.
func NewLiveSink(rec RecordingSink) *LiveSink { return &LiveSink{rec: rec} }

// Input records user input until End.
func (l *LiveSink) Input(p []byte) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if !l.ended {
		l.rec.Input(p)
	}
}

// Resize records a window change until End.
func (l *LiveSink) Resize(cols, rows int) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if !l.ended {
		l.rec.Resize(cols, rows)
	}
}

// End stops passing calls.
func (l *LiveSink) End() {
	l.mu.Lock()
	l.ended = true
	l.mu.Unlock()
}
