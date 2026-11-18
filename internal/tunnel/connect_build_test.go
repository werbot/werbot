package tunnel

import (
	"context"
	"errors"
	"log/slog"
	"runtime"
	"strings"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"

	gatewayv1 "github.com/werbot/werbot/gen/gateway/v1"
)

// holdBuilds makes every Pool call wait until the returned func runs.
func holdBuilds(t *testing.T, f *fake) (release func()) {
	t.Helper()
	ch := make(chan struct{})
	f.hold.Store(&ch)
	released := false
	release = func() {
		if !released {
			released = true
			f.hold.Store(nil)
			close(ch)
		}
	}
	t.Cleanup(release)
	return release
}

// waiting counts the streams parked in newSnapshot's wait for a build (the
// only select there), read from the goroutine dump: no product state for tests.
func waiting() int {
	buf := make([]byte, 1<<20)
	for {
		n := runtime.Stack(buf, true)
		if n < len(buf) {
			buf = buf[:n]
			break
		}
		buf = make([]byte, 2*len(buf))
	}
	count := 0
	for g := range strings.SplitSeq(string(buf), "\n\n") {
		if strings.Contains(g, " [select") && strings.Contains(g, "tunnel.(*Handler).newSnapshot(") {
			count++
		}
	}
	return count
}

func tagged(tag *string) func(_, _ uuid.UUID) *gatewayv1.Snapshot {
	return func(_, _ uuid.UUID) *gatewayv1.Snapshot { return &gatewayv1.Snapshot{CedarPolicies: *tag} }
}

// Streams that connect while a build of their pool runs share the next
// build: 1 + n Connects build twice, not 1 + n times.
func TestConcurrentConnectsShareBuild(t *testing.T) {
	b := newBench(t, nil)
	pool := uuid.New()
	release := holdBuilds(t, b.f)
	first := b.instance(t, b.org, pool).connect(t, nil)
	eventually(t, func() bool { return b.f.buildCount(b.org, pool) == 1 }, "the first build")

	conns := make([]*conn, 10)
	for i := range conns {
		conns[i] = b.instance(t, b.org, pool).connect(t, nil)
	}
	eventually(t, func() bool { return waiting() == 1+len(conns) }, "streams to wait for the builds")
	release()
	first.next(t, 2*time.Second)
	for _, c := range conns {
		c.next(t, 2*time.Second)
	}
	if n := b.f.buildCount(b.org, pool); n != 2 {
		t.Fatalf("%d Connects built %d times, want 2", 1+len(conns), n)
	}
}

// A build serves only streams that asked before it started: a Connect after
// a change never gets the snapshot of a build that read the data before it.
func TestConnectAfterChangeGetsNewBuild(t *testing.T) {
	b := newBench(t, nil)
	pool := uuid.New()
	tag := "old"
	b.f.with(func(f *fake) { f.snapshot = tagged(&tag) })
	release := holdBuilds(t, b.f)
	early := b.instance(t, b.org, pool).connect(t, nil)
	eventually(t, func() bool { return b.f.buildCount(b.org, pool) == 1 }, "the first build")

	b.f.with(func(*fake) { tag = "new" })
	late := b.instance(t, b.org, pool).connect(t, nil)
	eventually(t, func() bool { return waiting() == 2 }, "both streams to wait")
	release()
	if got := early.next(t, 2*time.Second).GetCedarPolicies(); got != "old" {
		t.Fatalf("early stream got %q, want old", got)
	}
	if got := late.next(t, 2*time.Second).GetCedarPolicies(); got != "new" {
		t.Fatalf("late stream got %q, want new", got)
	}

	// No build runs: an org change between Connects gives the next one a new build.
	b.f.with(func(*fake) { tag = "newer" })
	if got := b.instance(t, b.org, pool).connect(t, nil).next(t, 2*time.Second).GetCedarPolicies(); got != "newer" {
		t.Fatalf("stream after the change got %q, want newer", got)
	}
	if n := b.f.buildCount(b.org, pool); n != 3 {
		t.Fatalf("builds = %d, want 3", n)
	}
}

// A failed shared build reaches every stream waiting for it: each fails
// closed, and the next Connect builds anew.
func TestSharedBuildErrorReachesEveryWaiter(t *testing.T) {
	b := newBench(t, nil)
	pool := uuid.New()
	release := holdBuilds(t, b.f)
	first := b.instance(t, b.org, pool).connect(t, nil)
	eventually(t, func() bool { return b.f.buildCount(b.org, pool) == 1 }, "the first build")
	conns := make([]*conn, 3)
	for i := range conns {
		conns[i] = b.instance(t, b.org, pool).connect(t, nil)
	}
	eventually(t, func() bool { return waiting() == 1+len(conns) }, "streams to wait for the builds")
	b.f.with(func(f *fake) { f.buildFail = 1 }) // the next build, the shared one
	release()
	first.next(t, 2*time.Second)
	for _, c := range conns {
		wantCode(t, c.closed(t, 2*time.Second), connect.CodeUnavailable, "")
	}
	b.instance(t, b.org, pool).connect(t, nil).next(t, 2*time.Second)
	if n := b.f.buildCount(b.org, pool); n != 3 {
		t.Fatalf("builds = %d, want first + failed shared + new = 3", n)
	}
}

// A stream that leaves while its shared build runs returns at once; the
// build goes on and serves the streams still waiting.
func TestSharedBuildOutlivesLeavingWaiter(t *testing.T) {
	b := newBench(t, nil)
	pool := uuid.New()
	first, second := make(chan struct{}), make(chan struct{})
	b.f.hold.Store(&first)
	releaseSecond := sync.OnceFunc(func() { b.f.hold.Store(nil); close(second) })
	t.Cleanup(releaseSecond)
	early := b.instance(t, b.org, pool).connect(t, nil)
	eventually(t, func() bool { return b.f.buildCount(b.org, pool) == 1 }, "the first build")
	leaving, staying := b.instance(t, b.org, pool).connect(t, nil), b.instance(t, b.org, pool).connect(t, nil)
	eventually(t, func() bool { return waiting() == 3 }, "streams to wait for the builds")

	b.f.hold.Store(&second) // the shared build blocks on second
	close(first)
	early.next(t, 2*time.Second)
	eventually(t, func() bool { return b.f.buildCount(b.org, pool) == 2 }, "the shared build to run")
	leaving.cancel()
	eventually(t, func() bool { return hasLine(b.metric(t), "werbot_tunnel_streams 2") }, "the leaving stream's Connect to return")
	if n := waiting(); n != 1 {
		t.Fatalf("%d streams wait for the shared build, want 1", n)
	}

	releaseSecond()
	staying.next(t, 2*time.Second)
	if n := b.f.buildCount(b.org, pool); n != 2 {
		t.Fatalf("builds = %d, want 2: the shared build serves the staying stream", n)
	}
}

// A shared build that hangs (a Postgres partition) ends at refreshTimeout: it
// fails every stream waiting for it, and the next Connect builds anew.
func TestSharedBuildTimesOut(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		f, hub := newFake(), NewHub()
		defer hub.Shutdown()
		d := f.deps()
		d.Logger = slog.New(slog.DiscardHandler)
		hd := New(d, hub)
		hold := make(chan struct{})
		f.hold.Store(&hold)
		org, pool := uuid.New(), uuid.New()
		start := time.Now()
		errs := make(chan error, 4)
		ask := func() {
			_, err := hd.newSnapshot(t.Context(), org, pool)
			errs <- err
		}
		go ask()
		synctest.Wait() // the first build runs
		for range 3 {
			go ask() // the waiters of the next build
		}
		for range 4 {
			if err := <-errs; !errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("newSnapshot = %v, want a deadline error", err)
			}
		}
		if took := time.Since(start); took != 2*refreshTimeout {
			t.Fatalf("the two hung builds took %v, want %v", took, 2*refreshTimeout)
		}
		f.hold.Store(nil)
		if _, err := hd.newSnapshot(t.Context(), org, pool); err != nil {
			t.Fatalf("the next Connect: %v", err)
		}
		if n := f.buildCount(org, pool); n != 3 {
			t.Fatalf("builds = %d, want 2 hung + 1 new", n)
		}
	})
}
