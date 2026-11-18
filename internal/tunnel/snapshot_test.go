package tunnel

import (
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"google.golang.org/protobuf/proto"

	gatewayv1 "github.com/werbot/werbot/gen/gateway/v1"
	"github.com/werbot/werbot/internal/platform/apperr"
)

// A refresh reads the organization-wide part once and completes it per pool:
// every stream gets exactly its own pool's snapshot.
func TestRefreshBuildsOrgPartOnce(t *testing.T) {
	b := newBench(t, func(h *Hub) { h.debounce = 20 * time.Millisecond })
	pools := []uuid.UUID{uuid.New(), uuid.New(), uuid.New()}
	conns := map[*conn]uuid.UUID{}
	for _, p := range pools {
		for range 2 {
			c := b.instance(t, b.org, p).connect(t, nil)
			c.next(t, 2*time.Second)
			conns[c] = p
		}
	}
	orgBase := b.f.orgBuildCount(b.org)
	poolBase := map[uuid.UUID]int{}
	for _, p := range pools {
		poolBase[p] = b.f.buildCount(b.org, p)
	}

	b.hub.OrgChanged(b.org.String())
	for c, p := range conns {
		got := c.next(t, 2*time.Second)
		want := &gatewayv1.Snapshot{CedarPolicies: "permit(principal, action, resource);",
			Members: []*gatewayv1.Member{{UserId: b.org.String()}}, Blocklist: []string{p.String()}}
		want.Version = snapshotVersion(want)
		if !proto.Equal(got, want) {
			t.Fatalf("pool %s got %v, want %v", p, got, want)
		}
	}
	if n := b.f.orgBuildCount(b.org) - orgBase; n != 1 {
		t.Fatalf("org part built %d times in one refresh, want 1", n)
	}
	for _, p := range pools {
		if n := b.f.buildCount(b.org, p) - poolBase[p]; n != 1 {
			t.Fatalf("pool %s built %d times, want 1", p, n)
		}
	}
}

// The pools of one refresh build concurrently, at most poolParallel at once,
// so a slow pool does not delay the others.
func TestRefreshBuildsPoolsConcurrently(t *testing.T) {
	b := newBench(t, func(h *Hub) { h.debounce = 10 * time.Millisecond })
	conns := make([]*conn, 2*poolParallel)
	for i := range conns {
		conns[i] = b.instance(t, b.org, uuid.New()).connect(t, nil)
		conns[i].next(t, 2*time.Second)
	}

	b.f.buildDelay.Store(int64(200 * time.Millisecond))
	b.f.maxInflight.Store(0)
	start := time.Now()
	b.hub.OrgChanged(b.org.String())
	for _, c := range conns {
		c.next(t, 3*time.Second)
	}
	if d := time.Since(start); d >= time.Duration(len(conns))*200*time.Millisecond/2 {
		t.Fatalf("%d pool builds took %v, want concurrent", len(conns), d)
	}
	if m := b.f.maxInflight.Load(); m < 2 || m > poolParallel {
		t.Fatalf("max in-flight pool builds = %d, want 2..%d", m, poolParallel)
	}
}

// A refresh re-authenticates the streams of an organization concurrently,
// at most authParallel at once, and still closes only the rejected one.
func TestRefreshAuthenticatesConcurrently(t *testing.T) {
	b := newBench(t, func(h *Hub) { h.debounce = 10 * time.Millisecond })
	pool := uuid.New()
	gws := make([]*gw, 2*authParallel)
	conns := make([]*conn, len(gws))
	for i := range gws {
		gws[i] = b.instance(t, b.org, pool)
		conns[i] = gws[i].connect(t, nil)
		conns[i].next(t, 2*time.Second)
	}

	b.f.with(func(f *fake) { f.authErr[gws[0].InstanceID] = apperr.ErrUnauthenticated })
	b.f.authDelay.Store(int64(100 * time.Millisecond))
	start := time.Now()
	b.hub.OrgChanged(b.org.String())
	wantCode(t, conns[0].closed(t, 3*time.Second), connect.CodeUnauthenticated, "")
	for _, c := range conns[1:] {
		c.next(t, 3*time.Second)
	}
	if d := time.Since(start); d >= time.Duration(len(gws))*100*time.Millisecond/2 {
		t.Fatalf("%d re-authentications took %v, want concurrent", len(gws), d)
	}
}
