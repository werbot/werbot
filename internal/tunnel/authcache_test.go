package tunnel

import (
	"sync"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"

	gatewayv1 "github.com/werbot/werbot/gen/gateway/v1"
	"github.com/werbot/werbot/internal/platform/apperr"
	"github.com/werbot/werbot/internal/platform/tlsca"
)

type clock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *clock) now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *clock) add(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = c.t.Add(d)
}

// cachedBench is a bench whose hub listens (Resync ran) on a fake clock.
func cachedBench(t *testing.T) (*bench, *clock) {
	t.Helper()
	clk := &clock{t: time.Unix(1_700_000_000, 0)}
	b := newBench(t, func(h *Hub) { h.auth.now = clk.now })
	b.hub.Resync()
	return b, clk
}

// call makes one unary RPC; the unknown login gets NotFound once mTLS passed.
func (g *gw) call(t *testing.T) error {
	t.Helper()
	_, err := g.client.AuthenticateUser(ctx, connect.NewRequest(&gatewayv1.AuthenticateUserRequest{
		By: &gatewayv1.AuthenticateUserRequest_Login{Login: "nobody"}, ClientIp: "10.0.0.1"}))
	return err
}

func (b *bench) authCount() int {
	var n int
	b.f.with(func(f *fake) { n = len(f.auths) })
	return n
}

func TestAuthCacheHitSkipsDB(t *testing.T) {
	b, _ := cachedBench(t)
	g := b.instance(t, b.org, uuid.New())
	for range 3 {
		wantCode(t, g.call(t), connect.CodeNotFound, "")
	}
	if n := b.authCount(); n != 1 {
		t.Fatalf("Authenticate calls = %d, want 1", n)
	}
}

func TestAuthCacheOffWithoutListen(t *testing.T) {
	b := newBench(t, nil)
	g := b.instance(t, b.org, uuid.New())
	wantCode(t, g.call(t), connect.CodeNotFound, "")
	wantCode(t, g.call(t), connect.CodeNotFound, "")
	if n := b.authCount(); n != 2 {
		t.Fatalf("Authenticate calls = %d, want 2 before the first LISTEN", n)
	}
}

func TestAuthCacheDroppedByNotify(t *testing.T) {
	for name, payload := range map[string]func(org uuid.UUID) string{
		"org": func(org uuid.UUID) string { return org.String() },
		"all": func(uuid.UUID) string { return "*" },
	} {
		t.Run(name, func(t *testing.T) {
			b, _ := cachedBench(t)
			g := b.instance(t, b.org, uuid.New())
			other := b.instance(t, uuid.New(), uuid.New())
			wantCode(t, g.call(t), connect.CodeNotFound, "")
			wantCode(t, other.call(t), connect.CodeNotFound, "")

			// The instance is deleted (or revoked, or its pool deleted): the row is gone.
			b.f.with(func(f *fake) { delete(f.instances, g.InstanceID) })
			b.hub.OrgChanged(payload(b.org))
			wantCode(t, g.call(t), connect.CodeUnauthenticated, "")
			wantCode(t, other.call(t), connect.CodeNotFound, "")
			want := 3
			if name == "all" {
				want = 4
			}
			if n := b.authCount(); n != want {
				t.Fatalf("Authenticate calls = %d, want %d", n, want)
			}
		})
	}
}

func TestAuthCacheRenewedSerialMisses(t *testing.T) {
	b, _ := cachedBench(t)
	g := b.instance(t, b.org, uuid.New())
	wantCode(t, g.call(t), connect.CodeNotFound, "")
	cert, serial := b.clientCert(t, tlsca.GatewayCN(g.ID, g.InstanceID))
	renewed := &gw{Gateway: g.Gateway, serial: serial, client: b.clientFor(t, cert, nil)}

	wantCode(t, renewed.call(t), connect.CodeNotFound, "")
	b.f.with(func(f *fake) {
		if len(f.auths) != 2 || f.auths[1].serial != serial {
			t.Fatalf("Authenticate calls = %+v, want the new serial checked on the DB", f.auths)
		}
	})
	// The new serial replaced the entry: the old one goes to the DB again,
	// which refuses it once the new one was used (first-use semantics).
	b.f.with(func(f *fake) { f.authErr[g.InstanceID] = apperr.ErrUnauthenticated })
	wantCode(t, g.call(t), connect.CodeUnauthenticated, "")
}

func TestAuthCacheFlushedOnListenLoss(t *testing.T) {
	b, _ := cachedBench(t)
	g := b.instance(t, b.org, uuid.New())
	wantCode(t, g.call(t), connect.CodeNotFound, "")

	b.hub.ListenLost()
	wantCode(t, g.call(t), connect.CodeNotFound, "")
	wantCode(t, g.call(t), connect.CodeNotFound, "")
	if n := b.authCount(); n != 3 {
		t.Fatalf("Authenticate calls = %d, want 3: no cache while LISTEN is down", n)
	}

	b.hub.Resync()
	wantCode(t, g.call(t), connect.CodeNotFound, "")
	wantCode(t, g.call(t), connect.CodeNotFound, "")
	if n := b.authCount(); n != 4 {
		t.Fatalf("Authenticate calls = %d, want 4: cache back after LISTEN", n)
	}

	b.hub.Resync() // a reconnect may have lost notifications
	wantCode(t, g.call(t), connect.CodeNotFound, "")
	if n := b.authCount(); n != 5 {
		t.Fatalf("Authenticate calls = %d, want 5: reconnect flushes", n)
	}
}

func TestAuthCacheTTL(t *testing.T) {
	b, clk := cachedBench(t)
	g := b.instance(t, b.org, uuid.New())
	wantCode(t, g.call(t), connect.CodeNotFound, "")
	clk.add(authCacheTTL - time.Second)
	wantCode(t, g.call(t), connect.CodeNotFound, "")
	clk.add(time.Second)
	wantCode(t, g.call(t), connect.CodeNotFound, "")
	if n := b.authCount(); n != 2 {
		t.Fatalf("Authenticate calls = %d, want 2", n)
	}
}

// A check that started before a change notification is not stored.
func TestAuthCacheInFlightCheckNotStored(t *testing.T) {
	c := authCache{listening: true}
	org := uuid.New()
	g := Gateway{ID: uuid.New(), InstanceID: uuid.New(), OrgID: org, CertSerial: "a"}
	_, gen, ok := c.get(g.ID, g.InstanceID, "a")
	if ok {
		t.Fatal("hit on an empty cache")
	}
	c.drop(org)
	c.put(gen, "a", g)
	if _, _, ok := c.get(g.ID, g.InstanceID, "a"); ok {
		t.Fatal("a check from before the notification was cached")
	}
	// Only the current serial is cached, never the previous one.
	_, gen, _ = c.get(g.ID, g.InstanceID, "old")
	c.put(gen, "old", g)
	if _, _, ok := c.get(g.ID, g.InstanceID, "old"); ok {
		t.Fatal("a check under the previous serial was cached")
	}
}
