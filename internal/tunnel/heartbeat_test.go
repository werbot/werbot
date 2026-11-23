package tunnel

import (
	"context"
	"math"
	"net/netip"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"buf.build/go/protovalidate"
	"connectrpc.com/connect"
	"github.com/google/uuid"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	gatewayv1 "github.com/werbot/werbot/gen/gateway/v1"
	"github.com/werbot/werbot/internal/platform/apperr"
)

// beatCall is one UpdateCounters, Deny or ReportHealth call of the fake.
type beatCall struct {
	orgID, instanceID uuid.UUID
	counters          []Counters
	denials           []Denial
	health            []ResourceHealth
}

// beatRecord is the fake's state for heartbeat payloads and pool lists.
type beatRecord struct {
	counters, denials, health []beatCall
	err                       error                     // returned by UpdateCounters, Deny and ReportHealth
	online                    map[uuid.UUID][]uuid.UUID // pool -> OnlineInstances result
	poolCalls                 []uuid.UUID
}

func (f *fake) UpdateCounters(_ context.Context, orgID, instanceID uuid.UUID, cs []Counters) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.beat.counters = append(f.beat.counters, beatCall{orgID: orgID, instanceID: instanceID, counters: cs})
	return f.beat.err
}

func (f *fake) Deny(_ context.Context, orgID uuid.UUID, ds []Denial) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.beat.denials = append(f.beat.denials, beatCall{orgID: orgID, denials: ds})
	return f.beat.err
}

func (f *fake) ReportHealth(_ context.Context, orgID, instanceID uuid.UUID, hs []ResourceHealth) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.beat.health = append(f.beat.health, beatCall{orgID: orgID, instanceID: instanceID, health: hs})
	return f.beat.err
}

func (f *fake) OnlineInstances(_ context.Context, gatewayID uuid.UUID) ([]uuid.UUID, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.beat.poolCalls = append(f.beat.poolCalls, gatewayID)
	return f.beat.online[gatewayID], nil
}

func TestHeartbeatForwardsCountersDenialsHealth(t *testing.T) {
	b := newBench(t, nil)
	g := b.instance(t, b.org, uuid.New())
	sid, user, res := uuid.New(), uuid.New(), uuid.New()
	at := time.Date(2026, 11, 17, 10, 0, 0, 0, time.UTC)
	hb := &gatewayv1.Heartbeat{Version: "1.0",
		Counters: []*gatewayv1.SessionCounters{{SessionId: sid.String(), BytesUp: 1, BytesDown: 2, DatagramsUp: 3, DatagramsDown: 4}},
		Denials: []*gatewayv1.Denial{
			{UserId: user.String(), KeyFingerprint: "SHA256:k", Resource: "db", Kind: "stream", Target: "db:5432",
				ClientIp: "::ffff:10.0.0.9", Reason: "target_not_allowed", At: timestamppb.New(at)},
			{Resource: "web", Reason: "auth_failed"},
		},
		Health: []*gatewayv1.ResourceHealth{
			{ResourceId: res.String(), Ok: false, Error: "refused", CheckedAt: timestamppb.New(at)},
			{ResourceId: uuid.New().String(), Ok: true}, // no checked_at: dropped
		},
	}
	c := g.connect(t, hb)
	c.next(t, 2*time.Second)
	eventually(t, func() (ok bool) {
		b.f.with(func(f *fake) { ok = len(f.beat.health) == 1 })
		return ok
	}, "heartbeat payload")
	b.f.with(func(f *fake) {
		want := []Counters{{SessionID: sid, BytesUp: 1, BytesDown: 2, DatagramsUp: 3, DatagramsDown: 4}}
		if cs := f.beat.counters; len(cs) != 1 || cs[0].orgID != b.org || cs[0].instanceID != g.InstanceID || !slices.Equal(cs[0].counters, want) {
			t.Fatalf("UpdateCounters(%+v), want %+v", cs, want)
		}
		wantD := []Denial{
			{UserID: &user, KeyFingerprint: "SHA256:k", Resource: "db", Kind: "stream", Target: "db:5432",
				ClientIP: netip.MustParseAddr("10.0.0.9"), Reason: "target_not_allowed", At: at},
			{Resource: "web", Reason: "auth_failed"},
		}
		ds := f.beat.denials
		if len(ds) != 1 || ds[0].orgID != b.org || len(ds[0].denials) != 2 || *ds[0].denials[0].UserID != user ||
			ds[0].denials[1].UserID != nil {
			t.Fatalf("Deny(%+v)", ds)
		}
		for i := range wantD {
			got := ds[0].denials[i]
			got.UserID, wantD[i].UserID = nil, nil
			if !reflect.DeepEqual(got, wantD[i]) {
				t.Fatalf("denial %d = %+v, want %+v", i, got, wantD[i])
			}
		}
		wantH := []ResourceHealth{{ResourceID: res, Error: "refused", CheckedAt: at}}
		if hs := f.beat.health; hs[0].orgID != b.org || hs[0].instanceID != g.InstanceID || !slices.Equal(hs[0].health, wantH) {
			t.Fatalf("ReportHealth(%+v), want %+v", hs, wantH)
		}
		f.beat.err = apperr.ErrUnavailable // a failing store only logs
	})

	if err := c.send(hb); err != nil {
		t.Fatal(err)
	}
	if err := c.send(&gatewayv1.Heartbeat{Version: "1.0"}); err != nil {
		t.Fatal(err)
	}
	eventually(t, func() (ok bool) {
		b.f.with(func(f *fake) { ok = len(f.hbs) == 3 })
		return ok
	}, "heartbeats after failed stores")
	c.quiet(t, 100*time.Millisecond) // still open
	b.f.with(func(f *fake) {
		if len(f.beat.counters) != 2 || len(f.beat.denials) != 2 || len(f.beat.health) != 2 {
			t.Fatalf("an empty heartbeat called the stores: %+v", f.beat)
		}
	})
}

func TestHeartbeatPayloadValidated(t *testing.T) {
	b := newBench(t, nil)
	for name, hb := range map[string]*gatewayv1.Heartbeat{
		"reason":       {Denials: []*gatewayv1.Denial{{Reason: "not_ready"}}},
		"user":         {Denials: []*gatewayv1.Denial{{UserId: "bob", Reason: "denied"}}},
		"ip":           {Denials: []*gatewayv1.Denial{{ClientIp: "x", Reason: "denied"}}},
		"session":      {Counters: []*gatewayv1.SessionCounters{{SessionId: "x"}}},
		"health error": {Health: []*gatewayv1.ResourceHealth{{ResourceId: uuid.NewString(), Error: strings.Repeat("e", 101)}}},
		// bounds count bytes: 257 four-byte characters are 1028 bytes
		"target bytes":  {Denials: []*gatewayv1.Denial{{Target: strings.Repeat("😀", 257), Reason: "denied"}}},
		"version bytes": {Version: strings.Repeat("😀", 17)},
		"ip zone":       {Denials: []*gatewayv1.Denial{{ClientIp: "fe80::1%" + strings.Repeat("z", 57), Reason: "denied"}}},
	} {
		c := b.instance(t, b.org, uuid.New()).connect(t, hb)
		if err := c.closed(t, 2*time.Second); err.Code() != connect.CodeInvalidArgument {
			t.Fatalf("%s: %v", name, err)
		}
	}
}

// fill is n bytes of four-byte characters (plus ASCII for the rest).
func fill(n int) string { return strings.Repeat("😀", n/4) + strings.Repeat("x", n%4) }

// The largest heartbeat validation lets through still fits the read limit.
func TestHeartbeatMaxSizeFitsReadLimit(t *testing.T) {
	id, ts := uuid.NewString(), &timestamppb.Timestamp{Seconds: math.MinInt64, Nanos: math.MinInt32}
	hb := &gatewayv1.Heartbeat{Version: fill(64)}
	for range 64 {
		hb.Connectors = append(hb.Connectors, fill(64))
	}
	for range 10000 {
		hb.ActiveSessionIds = append(hb.ActiveSessionIds, id)
	}
	for range 5000 {
		hb.Counters = append(hb.Counters, &gatewayv1.SessionCounters{SessionId: id,
			BytesUp: math.MinInt64, BytesDown: math.MinInt64, DatagramsUp: math.MinInt64, DatagramsDown: math.MinInt64})
	}
	for range 100 {
		hb.Denials = append(hb.Denials, &gatewayv1.Denial{UserId: id, KeyFingerprint: fill(128),
			Resource: fill(63), Kind: fill(16), Target: fill(1024),
			ClientIp: "fe80::ffff:ffff:ffff:ffff%" + strings.Repeat("z", 38), Reason: "target_not_allowed", At: ts, // 64 bytes
			SessionId: id, AgentRunId: id, Classes: []string{"destructive", "destructive", "destructive", "destructive", "destructive", "destructive", "destructive"},
			Paths: slices.Repeat([]string{fill(1024)}, 16), Reasons: slices.Repeat([]string{fill(128)}, 16)})
	}
	for range 1000 {
		hb.Health = append(hb.Health, &gatewayv1.ResourceHealth{ResourceId: id, Ok: true, Error: fill(100), CheckedAt: ts})
	}
	msg := &gatewayv1.ConnectRequest{Msg: &gatewayv1.ConnectRequest_Heartbeat{Heartbeat: hb}}
	if err := protovalidate.Validate(msg); err != nil {
		t.Fatalf("the worst case must be valid: %v", err)
	}
	if n := proto.Size(msg); n >= readMaxBytes {
		t.Fatalf("worst-case heartbeat is %d bytes, read limit %d", n, readMaxBytes)
	}
}

// Every pool with streams on this replica gets one OnlineInstances query per
// pass; all its streams receive the same sorted list.
func TestPoolInstancesTick(t *testing.T) {
	b := newBench(t, nil)
	pool, other := uuid.New(), uuid.New()
	low, high := uuid.MustParse("00000000-0000-4000-8000-000000000001"), uuid.MustParse("ffffffff-0000-4000-8000-000000000001")
	b.f.with(func(f *fake) { f.beat.online = map[uuid.UUID][]uuid.UUID{pool: {high, low}} })
	a, z := b.instance(t, b.org, pool).connectPools(t), b.instance(t, b.org, pool).connectPools(t)
	o := b.instance(t, uuid.New(), other).connectPools(t)
	want := []string{low.String(), high.String()}
	for _, c := range []*poolConn{a, z} { // sent right after the first snapshot
		if got := c.nextPool(t); !slices.Equal(got, want) {
			t.Fatalf("first pool list %v, want %v", got, want)
		}
	}
	if got := o.nextPool(t); len(got) != 0 {
		t.Fatalf("pool without online instances got %v", got)
	}

	b.f.with(func(f *fake) {
		f.beat.poolCalls = nil
		f.beat.online[pool] = []uuid.UUID{low}
	})
	b.hub.h.sendPools(context.Background())
	for _, c := range []*poolConn{a, z} {
		if got := c.nextPool(t); !slices.Equal(got, []string{low.String()}) {
			t.Fatalf("tick pool list %v", got)
		}
	}
	o.nextPool(t)
	b.f.with(func(f *fake) {
		slices.SortFunc(f.beat.poolCalls, func(x, y uuid.UUID) int { return strings.Compare(x.String(), y.String()) })
		wantCalls := []uuid.UUID{pool, other}
		slices.SortFunc(wantCalls, func(x, y uuid.UUID) int { return strings.Compare(x.String(), y.String()) })
		if !slices.Equal(f.beat.poolCalls, wantCalls) {
			t.Fatalf("OnlineInstances calls %v, want one per pool %v", f.beat.poolCalls, wantCalls)
		}
	})
}

// poolConn is a Connect stream that keeps the pool lists it receives.
type poolConn struct {
	pools chan []string
	err   chan error
}

func (g *gw) connectPools(t *testing.T) *poolConn {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	st := g.client.Connect(ctx)
	if err := st.Send(&gatewayv1.ConnectRequest{Msg: &gatewayv1.ConnectRequest_Heartbeat{Heartbeat: &gatewayv1.Heartbeat{}}}); err != nil {
		t.Fatal(err)
	}
	c := &poolConn{pools: make(chan []string, 8), err: make(chan error, 1)}
	go func() {
		for {
			m, err := st.Receive()
			if err != nil {
				c.err <- err
				return
			}
			if p := m.GetPoolInstances(); p != nil {
				c.pools <- p.GetIds()
			}
		}
	}()
	return c
}

func (c *poolConn) nextPool(t *testing.T) []string {
	t.Helper()
	select {
	case p := <-c.pools:
		return p
	case err := <-c.err:
		t.Fatalf("stream closed: %v", err)
	case <-time.After(2 * time.Second):
		t.Fatal("no pool list within 2s")
	}
	return nil
}

func TestHeartbeatHealthCountsDropped(t *testing.T) {
	at := timestamppb.Now()
	hs, dropped := heartbeatHealth([]*gatewayv1.ResourceHealth{
		{ResourceId: uuid.NewString(), CheckedAt: at},
		{ResourceId: uuid.NewString()},
		{ResourceId: uuid.NewString(), CheckedAt: &timestamppb.Timestamp{Nanos: -1}},
	})
	if len(hs) != 1 || dropped != 2 {
		t.Fatalf("kept %d, dropped %d; want 1, 2", len(hs), dropped)
	}
}
