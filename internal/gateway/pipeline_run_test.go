package gateway

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/platform/metrics"
)

func TestRunEndReasons(t *testing.T) {
	cases := []struct {
		name   string
		serve  func(context.Context, *Session) error
		cancel bool
		want   string
	}{
		{name: "serve ok", serve: func(context.Context, *Session) error { return nil }, want: "closed"},
		{name: "serve error", serve: func(context.Context, *Session) error { return errors.New("target refused") }, want: "error"},
		{name: "client cancel", serve: block, cancel: true, want: "closed"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := newPipeEnv(t, 10)
			s := e.open(t, "fake", KindExec)
			ctx, cancel := context.WithCancel(t.Context())
			if tc.cancel {
				time.AfterFunc(20*time.Millisecond, cancel)
			}
			defer cancel()
			_ = e.p.Run(ctx, s, funcConn{typ: "fake", kinds: []Kind{KindExec}, serve: tc.serve})
			require.Equal(t, []string{tc.want}, e.f.endsOf(s.ID), "exactly one EndSession")
			e.f.with(func(f *fakeControl) { require.Equal(t, []string{"end:" + tc.want, "close"}, f.events) })
			require.False(t, e.inHeartbeat(s.ID))
			require.Zero(t, e.p.active.used())
		})
	}
}

func TestPanicEndsOnlyOwnSession(t *testing.T) {
	e := newPipeEnv(t, 10)
	panicky := e.open(t, "panicky", KindTerminal)
	blocker := e.open(t, "blocker", KindTerminal)
	c, _ := LookupConnector("blocker")
	done := make(chan error, 1)
	go func() { done <- e.p.Run(t.Context(), blocker, c) }()

	pc, _ := LookupConnector("panicky")
	require.Error(t, e.p.Run(t.Context(), panicky, pc))
	require.Equal(t, []string{"internal_error"}, e.f.endsOf(panicky.ID))
	m := e.metrics(t)
	require.Contains(t, m, "werbot_gateway_panics_total 1")
	require.Contains(t, m, "werbot_gateway_sessions_active 1")
	require.Contains(t, e.log.String(), "boom")
	require.Contains(t, e.log.String(), "goroutine ", "the stack is logged")
	require.NotContains(t, e.log.String(), "s3cret-value")

	select {
	case err := <-done:
		t.Fatalf("the neighbour session ended: %v", err)
	case <-time.After(50 * time.Millisecond):
	}
	require.Empty(t, e.f.endsOf(blocker.ID))
	require.True(t, e.inHeartbeat(blocker.ID))
}

func TestPanicAfterCancelIsInternalError(t *testing.T) {
	e := newPipeEnv(t, 10)
	s := e.open(t, "panicky", KindTerminal)
	ctx, cancel := context.WithCancel(t.Context())
	time.AfterFunc(20*time.Millisecond, cancel)
	c := funcConn{typ: "panicky", kinds: []Kind{KindTerminal}, serve: func(ctx context.Context, _ *Session) error {
		<-ctx.Done()
		panic("boom on close")
	}}
	require.Error(t, e.p.Run(ctx, s, c))
	require.Equal(t, []string{"internal_error"}, e.f.endsOf(s.ID))
}

func TestEndSessionRetriedUntilDelivered(t *testing.T) {
	e := newPipeEnv(t, 10)
	unavailable := connect.NewError(connect.CodeUnavailable, errors.New("down"))
	s := e.open(t, "fake", KindExec)
	var listedBefore atomic.Int32
	e.f.with(func(f *fakeControl) {
		f.endErrs = []error{unavailable, unavailable, unavailable}
		f.onEnd = func(int) {
			if e.inHeartbeat(s.ID) {
				listedBefore.Add(1)
			}
		}
	})
	start := time.Now()
	require.NoError(t, e.p.Run(t.Context(), s, fakeConn{}))
	require.Less(t, time.Since(start), 100*time.Millisecond, "Run returns after the first attempt")
	require.True(t, e.inHeartbeat(s.ID))

	require.Eventually(t, func() bool { return len(e.f.endsOf(s.ID)) == 4 }, 2*time.Second, 5*time.Millisecond)
	require.Eventually(t, func() bool { return !e.inHeartbeat(s.ID) }, time.Second, 5*time.Millisecond)
	require.EqualValues(t, 4, listedBefore.Load(), "the id stays in the heartbeat during every attempt")
	time.Sleep(100 * time.Millisecond)
	require.Equal(t, []string{"closed", "closed", "closed", "closed"}, e.f.endsOf(s.ID))
}

func TestEndSessionStopsOnPermanentError(t *testing.T) {
	e := newPipeEnv(t, 10)
	s := e.open(t, "fake", KindExec)
	e.f.with(func(f *fakeControl) { f.endErrs = []error{connect.NewError(connect.CodeNotFound, errors.New("gone"))} })
	require.NoError(t, e.p.Run(t.Context(), s, fakeConn{}))
	require.False(t, e.inHeartbeat(s.ID))
	time.Sleep(100 * time.Millisecond)
	require.Len(t, e.f.endsOf(s.ID), 1)
}

func TestMaxSessionsSlotReleased(t *testing.T) {
	e := newPipeEnv(t, 1)
	s := e.open(t, "fake", KindExec)
	_, err := e.p.Open(t.Context(), e.req("fake", KindExec))
	require.ErrorIs(t, err, ErrBusy)
	require.NoError(t, e.p.Run(t.Context(), s, fakeConn{}))

	_, err = e.p.Open(t.Context(), e.req("denied", KindExec))
	require.ErrorIs(t, err, ErrDenied)
	s = e.open(t, "fake", KindExec) // the denied Open took no slot

	e.p.abort(s)
	require.Equal(t, []string{"error"}, e.f.endsOf(s.ID))
	require.False(t, e.inHeartbeat(s.ID))
	e.open(t, "fake", KindExec)
}

func TestDrain(t *testing.T) {
	e := newPipeEnv(t, 10)
	short := e.open(t, "fake", KindExec)
	long := e.open(t, "blocker", KindExec)
	shortConn := funcConn{typ: "fake", kinds: []Kind{KindExec}, serve: func(ctx context.Context, _ *Session) error {
		select {
		case <-time.After(50 * time.Millisecond):
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}}
	go func() { _ = e.p.Run(t.Context(), short, shortConn) }()
	c, _ := LookupConnector("blocker")
	go func() { _ = e.p.Run(t.Context(), long, c) }()

	start := time.Now()
	drained := make(chan struct{})
	go func() { e.p.drain(200 * time.Millisecond); close(drained) }()
	require.Eventually(t, func() bool { ok, r := e.p.ready(); return !ok && r == "draining" }, time.Second, time.Millisecond)
	_, err := e.p.Open(t.Context(), e.req("fake", KindExec))
	require.ErrorIs(t, err, ErrNotReady)

	<-drained
	took := time.Since(start)
	require.GreaterOrEqual(t, took, 200*time.Millisecond)
	require.Less(t, took, 400*time.Millisecond)
	require.Equal(t, []string{"closed"}, e.f.endsOf(short.ID))
	require.Equal(t, []string{"gateway_shutdown"}, e.f.endsOf(long.ID))

	t.Run("zero timeout", func(t *testing.T) {
		e := newPipeEnv(t, 10)
		s := e.open(t, "blocker", KindExec)
		go func() { _ = e.p.Run(t.Context(), s, c) }()
		start := time.Now()
		e.p.drain(0)
		require.Less(t, time.Since(start), 100*time.Millisecond)
		require.Equal(t, []string{"gateway_shutdown"}, e.f.endsOf(s.ID))
	})

	t.Run("waits for a pending end", func(t *testing.T) {
		e := newPipeEnv(t, 10)
		e.p.flushTimeout = time.Second
		s := e.open(t, "fake", KindExec)
		e.f.with(func(f *fakeControl) {
			f.endErrs = []error{connect.NewError(connect.CodeUnavailable, errors.New("down"))}
		})
		require.NoError(t, e.p.Run(t.Context(), s, fakeConn{}))
		start := time.Now()
		e.p.drain(0)
		require.Less(t, time.Since(start), 500*time.Millisecond)
		require.Len(t, e.f.endsOf(s.ID), 2, "drain returned after the retry delivered the end")
		require.False(t, e.inHeartbeat(s.ID))
	})

	t.Run("gives up a pending end after flushTimeout", func(t *testing.T) {
		e := newPipeEnv(t, 10)
		e.p.flushTimeout = 100 * time.Millisecond
		s := e.open(t, "fake", KindExec)
		e.f.with(func(f *fakeControl) { f.endErr = connect.NewError(connect.CodeUnavailable, errors.New("down")) })
		require.NoError(t, e.p.Run(t.Context(), s, fakeConn{}))
		start := time.Now()
		e.p.drain(0)
		took := time.Since(start)
		require.GreaterOrEqual(t, took, 100*time.Millisecond)
		require.Less(t, took, 300*time.Millisecond)
		waited := make(chan struct{})
		go func() { e.p.retries.Wait(); close(waited) }()
		select {
		case <-waited:
		case <-time.After(time.Second):
			t.Fatal("the retry goroutine outlived drain")
		}
		n := len(e.f.endsOf(s.ID))
		require.Greater(t, n, 1)
		time.Sleep(50 * time.Millisecond)
		require.Len(t, e.f.endsOf(s.ID), n, "no retry after stop")
		require.True(t, e.inHeartbeat(s.ID), "an undelivered end keeps its id")
	})

	t.Run("bounds the wait for sessions that ignore the cancel", func(t *testing.T) {
		e := newPipeEnv(t, 10)
		e.p.flushTimeout = 50 * time.Millisecond
		s := e.open(t, "fake", KindExec) // never run nor aborted
		start := time.Now()
		e.p.drain(0)
		require.Less(t, time.Since(start), 300*time.Millisecond)
		require.Contains(t, e.log.String(), s.ID.String())
	})

	t.Run("opened before drain, run after", func(t *testing.T) {
		e := newPipeEnv(t, 10)
		s := e.open(t, "blocker", KindExec)
		drained := make(chan struct{})
		go func() { e.p.drain(0); close(drained) }()
		time.Sleep(20 * time.Millisecond)
		_ = e.p.Run(t.Context(), s, c) // the drain already ended: Run starts cancelled
		<-drained
		require.Equal(t, []string{"gateway_shutdown"}, e.f.endsOf(s.ID))
	})
}

// An end whose first attempt fails after drain closed the retry gate gets no
// retry goroutine (drain already waits on retries) and keeps its id pending.
func TestEndAfterRetryGateClosedNoRetry(t *testing.T) {
	e := newPipeEnv(t, 10)
	e.f.with(func(f *fakeControl) { f.endErr = connect.NewError(connect.CodeUnavailable, errors.New("down")) })
	e.p.active.closeRetries()
	id := uuid.New()
	e.p.endSession(id, "closed")
	time.Sleep(3 * e.p.backoffMin)
	require.Len(t, e.f.endsOf(id), 1, "no retry after the gate closed")
	require.Contains(t, e.p.active.ids(), id.String())
	require.Contains(t, e.log.String(), "end not delivered before stop")
}

// An EndSession the server accepted is delivered even when stop came during the call.
func TestEndDeliveredAtStopIsDelivered(t *testing.T) {
	t.Run("first attempt", func(t *testing.T) {
		e := newPipeEnv(t, 10)
		e.f.with(func(f *fakeControl) { f.onEnd = func(int) { e.p.stopNow() } })
		id := uuid.New()
		e.p.endSession(id, "closed")
		require.Empty(t, e.p.active.ids())
		require.NotContains(t, e.log.String(), "not delivered")
	})
	t.Run("retry", func(t *testing.T) {
		e := newPipeEnv(t, 10)
		e.f.with(func(f *fakeControl) {
			f.endErrs = []error{connect.NewError(connect.CodeUnavailable, errors.New("down"))}
			f.onEnd = func(n int) {
				if n == 2 {
					e.p.stopNow()
				}
			}
		})
		id := uuid.New()
		e.p.endSession(id, "closed")
		e.p.retries.Wait()
		require.Len(t, e.f.endsOf(id), 2)
		require.Empty(t, e.p.active.ids())
		require.NotContains(t, e.log.String(), "not delivered")
	})
}

func TestIDsCapWarnsOnce(t *testing.T) {
	log := &syncBuffer{}
	a := newActiveSessions(metrics.NewRegistry().Gauge("werbot_test_sessions", "test"), slog.New(slog.NewTextHandler(log, nil)))
	for range maxSessionIDs + 1 {
		a.done(uuid.New(), true)
	}
	for range 3 {
		require.Len(t, a.ids(), maxSessionIDs)
	}
	require.Equal(t, 1, strings.Count(log.String(), "heartbeat full"))
	a.done(uuid.New(), true)
	a.ids()
	require.Equal(t, 2, strings.Count(log.String(), "heartbeat full"), "the omitted count changed")
}
