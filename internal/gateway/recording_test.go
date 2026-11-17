package gateway

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/gateway/recorder"
	"github.com/werbot/werbot/internal/platform/metrics"
)

func (f *fakeControl) UploadChunk(context.Context, uuid.UUID, int, []byte, []byte) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.upErr
}

func (f *fakeControl) FinishRecording(context.Context, recorder.Manifest) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.upErr
}

// newTestSpool is a running spool in a temp dir with fast retries.
func newTestSpool(t *testing.T, up recorder.Uploader) *recorder.Spool {
	t.Helper()
	sp, err := recorder.OpenSpool(t.TempDir(), up, recorder.SpoolOptions{RetryMin: 10 * time.Millisecond, RetryMax: 50 * time.Millisecond},
		slog.New(slog.DiscardHandler))
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- sp.Run(ctx) }()
	t.Cleanup(func() {
		cancel()
		require.NoError(t, <-done)
	})
	return sp
}

var errDown = connect.NewError(connect.CodeUnavailable, errors.New("down"))

func TestRecorderSatisfiesSink(*testing.T) {
	var _ RecordingSink = (*recorder.Recorder)(nil)
	var _ recorder.Uploader = (*tunnelClient)(nil)
}

// bound reports whether Run has bound the session's cancel.
func bound(a *activeSessions, id uuid.UUID) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.cancels[id] != nil
}

func TestTerminateEndsSessionWithReason(t *testing.T) {
	e := newPipeEnv(t, 10)
	s := e.open(t, "blocker", KindTerminal)
	c, _ := LookupConnector("blocker")
	done := make(chan error, 1)
	go func() { done <- e.p.Run(t.Context(), s, c) }()
	require.Eventually(t, func() bool { return bound(e.p.active, s.ID) }, 2*time.Second, time.Millisecond)

	e.p.active.Terminate(uuid.New(), recorder.EndReasonRecordingUnavailable) // unknown: no-op
	e.p.active.Terminate(s.ID, recorder.EndReasonRecordingUnavailable)
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not return after Terminate")
	}
	requireEnds(t, e.f, s.ID, "recording_unavailable")
}

func TestRunKeepsSessionInHeartbeatUntilEndDelivered(t *testing.T) {
	e := newPipeEnv(t, 10)
	e.f.with(func(f *fakeControl) { f.upErr = errDown })
	s := e.open(t, "fake", KindExec)
	require.NoError(t, e.p.Run(t.Context(), s, fakeConn{}))
	require.Equal(t, []string{s.ID.String()}, heartbeatSessionIDs(e.p.active, e.p.spool))
	time.Sleep(50 * time.Millisecond)
	require.Empty(t, e.f.endsOf(s.ID), "no EndSession while the server is down")

	e.f.with(func(f *fakeControl) { f.upErr = nil })
	requireEnds(t, e.f, s.ID, "closed")
	require.Eventually(t, func() bool { return len(heartbeatSessionIDs(e.p.active, e.p.spool)) == 0 }, 2*time.Second, 5*time.Millisecond)
}

func TestOpenRejectsWhenSpoolFull(t *testing.T) {
	e := newPipeEnv(t, 10)
	e.p.cfg.SpoolMax = 100
	e.f.with(func(f *fakeControl) { f.upErr = errDown })
	a := e.open(t, "blocker", KindTerminal)
	c, _ := LookupConnector("blocker")
	done := make(chan error, 1)
	ran := make(chan struct{})
	go func() { done <- e.p.Run(t.Context(), a, c); close(ran) }()
	t.Cleanup(func() { <-ran }) // t.Context ends first; the end is spooled before TempDir goes

	h := make([]byte, 32)
	require.NoError(t, e.p.spool.Append(uuid.New(), 0, h, make([]byte, 100)))
	require.EqualValues(t, 100, e.p.spool.Total())
	var auth, starts int
	e.f.with(func(f *fakeControl) { auth, starts = len(f.authCalls), f.startCalls })
	_, err := e.p.Open(t.Context(), e.req("fake", KindTerminal))
	require.ErrorIs(t, err, ErrSpoolMax)
	e.f.with(func(f *fakeControl) {
		require.Len(t, f.authCalls, auth, "AuthenticateUser is not called")
		require.Equal(t, starts, f.startCalls, "StartSession is not called")
	})
	require.Contains(t, e.metrics(t), `werbot_gateway_sessions_rejected_total{reason="spool_full"} 1`)
	require.Equal(t, 1, e.p.active.used())

	select {
	case err := <-done:
		t.Fatalf("the running session ended: %v", err)
	case <-time.After(50 * time.Millisecond):
	}
	require.NoError(t, e.p.spool.Append(a.ID, 0, h, make([]byte, 1<<10)), "a running session keeps its own limit")
}

// A spool that cannot write refuses new sessions like a full one, until a write
// succeeds again.
func TestOpenRejectsWhileSpoolCannotWrite(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores directory permissions")
	}
	e := newPipeEnv(t, 10)
	dir := t.TempDir()
	sp, err := recorder.OpenSpool(dir, e.f, recorder.SpoolOptions{}, slog.New(slog.DiscardHandler))
	require.NoError(t, err)
	e.p.spool = sp
	require.NoError(t, os.Chmod(dir, 0o500))       //nolint:gosec // a directory: read and search only
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) }) //nolint:gosec // a directory, as the spool makes it
	require.Error(t, sp.End(uuid.New(), "closed"), "the session directory cannot be created")

	_, err = e.p.Open(t.Context(), e.req("fake", KindTerminal))
	require.ErrorIs(t, err, ErrSpoolMax)
	require.Contains(t, e.metrics(t), `werbot_gateway_sessions_rejected_total{reason="spool_full"} 1`)
	require.Zero(t, e.p.active.used())

	require.NoError(t, os.Chmod(dir, 0o700)) //nolint:gosec // a directory, as the spool makes it
	e.open(t, "fake", KindTerminal)
}

func TestSSHFrontSpoolFull(t *testing.T) {
	e := newSSHEnv(t, Config{SpoolMax: 100})
	e.f.with(func(f *fakeControl) { f.upErr = errDown })
	require.NoError(t, e.p.spool.Append(uuid.New(), 0, make([]byte, 32), make([]byte, 100)))
	out, code := run(t, e.client(t, "alice+db"), false, "uptime")
	require.Equal(t, "Gateway is busy, try again later\r\n", out)
	require.NotZero(t, code)
	require.Zero(t, e.startCalls())
}

func TestEndReasonFallback(t *testing.T) {
	require.Equal(t, "closed", endReason(context.Background(), nil))

	a := newActiveSessions(metrics.NewRegistry().Gauge("g", "g"), slog.New(slog.DiscardHandler))
	id := uuid.New()
	ctx, cancel := context.WithCancelCause(context.Background())
	defer cancel(nil)
	a.add(id)
	a.bind(id, cancel)
	a.Terminate(id, recorder.EndReasonRecordingUnavailable)
	require.Equal(t, "recording_unavailable", endReason(ctx, context.Canceled))
}
