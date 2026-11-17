package gateway

import (
	"cmp"
	"context"
	"hash/fnv"
	"math/rand/v2"
	"net"
	"slices"
	"strconv"
	"sync"
	"time"

	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/timestamppb"

	gatewayv1 "github.com/werbot/werbot/gen/gateway/v1"
	"github.com/werbot/werbot/internal/platform/metrics"
)

const (
	probersPerResource = 2                // instances that probe one resource
	healthEvery        = 60 * time.Second // ±healthJitter
	healthJitter       = 0.2
	healthRefresh      = 90 * time.Second // an unchanged result is queued again: the server counts results younger than 3 min
	healthRetry        = time.Second      // pause while no snapshot has arrived
	probeTimeout       = 5 * time.Second
	probeParallel      = 8    // concurrent dials
	maxHealth          = 1000 // max_items of Heartbeat.health; the rest waits for the next one
	maxHealthError     = 100  // max_bytes of ResourceHealth.error
)

// probers returns the n instances that probe a resource: those with the highest
// FNV-64a of resource id || instance id (rendezvous hashing), so a change in the
// pool moves only the resources of the instances that came or went.
//
//nolint:unparam // n is part of the contract; the caller passes probersPerResource
func probers(resourceID uuid.UUID, instances []uuid.UUID, n int) []uuid.UUID {
	score := func(id uuid.UUID) uint64 {
		h := fnv.New64a()
		_, _ = h.Write(resourceID[:])
		_, _ = h.Write(id[:])
		return h.Sum64()
	}
	out := slices.Clone(instances)
	slices.SortFunc(out, func(a, b uuid.UUID) int {
		if sa, sb := score(a), score(b); sa != sb { // highest first
			return -cmp.Compare(sa, sb)
		}
		return slices.Compare(a[:], b[:])
	})
	return out[:min(n, len(out))]
}

// healthResult is the outcome of one resource check.
type healthResult struct {
	ok  bool
	err string
}

// queued is a result and when it was last queued.
type queued struct {
	r  healthResult
	at time.Time
}

// prober checks the TCP targets of the pool resources this instance is a prober
// of and keeps the changed results until the next heartbeat takes them.
type prober struct {
	policy func() *Policy
	dial   func(ctx context.Context, addr string) error
	ok     *metrics.Counter
	fail   *metrics.Counter

	mu      sync.Mutex
	last    map[uuid.UUID]queued // what was last queued, per probed resource
	pending map[uuid.UUID]*gatewayv1.ResourceHealth
}

func newProber(policy func() *Policy, dial func(ctx context.Context, addr string) error, reg *metrics.Registry) *prober {
	const help = "Resource health checks by result."
	return &prober{
		policy: policy, dial: dial,
		ok:      reg.Counter("werbot_gateway_health_checks_total", help, "result", "ok"),
		fail:    reg.Counter("werbot_gateway_health_checks_total", help, "result", "fail"),
		last:    map[uuid.UUID]queued{},
		pending: map[uuid.UUID]*gatewayv1.ResourceHealth{},
	}
}

// tcpProbe is the real dial of a probe: a connect through dial (Session.Dial, so
// the deny-local control applies), closed at once.
func tcpProbe(dial func(ctx context.Context, network, address string) (net.Conn, error)) func(context.Context, string) error {
	return func(ctx context.Context, addr string) error {
		c, err := dial(ctx, "tcp", addr)
		if err == nil {
			_ = c.Close()
		}
		return err
	}
}

// run checks right away, then every healthEvery ±20 %, until ctx ends. self is
// this instance; instances is the pool (empty until the server sent it: self alone).
func (h *prober) run(ctx context.Context, self uuid.UUID, instances func() []uuid.UUID) {
	for {
		d := healthRetry
		if h.pass(ctx, self, instances()) {
			d = time.Duration(float64(healthEvery) * (1 + healthJitter*(2*rand.Float64()-1))) //nolint:gosec // jitter, not security
		}
		t := time.NewTimer(d)
		select {
		case <-ctx.Done():
			t.Stop()
			return
		case <-t.C:
		}
	}
}

// probeTarget is a resource this instance checks.
type probeTarget struct {
	id    uuid.UUID
	host  string
	ports []uint16
}

// pass checks the resources of the snapshot this instance is a prober of, at most
// probeParallel at a time, and records the changed results.
// It reports false when there is no snapshot yet.
func (h *prober) pass(ctx context.Context, self uuid.UUID, pool []uuid.UUID) bool {
	pol := h.policy()
	if pol == nil {
		return false
	}
	if len(pool) == 0 {
		pool = []uuid.UUID{self}
	}
	var mine []probeTarget
	for _, r := range pol.resources {
		ep, ok, err := r.Endpoint()
		if err != nil || !ok || ep.NoHealth || ep.Host == "" || !slices.Contains(probers(r.ID, pool, probersPerResource), self) {
			continue
		}
		t := probeTarget{id: r.ID, host: ep.Host}
		for _, p := range ep.Ports {
			t.ports = append(t.ports, p.Port)
		}
		mine = append(mine, t)
	}
	results := make([]healthResult, len(mine))
	sem := make(chan struct{}, probeParallel)
	var wg sync.WaitGroup
	for i, t := range mine {
		wg.Go(func() {
			sem <- struct{}{}
			defer func() { <-sem }()
			results[i] = h.check(ctx, t)
		})
	}
	wg.Wait()
	if ctx.Err() == nil { // a canceled pass reports nothing
		h.record(mine, results)
	}
	return true
}

// check connects to the first port of each range; the result is ok when all
// succeed, else the error of the first that failed. The text is cut to the
// server's bound and holds only what the dial reported: no credentials.
func (h *prober) check(ctx context.Context, t probeTarget) healthResult {
	for _, port := range t.ports {
		cctx, cancel := context.WithTimeout(ctx, probeTimeout)
		err := h.dial(cctx, net.JoinHostPort(t.host, strconv.Itoa(int(port))))
		cancel()
		if err != nil {
			return healthResult{err: TruncateUTF8(err.Error(), maxHealthError)}
		}
	}
	return healthResult{ok: true}
}

// record counts the results and queues those that differ from the last queued or
// were queued healthRefresh ago;
// resources no longer probed here are forgotten, so they report again if they return.
func (h *prober) record(mine []probeTarget, results []healthResult) {
	seen := make(map[uuid.UUID]bool, len(mine))
	now := time.Now()
	h.mu.Lock()
	defer h.mu.Unlock()
	for i, t := range mine {
		seen[t.id] = true
		r := results[i]
		if r.ok {
			h.ok.Inc()
		} else {
			h.fail.Inc()
		}
		if prev, known := h.last[t.id]; !known || prev.r != r || now.Sub(prev.at) >= healthRefresh {
			h.last[t.id] = queued{r, now}
			h.pending[t.id] = &gatewayv1.ResourceHealth{ResourceId: t.id.String(), Ok: r.ok, Error: r.err, CheckedAt: timestamppb.New(now)}
		}
	}
	for id := range h.last {
		if !seen[id] {
			delete(h.last, id)
		}
	}
}

// take returns up to maxHealth queued results and drops them from the queue.
func (h *prober) take() []*gatewayv1.ResourceHealth {
	h.mu.Lock()
	defer h.mu.Unlock()
	var out []*gatewayv1.ResourceHealth
	for id, r := range h.pending {
		if len(out) == maxHealth {
			break
		}
		out = append(out, r)
		delete(h.pending, id)
	}
	return out
}
