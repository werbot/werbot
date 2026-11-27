package gateway

import (
	"context"
	"crypto/sha256"
	"fmt"
	"hash"
	"net"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// nextEvent is the next event of an observer, within a second.
func nextEvent(t *testing.T, ch <-chan ObserveEvent) ObserveEvent {
	t.Helper()
	select {
	case ev, ok := <-ch:
		require.True(t, ok, "the observer channel is closed")
		return ev
	case <-time.After(time.Second):
		t.Fatal("no observer event")
		return ObserveEvent{}
	}
}

// requireEnded checks that ch is closed within a second, after any events left.
func requireEnded(t *testing.T, ch <-chan ObserveEvent) {
	t.Helper()
	deadline := time.After(time.Second)
	for {
		select {
		case _, ok := <-ch:
			if !ok {
				return
			}
		case <-deadline:
			t.Fatal("the observer channel is still open")
		}
	}
}

// observe is a new observer of session id that ends with the test.
func (e *pipeEnv) observe(t *testing.T, id uuid.UUID) <-chan ObserveEvent {
	t.Helper()
	ch, cancel, err := e.p.active.Observe(id)
	require.NoError(t, err)
	t.Cleanup(cancel)
	return ch
}

// capturedSink records events as "o:<output>", "r:<cols>x<rows>" and
// "m:<marker>", and hashes the output like a recorder's chunk hash.
type capturedSink struct {
	nopSink
	mu     sync.Mutex
	events []string
	bytes  int
	sum    hash.Hash
}

func (c *capturedSink) Output(p []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.bytes += len(p)
	c.sum.Write(p)
	if len(p) < 64 {
		c.events = append(c.events, "o:"+string(p))
	}
}

func (c *capturedSink) Resize(cols, rows int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.events = append(c.events, fmt.Sprintf("r:%dx%d", cols, rows))
}

func (c *capturedSink) Marker(text string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.events = append(c.events, "m:"+text)
}

// captureEnv is a pipeline env whose recorded sessions record into the returned sink.
func captureEnv(t *testing.T) (*pipeEnv, *capturedSink) {
	t.Helper()
	e := newPipeEnv(t, 50)
	c := &capturedSink{sum: sha256.New()}
	e.p.cfg.Recording = func(uuid.UUID, int, int, string) RecordingSink { return c }
	return e, c
}

func TestObserveFanOut(t *testing.T) {
	e, rec := captureEnv(t)
	s := e.open(t, "blocker", KindTerminal)
	first, cancel, err := e.p.active.Observe(s.ID)
	require.NoError(t, err)
	second := e.observe(t, s.ID)
	for _, ch := range []<-chan ObserveEvent{first, second} {
		require.Equal(t, ObserveEvent{Window: Window{120, 40}}, nextEvent(t, ch), "the first event is the window")
	}
	s.Recording.Output([]byte("a"))
	s.Recording.Resize(100, 30)
	for _, ch := range []<-chan ObserveEvent{first, second} {
		require.Equal(t, ObserveEvent{Data: []byte("a")}, nextEvent(t, ch))
		require.Equal(t, ObserveEvent{Window: Window{100, 30}}, nextEvent(t, ch))
	}
	cancel()
	cancel() // idempotent
	requireEnded(t, first)
	s.Recording.Output([]byte("b"))
	require.Equal(t, ObserveEvent{Data: []byte("b")}, nextEvent(t, second))
	require.Equal(t, []string{"o:a", "r:100x30", "o:b"}, rec.events, "the recording gets everything once")
}

func TestObserveLimit(t *testing.T) {
	e := newPipeEnv(t, 10)
	s := e.open(t, "blocker", KindTerminal)
	cancels := make([]func(), maxObservers)
	for i := range cancels {
		_, c, err := e.p.active.Observe(s.ID)
		require.NoError(t, err)
		cancels[i] = c
	}
	_, _, err := e.p.active.Observe(s.ID)
	require.ErrorIs(t, err, errObserverLimit, "the 17th observer is refused")
	cancels[0]()
	_, c, err := e.p.active.Observe(s.ID)
	require.NoError(t, err, "a canceled observer frees its place")
	c()
	for _, c := range cancels[1:] {
		c()
	}
}

func TestObserveEndsWithSession(t *testing.T) {
	e := newPipeEnv(t, 10)
	s := e.open(t, "blocker", KindTerminal)
	ch := e.observe(t, s.ID)
	done := e.runBlocker(t, s)
	e.p.active.Terminate(s.ID, "terminated")
	requireEnded(t, ch)
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Run did not return")
	}
	ch, cancel, err := e.p.active.Observe(s.ID)
	require.NoError(t, err)
	cancel()
	requireEnded(t, ch)

	ch, _, err = e.p.active.Observe(uuid.New())
	require.NoError(t, err)
	requireEnded(t, ch)
	x, err := e.p.Open(t.Context(), e.cmdReq("fake", KindExec, "ls"))
	require.NoError(t, err)
	ch, _, err = e.p.active.Observe(x.ID)
	require.NoError(t, err)
	requireEnded(t, ch)
}

func TestObserveCopiesOutput(t *testing.T) {
	e := newPipeEnv(t, 10)
	s := e.open(t, "blocker", KindTerminal)
	ch := e.observe(t, s.ID)
	nextEvent(t, ch)
	buf := []byte("abc")
	s.Recording.Output(buf)
	copy(buf, "XYZ") // a connector reuses its buffer
	require.Equal(t, []byte("abc"), nextEvent(t, ch).Data)
}

func TestObserveSlowObserverDropsFrames(t *testing.T) {
	e, rec := captureEnv(t)
	chunk := make([]byte, 4<<10)
	const total, rounds = 10 << 20, 5
	write := func(r RecordingSink) time.Duration {
		start := time.Now()
		for range total / len(chunk) {
			r.Output(chunk)
		}
		return time.Since(start)
	}
	plain := e.open(t, "blocker", KindTerminal)
	s := e.open(t, "blocker", KindTerminal)
	ch := e.observe(t, s.ID) // never read
	alone, watched := time.Duration(1<<62), time.Duration(1<<62)
	done := make(chan struct{})
	go func() {
		defer close(done)
		for range rounds { // interleaved best of rounds: both see the same machine load
			alone, watched = min(alone, write(plain.Recording)), min(watched, write(s.Recording))
		}
	}()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("the session waits for its observer")
	}
	require.LessOrEqual(t, watched, alone*3/2, "a slow observer does not slow the session (alone %v)", alone)
	require.Equal(t, 2*rounds*total, rec.bytes, "the recording gets every byte")
	require.Len(t, ch, observerBuffer)
	require.Regexp(t, `werbot_gateway_observer_drops_total [1-9]`, e.metrics(t))
}

// consoleWatch serves a console: a prompt, a marker and a line of output,
// then runs until the session ends.
type consoleWatch struct{ funcConn }

func (consoleWatch) ServeConsole(ctx context.Context, s *Session) error {
	s.Recording.Output([]byte("alice@c:~$ "))
	s.Recording.Marker("ls [read]")
	s.Recording.Output([]byte("file\r\n"))
	<-ctx.Done()
	return ctx.Err()
}

func TestObserveConsoleSession(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		e := pathsPipeEnv(t, ids{}, pathsPolicies+consoleOn("rc"))
		s := e.open(t, "c", KindTerminal)
		require.True(t, s.Console)
		ch := e.observe(t, s.ID) // the type is not recorded: observed all the same
		client, server := net.Pipe()
		t.Cleanup(func() { _ = client.Close() })
		s.Terminal = &Terminal{ReadWriter: server}
		done := make(chan struct{})
		go func() { defer close(done); _ = e.p.Run(t.Context(), s, consoleWatch{}) }()
		synctest.Wait()
		e.p.active.Terminate(s.ID, "terminated")
		readText(t, client, "Session closed: terminated\r\n")
		<-done
		var got []ObserveEvent
		for ev := range ch {
			got = append(got, ev)
		}
		require.Equal(t, []ObserveEvent{{Window: Window{120, 40}}, {Data: []byte("alice@c:~$ ")}, {Data: []byte("file\r\n")}}, got,
			"observers get the prompt and the output, never a marker")
		require.Equal(t, []string{"terminated"}, e.endedWith(s.ID))
	})

	e, rec := captureEnv(t)
	s := e.open(t, "blocker", KindTerminal)
	ch := e.observe(t, s.ID)
	nextEvent(t, ch)
	s.Recording.Marker("ls [read]")
	s.Recording.Input([]byte("ls\r"))
	require.Equal(t, []string{"m:ls [read]"}, rec.events, "the marker is recorded once")
	require.Empty(t, ch, "observers get no marker and no input")
}
