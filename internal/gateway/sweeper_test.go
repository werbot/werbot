package gateway

import (
	"context"
	"io"
	"net"
	"testing"
	"testing/synctest"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/durationpb"

	gatewayv1 "github.com/werbot/werbot/gen/gateway/v1"
)

// startSweeper runs the pipeline's sweeper until the test ends.
func startSweeper(t *testing.T, p *Pipeline) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { defer close(done); p.sweep(ctx) }()
	t.Cleanup(func() { cancel(); <-done })
}

// compileV compiles a test snapshot of version v.
func compileV(t *testing.T, m ids, v, policies string, rs ...res) *Policy {
	t.Helper()
	s := testSnapshot(t, m, policies, nil, rs...)
	s.Version = v
	p, err := CompilePolicy(s)
	require.NoError(t, err)
	return p
}

// runBlocker runs s until its context ends; the returned channel closes when Run returned.
func (e *pipeEnv) runBlocker(t *testing.T, s *Session) <-chan struct{} {
	t.Helper()
	c, _ := LookupConnector("blocker")
	done := make(chan struct{})
	go func() { defer close(done); _ = e.p.Run(t.Context(), s, c) }()
	t.Cleanup(func() { <-done })
	return done
}

// endedWith reports the delivered end reasons of id once the bubble is idle.
func (e *pipeEnv) endedWith(id uuid.UUID) []string {
	synctest.Wait()
	return e.f.endsOf(id)
}

// withLimits sets the idle timeout and max duration of the snapshot resource "pg".
func withLimits(idle, maxDur time.Duration) func(*gatewayv1.Snapshot) {
	return func(s *gatewayv1.Snapshot) {
		if idle > 0 {
			s.Resources[2].IdleTimeout = durationpb.New(idle)
		}
		if maxDur > 0 {
			s.Resources[2].MaxDuration = durationpb.New(maxDur)
		}
	}
}

func TestIdleTimeoutSession(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		e := newPipeEnv(t, 10)
		e.f.setPolicy(e.plainSnapshot(t, plainPolicies, "v1", withLimits(time.Minute, 0)))
		e.p.cfg.IdleTimeout = time.Hour // the resource's value wins
		startSweeper(t, e.p)
		quiet, _, _ := e.runStream(t, "pg:5432")
		busy, cb, _ := e.runStream(t, "pg:5432")
		for range 4 {
			time.Sleep(30 * time.Second)
			roundTrip(t, cb, []byte("x"))
			if len(e.endedWith(quiet.ID)) > 0 {
				break
			}
		}
		require.Equal(t, []string{"idle_timeout"}, e.endedWith(quiet.ID), "no traffic for 61 s")
		require.Empty(t, e.endedWith(busy.ID), "traffic every 30 s keeps it")
	})
}

func TestMaxDuration(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		e := newPipeEnv(t, 10)
		e.f.setPolicy(e.plainSnapshot(t, plainPolicies, "v1", withLimits(0, 2*time.Minute)))
		startSweeper(t, e.p)
		limited, c, _ := e.runStream(t, "pg:5432")
		unlimited := e.open(t, "blocker", KindTerminal) // Config.MaxDuration 0, resource unset
		e.runBlocker(t, unlimited)
		time.Sleep(2*time.Minute - 5*time.Second)
		roundTrip(t, c, []byte("busy")) // traffic does not extend it
		require.Empty(t, e.endedWith(limited.ID))
		time.Sleep(10 * time.Second)
		require.Equal(t, []string{"max_duration"}, e.endedWith(limited.ID))
		time.Sleep(3 * time.Hour)
		require.Empty(t, e.endedWith(unlimited.ID), "no limit")
	})
}

func TestTerminalCloseMessage(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		e := newPipeEnv(t, 10)
		e.p.cfg.IdleTimeout = time.Minute
		startSweeper(t, e.p)
		s := e.open(t, "blocker", KindTerminal)
		client, server := net.Pipe()
		defer client.Close() //nolint:errcheck // test pipe
		s.Terminal = &Terminal{ReadWriter: server}
		done := e.runBlocker(t, s)
		time.Sleep(61 * time.Second)
		require.NoError(t, client.SetReadDeadline(time.Now().Add(time.Minute))) // fails, not hangs, without the text
		want := "Session closed: idle timeout\r\n"
		got := make([]byte, len(want))
		_, err := io.ReadFull(client, got)
		require.NoError(t, err)
		require.Equal(t, want, string(got))
		<-done
		require.Equal(t, []string{"idle_timeout"}, e.endedWith(s.ID))
	})
}

func TestRevokeOnSnapshot(t *testing.T) {
	e := newPipeEnv(t, 10)
	startSweeper(t, e.p)
	revoked := e.open(t, "fake", KindExec)
	kept := e.open(t, "blocker", KindExec)
	e.runBlocker(t, revoked)
	keptDone := e.runBlocker(t, kept)
	e.f.setPolicy(compileV(t, e.m, "v2", `permit (principal == Werbot::User::"u1", action, resource == Werbot::Resource::"rblocker");`,
		res{"rfake", "fake", "fake"}, res{"rblocker", "blocker", "blocker"}))
	require.Eventually(t, func() bool { return len(e.f.endsOf(revoked.ID)) == 1 }, time.Second, 5*time.Millisecond)
	require.Equal(t, []string{"access_revoked"}, e.f.endsOf(revoked.ID))
	select {
	case <-keptDone:
		t.Fatal("the session with a binding ended")
	case <-time.After(50 * time.Millisecond):
	}
}

func TestRevokeOnSchedule(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		e := newPipeEnv(t, 10)
		e.f.setPolicy(compileTest(t, e.m, `permit (principal, action, resource) when { context.hour < 10 };`, nil, res{"rblocker", "blocker", "blocker"}))
		startSweeper(t, e.p)
		s := e.open(t, "blocker", KindExec) // the bubble starts at 2000-01-01 00:00 UTC
		e.runBlocker(t, s)
		time.Sleep(9*time.Hour + 59*time.Minute + 30*time.Second)
		require.Empty(t, e.endedWith(s.ID))
		time.Sleep(time.Minute) // 10:00:30
		require.Equal(t, []string{"access_revoked"}, e.endedWith(s.ID))
	})
}

func TestRevokeAppliesToSessionOpenedDuringSnapshot(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		e := newPipeEnv(t, 10)
		startSweeper(t, e.p)
		revoke := compileV(t, e.m, "v2", `forbid (principal, action, resource);`, res{"rblocker", "blocker", "blocker"})
		e.f.onStart = func() { // the snapshot lands, and the sweeper handles it, between the PDP and add
			e.f.setPolicy(revoke)
			synctest.Wait()
		}
		s := e.open(t, "blocker", KindExec)
		e.runBlocker(t, s)
		require.Empty(t, e.endedWith(s.ID))
		time.Sleep(sweepEvery)
		require.Equal(t, []string{"access_revoked"}, e.endedWith(s.ID), "the next tick re-evaluates a session of an older snapshot")
	})
}

// A client close and a revoke race: the session ends once, with the first cause.
func TestRevokeRacesClose(t *testing.T) {
	e := newPipeEnv(t, 1)
	startSweeper(t, e.p)
	allow := e.f.Policy()
	revoke := compileV(t, e.m, "v2", `forbid (principal, action, resource);`, res{"rblocker", "blocker", "blocker"})
	c, _ := LookupConnector("blocker")
	n := 1000
	if raceEnabled {
		n = 200 // R20: the race detector and a spool fsync per end make 1000 iterations too slow
	}
	ids := make([]uuid.UUID, n)
	for i := range ids {
		e.f.setPolicy(allow)
		s := e.open(t, "blocker", KindExec)
		ids[i] = s.ID
		ctx, cancel := context.WithCancel(t.Context())
		revoked := make(chan struct{})
		go cancel()
		go func() { e.f.setPolicy(revoke); close(revoked) }()
		_ = e.p.Run(ctx, s, c)
		<-revoked
	}
	require.Eventually(t, func() bool {
		e.f.with(func(f *fakeControl) { n = len(f.ends) })
		return n == len(ids)
	}, 30*time.Second, 10*time.Millisecond)
	for _, id := range ids {
		reasons := e.f.endsOf(id)
		require.Len(t, reasons, 1)
		require.Contains(t, []string{"closed", "access_revoked"}, reasons[0])
	}
}
