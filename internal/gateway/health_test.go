package gateway

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/wrapperspb"

	"github.com/werbot/werbot/internal/connector"
	"github.com/werbot/werbot/internal/platform/metrics"
)

func init() {
	// "hprobe": the config string is the endpoint host; "nohealth" and "any" pick the flags
	connector.Register(connector.Type{Name: "hprobe", NewConfig: func() proto.Message { return &wrapperspb.StringValue{} },
		Validate: func(proto.Message) error { return nil }, Actions: []string{"hprobe.connect"},
		Kinds: []connector.Kind{connector.KindStream},
		Endpoint: func(cfg proto.Message) connector.Endpoint {
			host := cfg.(*wrapperspb.StringValue).GetValue()
			ep := connector.Endpoint{Host: host, Ports: []connector.PortRange{{Port: 22}, {Port: 2000, EndPort: 2005}}}
			switch host {
			case "nohealth":
				ep.NoHealth = true
			case "any":
				ep.AnyTarget = true
			}
			return ep
		}})
	connector.Register(connector.Type{Name: "hnoendpoint", NewConfig: func() proto.Message { return &wrapperspb.StringValue{} },
		Validate: func(proto.Message) error { return nil }, Actions: []string{"hnoendpoint.connect"},
		Kinds: []connector.Kind{connector.KindTerminal}})
}

// uid is a fixed UUID, so the rendezvous spread is the same on every run.
func uid(n int) uuid.UUID { return uuid.NewSHA1(uuid.NameSpaceOID, []byte("health-"+strconv.Itoa(n))) }

func TestProbersStable(t *testing.T) {
	pool := []uuid.UUID{uid(1), uid(2), uid(3), uid(4), uid(5)}
	rid := uid(100)
	got := probers(rid, pool, 2)
	require.Len(t, got, 2)
	require.Equal(t, got, probers(rid, slices.Clone(pool), 2))
	require.Equal(t, got, probers(rid, []uuid.UUID{pool[4], pool[3], pool[2], pool[1], pool[0]}, 2), "input order does not matter")
	for _, drop := range pool {
		if slices.Contains(got, drop) {
			continue
		}
		rest := slices.DeleteFunc(slices.Clone(pool), func(id uuid.UUID) bool { return id == drop })
		require.Equal(t, got, probers(rid, rest, 2), "removing an unchosen instance keeps the choice")
	}
	require.Len(t, probers(rid, pool[:1], 2), 1, "fewer instances than n")
	require.Empty(t, probers(rid, nil, 2))
}

func TestProbersSpread(t *testing.T) {
	pool := make([]uuid.UUID, 10)
	for i := range pool {
		pool[i] = uid(i)
	}
	count := map[uuid.UUID]int{}
	for r := range 1000 {
		for _, id := range probers(uid(1000+r), pool, 2) {
			count[id]++
		}
	}
	require.Len(t, count, 10)
	for id, n := range count {
		require.GreaterOrEqual(t, n, 150, id)
		require.LessOrEqual(t, n, 250, id)
	}
}

// dialLog is a fake dial that records the addresses and fails those in fail.
type dialLog struct {
	mu    sync.Mutex
	addrs []string
	fail  map[string]error
}

func (d *dialLog) dial(_ context.Context, addr string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.addrs = append(d.addrs, addr)
	return d.fail[addr]
}

func (d *dialLog) setFail(addr string, err error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.fail[addr] = err
}

// nextPass advances the fake clock until the prober dialed again, then lets the pass finish.
func nextPass(d *dialLog) {
	for {
		time.Sleep(time.Second)
		synctest.Wait()
		d.mu.Lock()
		n := len(d.addrs)
		d.mu.Unlock()
		if n > 0 {
			return
		}
	}
}

func (d *dialLog) take() []string {
	d.mu.Lock()
	defer d.mu.Unlock()
	out := slices.Sorted(slices.Values(d.addrs))
	d.addrs = nil
	return out
}

// healthEnv is a prober over resources of the given (type, config) pairs; ids are uid(i).
func healthEnv(dial func(context.Context, string) error, rs ...Resource) (*prober, *metrics.Registry) {
	reg := metrics.NewRegistry()
	pol := &Policy{resources: rs}
	return newProber(func() *Policy { return pol }, dial, reg), reg
}

func hres(n int, typ, cfg string) Resource {
	return Resource{ID: uid(n), Name: "r" + strconv.Itoa(n), Type: typ, Config: []byte(strconv.Quote(cfg))}
}

func TestHealthSkipsNoHealth(t *testing.T) {
	d := &dialLog{}
	h, _ := healthEnv(d.dial,
		hres(1, "hprobe", "10.0.0.1"), hres(2, "hprobe", "nohealth"), hres(3, "hprobe", ""),
		hres(4, "hnoendpoint", ""), hres(5, "hprobe", "any"), hres(6, "unknowntype", "x"))
	h.pass(t.Context(), uid(0), nil)
	require.Equal(t, []string{"10.0.0.1:2000", "10.0.0.1:22", "any:2000", "any:22"}, d.take(), "first port of each range; AnyTarget is probed")
	got := h.take()
	require.Len(t, got, 2)
	for _, r := range got {
		require.True(t, r.GetOk())
		require.NotNil(t, r.GetCheckedAt())
	}
}

func TestHealthSkipsOtherInstances(t *testing.T) {
	pool := []uuid.UUID{uid(1), uid(2), uid(3), uid(4)}
	rid := uid(1)
	chosen := probers(rid, pool, 2)
	var other uuid.UUID
	for _, id := range pool {
		if !slices.Contains(chosen, id) {
			other = id
		}
	}
	d := &dialLog{}
	h, _ := healthEnv(d.dial, hres(1, "hprobe", "10.0.0.1"))
	h.pass(t.Context(), other, pool)
	require.Empty(t, d.take())
	h.pass(t.Context(), chosen[0], pool)
	require.NotEmpty(t, d.take())
}

func TestHealthReportsChangesOnly(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		d := &dialLog{fail: map[string]error{}}
		h, reg := healthEnv(d.dial, hres(1, "hprobe", "10.0.0.1"))
		ctx, cancel := context.WithCancel(t.Context())
		done := make(chan struct{})
		go func() { defer close(done); h.run(ctx, uid(0), func() []uuid.UUID { return nil }) }()
		defer func() { cancel(); <-done }()

		synctest.Wait() // the first pass runs at once
		require.Len(t, d.take(), 2)
		first := h.take()
		require.Len(t, first, 1)
		require.True(t, first[0].GetOk())

		nextPass(d)
		require.Len(t, d.take(), 2)
		require.Empty(t, h.take(), "an unchanged result is not reported again")

		d.setFail("10.0.0.1:2000", errors.New("dial tcp 10.0.0.1:2000: connection refused"))
		nextPass(d)
		got := h.take()
		require.Len(t, got, 1)
		require.False(t, got[0].GetOk())
		require.Equal(t, "dial tcp 10.0.0.1:2000: connection refused", got[0].GetError())
		require.Empty(t, h.take(), "take empties the queue")

		out := scrape(t, reg)
		require.Contains(t, out, `werbot_gateway_health_checks_total{result="ok"} 2`)
		require.Contains(t, out, `werbot_gateway_health_checks_total{result="fail"} 1`)
	})
}

func scrape(t *testing.T, reg *metrics.Registry) string {
	t.Helper()
	rec := httptest.NewRecorder()
	reg.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	return rec.Body.String()
}

func TestHealthConcurrencyLimit(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var mu sync.Mutex
		var cur, peak int
		dial := func(context.Context, string) error {
			mu.Lock()
			cur++
			peak = max(peak, cur)
			mu.Unlock()
			time.Sleep(time.Second)
			mu.Lock()
			cur--
			mu.Unlock()
			return nil
		}
		rs := make([]Resource, 20)
		for i := range rs {
			rs[i] = hres(i, "hprobe", "10.0.0."+strconv.Itoa(i+1))
		}
		h, _ := healthEnv(dial, rs...)
		h.pass(t.Context(), uid(0), nil)
		require.Equal(t, probeParallel, peak)
		require.Len(t, h.take(), 20)
	})
}

func TestHealthErrorBounded(t *testing.T) {
	long := "dial tcp: " + strings.Repeat("ошибка ", 40)
	h, _ := healthEnv(func(context.Context, string) error { return errors.New(long) }, hres(1, "hprobe", "10.0.0.1"))
	h.pass(t.Context(), uid(0), nil)
	got := h.take()
	require.Len(t, got, 1)
	require.LessOrEqual(t, len(got[0].GetError()), maxHealthError)
	require.True(t, utf8.ValidString(got[0].GetError()))
	require.NotEmpty(t, got[0].GetError())
}

func TestHealthDeniedLocalTarget(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer ln.Close() //nolint:errcheck // test listener
	_, port, _ := net.SplitHostPort(ln.Addr().String())
	n, _ := strconv.Atoi(port)

	probe := func(deny bool) healthResult {
		h, _ := healthEnv(tcpProbe(localGuard{deny: deny}.dialer()), hres(1, "hprobe", "127.0.0.1"))
		return h.check(t.Context(), probeTarget{host: "127.0.0.1", ports: []uint16{uint16(n)}}) //nolint:gosec // test port
	}
	require.True(t, probe(false).ok, "allowed: the listener answers")
	got := probe(true)
	require.False(t, got.ok, "denied: no loopback health oracle")
	require.Contains(t, got.err, ErrTargetNotAllowed.Error())
}

func TestHealthResendsUnchangedAfterRefresh(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		d := &dialLog{}
		h, _ := healthEnv(d.dial, hres(1, "hprobe", "10.0.0.1"))
		ctx, cancel := context.WithCancel(t.Context())
		done := make(chan struct{})
		go func() { defer close(done); h.run(ctx, uid(0), func() []uuid.UUID { return nil }) }()
		defer func() { cancel(); <-done }()

		synctest.Wait()
		require.Len(t, h.take(), 1)
		start := time.Now()
		for time.Since(start) < healthRefresh+healthEvery*2 { // a pass at or after the refresh age queues it again
			time.Sleep(time.Second)
			synctest.Wait()
			if got := h.take(); len(got) > 0 {
				require.GreaterOrEqual(t, time.Since(start), healthRefresh)
				require.True(t, got[0].GetOk())
				return
			}
		}
		t.Fatal("unchanged result was not sent again")
	})
}

func TestHealthRetriesWithoutSnapshot(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		d := &dialLog{}
		h, _ := healthEnv(d.dial, hres(1, "hprobe", "10.0.0.1"))
		var have atomic.Bool
		pol := h.policy()
		h.policy = func() *Policy {
			if have.Load() {
				return pol
			}
			return nil
		}
		ctx, cancel := context.WithCancel(t.Context())
		done := make(chan struct{})
		go func() { defer close(done); h.run(ctx, uid(0), func() []uuid.UUID { return nil }) }()
		defer func() { cancel(); <-done }()

		synctest.Wait()
		require.Empty(t, d.take())
		have.Store(true)
		time.Sleep(2 * healthRetry)
		synctest.Wait()
		require.Len(t, d.take(), 2, "the first pass follows the snapshot within seconds")
	})
}
