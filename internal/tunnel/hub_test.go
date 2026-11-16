package tunnel

import (
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"

	gatewayv1 "github.com/werbot/werbot/gen/gateway/v1"
	"github.com/werbot/werbot/internal/platform/apperr"
)

func TestSnapshotVersion(t *testing.T) {
	a := &gatewayv1.Snapshot{CedarPolicies: "permit(principal, action, resource);", Blocklist: []string{"10.0.0.0/8"},
		Resources: []*gatewayv1.Resource{{Id: "r1", Name: "db", Type: "ssh", ConfigJson: []byte(`{"host":"db"}`)}}}
	b := &gatewayv1.Snapshot{CedarPolicies: a.CedarPolicies, Blocklist: []string{"10.0.0.0/8"},
		Resources: []*gatewayv1.Resource{{Id: "r1", Name: "db", Type: "ssh", ConfigJson: []byte(`{"host":"db"}`)}}, Version: "stale"}
	if v := snapshotVersion(a); len(v) != 64 || v != snapshotVersion(b) {
		t.Fatalf("equal content: %q vs %q", v, snapshotVersion(b))
	}
	if b.GetVersion() != "stale" {
		t.Fatal("snapshotVersion changed its argument")
	}
	b.Blocklist = []string{"192.168.0.0/16"}
	if snapshotVersion(a) == snapshotVersion(b) {
		t.Fatal("different blocklist, same version")
	}
}

func TestDebounceBuildsOncePerPool(t *testing.T) {
	b := newBench(t, func(h *Hub) { h.debounce = 100 * time.Millisecond })
	p, q := uuid.New(), uuid.New()
	conns := []*conn{
		b.instance(t, b.org, p).connect(t, nil),
		b.instance(t, b.org, p).connect(t, nil),
		b.instance(t, b.org, q).connect(t, nil),
	}
	for _, c := range conns {
		c.next(t, 2*time.Second)
	}
	base := map[uuid.UUID]int{p: b.f.buildCount(b.org, p), q: b.f.buildCount(b.org, q)}
	builds := func(pool uuid.UUID) int { return b.f.buildCount(b.org, pool) - base[pool] }

	org := b.org.String()
	for range 50 {
		b.hub.OrgChanged(org)
	}
	for _, c := range conns {
		c.next(t, 2*time.Second)
	}
	time.Sleep(150 * time.Millisecond)
	if builds(p) != 1 || builds(q) != 1 {
		t.Fatalf("builds P=%d Q=%d, want 1 each", builds(p), builds(q))
	}

	b.hub.OrgChanged(uuid.New().String())
	b.hub.OrgChanged("garbage")
	for _, c := range conns {
		c.quiet(t, 300*time.Millisecond)
	}

	b.hub.OrgChanged("*")
	for _, c := range conns {
		c.next(t, 2*time.Second)
	}
	time.Sleep(150 * time.Millisecond)
	if builds(p) != 2 || builds(q) != 2 {
		t.Fatalf("after *: builds P=%d Q=%d, want 2 each", builds(p), builds(q))
	}
}

func TestRevokedInstanceStreamClosed(t *testing.T) {
	b := newBench(t, func(h *Hub) { h.debounce = 50 * time.Millisecond })
	pool := uuid.New()
	a1, a2 := b.instance(t, b.org, pool), b.instance(t, b.org, pool)
	c1, c2 := a1.connect(t, nil), a2.connect(t, nil)
	c1.next(t, 2*time.Second)
	c2.next(t, 2*time.Second)

	b.f.with(func(f *fake) { f.authErr[a1.InstanceID] = apperr.ErrUnauthenticated })
	b.hub.OrgChanged(b.org.String())
	wantCode(t, c1.closed(t, 2*time.Second), connect.CodeUnauthenticated, "")
	c2.next(t, 2*time.Second)
}

func TestBuildErrorRetries(t *testing.T) {
	b := newBench(t, func(h *Hub) { h.debounce = 50 * time.Millisecond })
	pool := uuid.New()
	g := b.instance(t, b.org, pool)
	c := g.connect(t, nil)
	c.next(t, 2*time.Second)

	b.f.with(func(f *fake) { f.buildFail = 1 })
	b.hub.OrgChanged(b.org.String())
	c.next(t, 2*time.Second)
	if n := b.f.buildCount(b.org, pool); n != 3 {
		t.Fatalf("builds = %d, want connect + failed + retry = 3", n)
	}

	// Repeated failures back off: debounce, 2×, 4× before the fourth build succeeds.
	b.f.with(func(f *fake) { f.buildFail = 3 })
	start := time.Now()
	b.hub.OrgChanged(b.org.String())
	c.next(t, 3*time.Second)
	if d := time.Since(start); d < 350*time.Millisecond {
		t.Fatalf("three retries took %v, want backoff 50+50+100+200ms", d)
	}
	// Success resets the backoff.
	b.f.with(func(f *fake) { f.buildFail = 1 })
	start = time.Now()
	b.hub.OrgChanged(b.org.String())
	c.next(t, 3*time.Second)
	if d := time.Since(start); d > 300*time.Millisecond {
		t.Fatalf("retry after reset took %v, want about 100ms", d)
	}

	// No snapshot at connect: the stream fails closed.
	b.f.with(func(f *fake) { f.buildFail = 1 })
	wantCode(t, b.instance(t, b.org, pool).connect(t, nil).closed(t, 2*time.Second), connect.CodeUnavailable, "")
}

func TestHubOrgChangedWithoutHandler(t *testing.T) {
	h := NewHub()
	h.debounce = time.Millisecond
	h.OrgChanged(uuid.New().String())
	h.Resync()
	time.Sleep(20 * time.Millisecond)
	h.Shutdown()
	h.OrgChanged("*")
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.timers) != 0 {
		t.Fatal("a refresh was scheduled after Shutdown")
	}
}

func TestMailboxKeepsNewestBuild(t *testing.T) {
	s := &stream{wake: make(chan struct{}, 1)}
	older, newer := &gatewayv1.Snapshot{Version: "a"}, &gatewayv1.Snapshot{Version: "b"}
	s.offer(newer, 2)
	s.offer(older, 1)
	if got := s.take(); got != newer {
		t.Fatalf("take() = %v, want the newer build", got)
	}
	if s.take() != nil {
		t.Fatal("mailbox not empty after take")
	}
}

func TestHeartbeatOfDeletedInstanceClosesStream(t *testing.T) {
	b := newBench(t, nil)
	g := b.instance(t, b.org, uuid.New())
	c := g.connect(t, nil)
	c.next(t, 2*time.Second)
	b.f.with(func(f *fake) { f.hbErr = apperr.ErrNotFound })
	if err := c.send(&gatewayv1.Heartbeat{Version: "1.0"}); err != nil {
		t.Fatal(err)
	}
	wantCode(t, c.closed(t, 2*time.Second), connect.CodeUnauthenticated, "")
}

func TestBackoffDroppedWithLastStream(t *testing.T) {
	b := newBench(t, func(h *Hub) { h.debounce = 20 * time.Millisecond })
	c := b.instance(t, b.org, uuid.New()).connect(t, nil)
	c.next(t, 2*time.Second)

	b.f.with(func(f *fake) { f.buildFail = 1 << 20 })
	b.hub.OrgChanged(b.org.String())
	backoff := func() (int, time.Duration) {
		b.hub.mu.Lock()
		defer b.hub.mu.Unlock()
		return len(b.hub.backoff), b.hub.backoff[b.org]
	}
	eventually(t, func() bool { _, d := backoff(); return d > 0 }, "a failed refresh")

	c.close()
	eventually(t, func() bool { n, _ := backoff(); return n == 0 }, "backoff dropped with the last stream")

	// A refresh still in flight when the last stream left does not re-arm.
	eventually(t, func() bool {
		b.hub.mu.Lock()
		defer b.hub.mu.Unlock()
		return b.hub.timers[b.org] == nil
	}, "the pending retry to fire")
	b.hub.arm(b.org, true)
	if n, _ := backoff(); n != 0 {
		t.Fatal("a retry was armed for an organization without streams")
	}
}

func TestStarRefreshBoundedParallel(t *testing.T) {
	b := newBench(t, func(h *Hub) { h.debounce = 10 * time.Millisecond })
	conns := make([]*conn, 20)
	for i := range conns {
		conns[i] = b.instance(t, uuid.New(), uuid.New()).connect(t, nil)
		conns[i].next(t, 2*time.Second)
	}

	b.f.buildDelay.Store(int64(100 * time.Millisecond))
	b.f.maxInflight.Store(0)
	start := time.Now()
	b.hub.OrgChanged("*")
	for _, c := range conns {
		c.next(t, 2*time.Second)
	}
	if d := time.Since(start); d >= time.Second {
		t.Fatalf("20 org refreshes took %v, want < 1s (sequential needs 2s)", d)
	}
	if m := b.f.maxInflight.Load(); m < 2 || m > refreshParallel {
		t.Fatalf("max in-flight builds = %d, want 2..%d", m, refreshParallel)
	}
}
