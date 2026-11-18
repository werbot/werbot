package tunnel

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"log/slog"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"google.golang.org/protobuf/proto"

	gatewayv1 "github.com/werbot/werbot/gen/gateway/v1"
)

// Hub holds the tunnel streams of this replica by organization and pushes
// fresh snapshots after werbot_org_changed notifications.
type Hub struct {
	// Tests shorten these before the first stream.
	debounce, maxAge, hbTimeout time.Duration

	ctx    context.Context // cancelled by Shutdown
	cancel context.CancelFunc
	gen    atomic.Uint64 // build generation: a newer build wins in a mailbox
	auth   authCache     // mTLS instance checks, dropped by OrgChanged

	mu      sync.Mutex
	h       *Handler
	closed  bool
	streams map[uuid.UUID]map[*stream]struct{} // org -> streams
	timers  map[uuid.UUID]*time.Timer          // org (uuid.Nil: every org) -> pending refresh
	backoff map[uuid.UUID]time.Duration        // org -> next retry delay after a failed build
}

// Refresh limits: one organization's refresh deadline, the longest build retry
// delay and how many organizations a "*" refresh rebuilds at once.
const (
	refreshTimeout  = 10 * time.Second
	maxRetryDelay   = 30 * time.Second
	refreshParallel = 8
)

// NewHub returns an empty hub; New attaches it to a handler.
func NewHub() *Hub {
	ctx, cancel := context.WithCancel(context.Background())
	return &Hub{debounce: SnapshotDebounce, maxAge: StreamMaxAge, hbTimeout: HeartbeatTimeout, ctx: ctx, cancel: cancel,
		streams: map[uuid.UUID]map[*stream]struct{}{}, timers: map[uuid.UUID]*time.Timer{}, backoff: map[uuid.UUID]time.Duration{}}
}

func (h *Hub) attach(hd *Handler) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.h = hd
}

func (h *Hub) logger() *slog.Logger {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.h == nil {
		return slog.Default()
	}
	return h.h.log
}

// OrgChanged drops the cached instance checks of one organization and
// schedules a refresh of its streams ("*": all of them).
func (h *Hub) OrgChanged(payload string) {
	if payload == "*" {
		h.auth.drop(uuid.Nil)
		h.arm(uuid.Nil, false)
		return
	}
	org, err := uuid.Parse(payload)
	if err != nil || org == uuid.Nil {
		h.logger().Warn("tunnel: ignoring org_changed payload", "payload", payload)
		return
	}
	h.auth.drop(org)
	h.arm(org, false)
}

// Resync flushes the instance check cache and refreshes every stream; call it
// after each LISTEN, the first included: notifications may have been lost.
func (h *Hub) Resync() {
	h.auth.reset(true)
	h.OrgChanged("*")
}

// Shutdown closes every stream with CodeUnavailable "shutdown" and refuses new ones.
func (h *Hub) Shutdown() {
	h.mu.Lock()
	h.closed = true
	for key, t := range h.timers {
		t.Stop()
		delete(h.timers, key)
	}
	for _, set := range h.streams {
		for s := range set {
			s.close(unavailable("shutdown"))
		}
	}
	h.mu.Unlock()
	h.cancel()
}

// arm starts the refresh timer of key unless one is already pending. A retry
// waits debounce, then twice as long after each failure, up to maxRetryDelay;
// it is dropped once the organization has no streams left.
func (h *Hub) arm(key uuid.UUID, retry bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed || h.timers[key] != nil || retry && h.streams[key] == nil {
		return
	}
	delay := h.debounce
	if retry {
		delay = max(h.backoff[key], h.debounce)
		h.backoff[key] = min(2*delay, maxRetryDelay)
	}
	h.timers[key] = time.AfterFunc(delay, func() { h.refresh(key) })
}

// refresh re-authenticates and re-snapshots the streams of key, up to
// refreshParallel organizations at once; an organization whose build failed
// is re-armed, so it never keeps a stale snapshot.
func (h *Hub) refresh(key uuid.UUID) {
	h.mu.Lock()
	delete(h.timers, key)
	hd := h.h
	byOrg := map[uuid.UUID][]*stream{}
	for org, set := range h.streams {
		if key == uuid.Nil || key == org {
			for s := range set {
				byOrg[org] = append(byOrg[org], s)
			}
		}
	}
	h.mu.Unlock()
	gen := h.gen.Add(1)
	sem := make(chan struct{}, refreshParallel)
	var wg sync.WaitGroup
	for org, ss := range byOrg {
		sem <- struct{}{}
		wg.Go(func() {
			defer func() { <-sem }()
			ctx, cancel := context.WithTimeout(h.ctx, refreshTimeout)
			ok := hd.refreshOrg(ctx, org, ss, gen)
			cancel()
			if !ok {
				h.arm(org, true)
				return
			}
			h.mu.Lock()
			delete(h.backoff, org)
			h.mu.Unlock()
		})
	}
	wg.Wait()
}

func (h *Hub) register(g Gateway, serial string) (*stream, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		return nil, unavailable("shutdown")
	}
	s := &stream{gw: g, serial: serial, wake: make(chan struct{}, 1), done: make(chan struct{})}
	if h.streams[g.OrgID] == nil {
		h.streams[g.OrgID] = map[*stream]struct{}{}
	}
	h.streams[g.OrgID][s] = struct{}{}
	return s, nil
}

func (h *Hub) unregister(s *stream) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.streams[s.gw.OrgID], s)
	if len(h.streams[s.gw.OrgID]) == 0 {
		delete(h.streams, s.gw.OrgID)
		delete(h.backoff, s.gw.OrgID) // a pending retry finds no streams and stops
	}
}

// refreshOrg drops streams whose instance no longer authenticates, then builds
// one snapshot per pool and offers it to that pool's streams. It reports
// false when a build failed.
func (hd *Handler) refreshOrg(ctx context.Context, org uuid.UUID, ss []*stream, gen uint64) bool {
	pools := map[uuid.UUID][]*stream{}
	for _, s := range ss {
		if _, err := hd.d.Gateways.Authenticate(ctx, s.gw.ID, s.gw.InstanceID, s.serial); err != nil {
			hd.log.Info("tunnel: closing stream of a rejected gateway", "instance_id", s.gw.InstanceID, "error", err)
			s.close(authError(err))
			continue
		}
		pools[s.gw.ID] = append(pools[s.gw.ID], s)
	}
	ok := true
	for pool, ps := range pools {
		snap, err := hd.build(ctx, org, pool)
		if err != nil {
			hd.log.Error("tunnel: build snapshot, retrying", "org_id", org, "gateway_id", pool, "error", err)
			ok = false
			continue
		}
		for _, s := range ps {
			s.offer(snap, gen)
		}
	}
	return ok
}

// build returns the pool snapshot stamped with its version.
func (hd *Handler) build(ctx context.Context, org, pool uuid.UUID) (*gatewayv1.Snapshot, error) {
	snap, err := hd.d.Snapshots.Build(ctx, org, pool)
	if err != nil {
		return nil, err
	}
	snap.Version = snapshotVersion(snap)
	return snap, nil
}

// snapshotVersion is the hex SHA-256 of the deterministic encoding of s without its version.
func snapshotVersion(s *gatewayv1.Snapshot) string {
	c := proto.CloneOf(s)
	c.Version = ""
	// A marshal error (invalid UTF-8) also fails the Send, so the hash is never used.
	b, _ := proto.MarshalOptions{Deterministic: true}.Marshal(c)
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// PoolInterval is how often Run sends every pool its online instances.
const PoolInterval = 60 * time.Second

// Run sends the online instances of every pool with streams on this replica
// once per PoolInterval until ctx ends, then returns nil.
func (hd *Handler) Run(ctx context.Context) error {
	t := time.NewTicker(PoolInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-t.C:
			hd.sendPools(ctx)
		}
	}
}

// sendPools is one Run pass: one OnlineInstances query per pool.
func (hd *Handler) sendPools(ctx context.Context) {
	for pool, ss := range hd.hub.byPool() {
		hd.offerPool(ctx, pool, ss)
	}
}

// offerPool offers the sorted online instances of pool to ss; a failed query
// only logs, the next pass retries.
func (hd *Handler) offerPool(ctx context.Context, pool uuid.UUID, ss []*stream) {
	ctx, cancel := context.WithTimeout(ctx, refreshTimeout)
	defer cancel()
	ids, err := hd.d.Pools.OnlineInstances(ctx, pool)
	if err != nil {
		hd.log.Warn("tunnel: online pool instances", "gateway_id", pool, "error", err)
		return
	}
	p := &gatewayv1.PoolInstances{Ids: make([]string, len(ids))}
	for i, id := range ids {
		p.Ids[i] = id.String()
	}
	slices.Sort(p.Ids)
	for _, s := range ss {
		s.offerPool(p)
	}
}

// byPool returns the streams of this replica by pool.
func (h *Hub) byPool() map[uuid.UUID][]*stream {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := map[uuid.UUID][]*stream{}
	for _, set := range h.streams {
		for s := range set {
			out[s.gw.ID] = append(out[s.gw.ID], s)
		}
	}
	return out
}

func unavailable(msg string) error {
	return connect.NewError(connect.CodeUnavailable, errors.New(msg))
}
