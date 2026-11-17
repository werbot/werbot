package gateway

import (
	"context"
	"errors"
	"log/slog"
	"strings"
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
			require.Zero(t, e.p.active.used())
			require.Eventually(t, func() bool { return !e.inHeartbeat(s.ID) }, 2*time.Second, 5*time.Millisecond)
			require.Equal(t, []string{tc.want}, e.f.endsOf(s.ID), "exactly one EndSession")
			e.f.with(func(f *fakeControl) {
				require.ElementsMatch(t, []string{"end:" + tc.want, "close"}, f.events, "the end is spooled before Recording.Close")
			})
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
	require.Eventually(t, func() bool { return len(e.f.endsOf(panicky.ID)) == 1 }, 2*time.Second, 5*time.Millisecond)
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
	requireEnds(t, e.f, s.ID, "internal_error")
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
	requireEnds(t, e.f, s.ID, "error")
	require.Eventually(t, func() bool { return !e.inHeartbeat(s.ID) }, 2*time.Second, 5*time.Millisecond)
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
	requireEnds(t, e.f, short.ID, "closed")
	requireEnds(t, e.f, long.ID, "gateway_shutdown")

	t.Run("zero timeout", func(t *testing.T) {
		e := newPipeEnv(t, 10)
		s := e.open(t, "blocker", KindExec)
		go func() { _ = e.p.Run(t.Context(), s, c) }()
		start := time.Now()
		e.p.drain(0)
		require.Less(t, time.Since(start), 100*time.Millisecond)
		requireEnds(t, e.f, s.ID, "gateway_shutdown")
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
		requireEnds(t, e.f, s.ID, "gateway_shutdown")
	})
}

// requireEnds waits for the spool to deliver the ends of id, then checks them.
func requireEnds(t *testing.T, f *fakeControl, id uuid.UUID, reasons ...string) {
	t.Helper()
	require.Eventually(t, func() bool { return len(f.endsOf(id)) >= len(reasons) }, 2*time.Second, 5*time.Millisecond)
	time.Sleep(20 * time.Millisecond) // a second delivery would show up by now
	require.Equal(t, reasons, f.endsOf(id))
}

func TestIDsCapWarnsOnce(t *testing.T) {
	log := &syncBuffer{}
	a := newActiveSessions(metrics.NewRegistry().Gauge("werbot_test_sessions", "test"), slog.New(slog.NewTextHandler(log, nil)))
	sp := newTestSpool(t, &fakeControl{upErr: connect.NewError(connect.CodeUnavailable, errors.New("down"))})
	for range maxSessionIDs {
		a.add(uuid.New())
	}
	require.NoError(t, sp.End(uuid.New(), "closed"))
	for range 3 {
		require.Len(t, heartbeatSessionIDs(a, sp), maxSessionIDs)
	}
	require.Equal(t, 1, strings.Count(log.String(), "heartbeat full"))
	require.NoError(t, sp.End(uuid.New(), "closed"))
	heartbeatSessionIDs(a, sp)
	require.Equal(t, 2, strings.Count(log.String(), "heartbeat full"), "the omitted count changed")
}
