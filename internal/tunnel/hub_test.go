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
		time.Sleep(time.Millisecond)
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
