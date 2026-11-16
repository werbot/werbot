package sshconnector

import (
	"fmt"
	"log/slog"
	"runtime/debug"
	"sync"

	"github.com/werbot/werbot/internal/gateway"
)

// guard runs the relay goroutines of one session. The pipeline recovers only
// the Serve goroutine, so a relay recovers its own panic: the first panic or
// error stops the session; done re-panics a panic on the calling (Serve)
// goroutine, so the pipeline ends the session as "internal_error".
type guard struct {
	stop     func()
	secret   gateway.Secret // redacted from the log
	wg       sync.WaitGroup
	mu       sync.Mutex
	err      error
	panicked *relayPanic
	finished bool // done has returned: later failures have no session to end
}

// relayPanic is a recovered relay panic with the relay goroutine's stack.
type relayPanic struct {
	value any
	stack []byte
}

func (p *relayPanic) String() string {
	return fmt.Sprintf("ssh relay panic: %v\n\nrelay goroutine stack:\n%s", p.value, p.stack)
}

// spawn runs fn on a goroutine that done waits for.
func (g *guard) spawn(fn func() error) {
	g.wg.Go(func() { g.call(fn) })
}

// detach runs fn on a goroutine that done does not wait for: fn may block on
// a reader the session cannot interrupt, until the front closes it.
func (g *guard) detach(fn func() error) {
	go g.call(fn)
}

func (g *guard) call(fn func() error) {
	defer func() {
		if r := recover(); r != nil {
			g.fail(nil, &relayPanic{value: r, stack: debug.Stack()})
		}
	}()
	if err := fn(); err != nil {
		g.fail(err, nil)
	}
}

// fail keeps the first failure and stops the session. A panic after done
// returned is only logged (the stack, never relayed data), the secret redacted.
func (g *guard) fail(err error, p *relayPanic) {
	g.mu.Lock()
	switch {
	case g.finished:
		if p != nil {
			slog.Error("ssh connector: relay panic after the session ended",
				"panic", g.secret.Redact(fmt.Sprint(p.value)), "stack", g.secret.Redact(string(p.stack)))
		}
	case g.err == nil && g.panicked == nil:
		g.err, g.panicked = err, p
	}
	g.mu.Unlock()
	g.stop()
}

// done waits for the spawned goroutines and returns the first error, or
// re-panics the first relay panic once the session is stopped.
func (g *guard) done() error {
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
