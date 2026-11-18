package tunnel

import (
	"testing"
	"time"

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

// waiting reports how many streams wait for the next build of (org, pool).
func waiting(hd *Handler, org, pool uuid.UUID) int {
	hd.newBuilds.mu.Lock()
	defer hd.newBuilds.mu.Unlock()
	if p := hd.newBuilds.pools[[2]uuid.UUID{org, pool}]; p != nil && p.next != nil {
		return p.next.joined
	}
	return 0
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
	eventually(t, func() bool { return waiting(b.hub.h, b.org, pool) == len(conns) }, "streams to wait for the next build")
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
	eventually(t, func() bool { return waiting(b.hub.h, b.org, pool) == 1 }, "the late stream to wait")
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
