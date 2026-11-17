package tunnel

import (
	"context"
	"errors"
	"io"
	"math/rand/v2"
	"net/netip"
	"sync"
	"sync/atomic"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"

	gatewayv1 "github.com/werbot/werbot/gen/gateway/v1"
)

// stream is one open Connect stream: a one-snapshot and a one-pool-list
// mailbox (a newer value replaces an unsent one) and a close signal from the hub.
type stream struct {
	gw     Gateway
	serial string
	wake   chan struct{} // capacity 1
	done   chan struct{}

	mu      sync.Mutex
	pending *gatewayv1.Snapshot
	pool    *gatewayv1.PoolInstances
	gen     uint64 // newest build generation offered
	err     error  // why the hub closed the stream
}

// offer puts snap in the mailbox unless a newer build was already offered.
func (s *stream) offer(snap *gatewayv1.Snapshot, gen uint64) {
	s.mu.Lock()
	if gen < s.gen {
		s.mu.Unlock()
		return
	}
	s.gen, s.pending = gen, snap
	s.mu.Unlock()
	s.signal()
}

// offerPool puts the pool list in the mailbox; the Connect loop sends it.
func (s *stream) offerPool(p *gatewayv1.PoolInstances) {
	s.mu.Lock()
	s.pool = p
	s.mu.Unlock()
	s.signal()
}

func (s *stream) signal() {
	select {
	case s.wake <- struct{}{}:
	default:
	}
}

// take empties the mailbox.
func (s *stream) take() (*gatewayv1.Snapshot, *gatewayv1.PoolInstances) {
	s.mu.Lock()
	defer s.mu.Unlock()
	snap, pool := s.pending, s.pool
	s.pending, s.pool = nil, nil
	return snap, pool
}

// close ends the stream with err; the first call wins.
func (s *stream) close(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.err == nil {
		s.err = err
		close(s.done)
	}
}

func (s *stream) closeErr() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.err
}

// Connect sends the pool snapshot at once and after every change, the pool's
// online instances after the first snapshot and on every Run tick, and takes
// heartbeats. It ends with CodeUnavailable on max age ("rebalance"), silence
// ("heartbeat timeout") or hub shutdown ("shutdown").
func (hd *Handler) Connect(ctx context.Context, st *connect.BidiStream[gatewayv1.ConnectRequest, gatewayv1.ConnectResponse]) error {
	g, err := caller(ctx)
	if err != nil {
		return err
	}
	serial, _ := ctx.Value(serialKey).(string)
	s, err := hd.hub.register(g, serial)
	if err != nil {
		return err
	}
	defer hd.hub.unregister(s)
	hd.streams.Add(1)
	defer hd.streams.Add(-1)

	// Registered first: a change during this build gets a newer generation.
	gen := hd.hub.gen.Add(1)
	snap, err := hd.build(ctx, g.OrgID, g.ID)
	if err != nil {
		return hd.fail(ctx, "build snapshot", err)
	}
	s.offer(snap, gen)
	hd.offerPool(ctx, g.ID, []*stream{s})

	var lastBeat atomic.Int64
	lastBeat.Store(time.Now().UnixNano())
	received := make(chan error, 1)
	go func() { received <- hd.receive(ctx, st, s, &lastBeat) }()

	maxAge := time.Duration(float64(hd.hub.maxAge) * (1 + StreamMaxAgeJitter*(2*rand.Float64()-1))) //nolint:gosec // jitter, not security
	age := time.NewTimer(maxAge)
	defer age.Stop()
	beat := time.NewTimer(hd.hub.hbTimeout)
	defer beat.Stop()
	for {
		select {
		case <-s.wake:
			snap, pool := s.take()
			if snap != nil {
				if err := st.Send(&gatewayv1.ConnectResponse{Msg: &gatewayv1.ConnectResponse_Snapshot{Snapshot: snap}}); err != nil {
					return err
				}
				hd.pushes.Inc()
			}
			if pool != nil {
				if err := st.Send(&gatewayv1.ConnectResponse{Msg: &gatewayv1.ConnectResponse_PoolInstances{PoolInstances: pool}}); err != nil {
					return err
				}
			}
		case <-s.done:
			return s.closeErr()
		case <-age.C:
			return unavailable("rebalance")
		case <-beat.C:
			idle := time.Since(time.Unix(0, lastBeat.Load()))
			if idle >= hd.hub.hbTimeout {
				return unavailable("heartbeat timeout")
			}
			beat.Reset(hd.hub.hbTimeout - idle)
		case err := <-received:
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// receive handles heartbeats until the gateway closes its side.
func (hd *Handler) receive(ctx context.Context, st *connect.BidiStream[gatewayv1.ConnectRequest, gatewayv1.ConnectResponse], s *stream, lastBeat *atomic.Int64) error {
	for {
		msg, err := st.Receive()
		if err != nil {
			return err
		}
		if hb := msg.GetHeartbeat(); hb != nil {
			lastBeat.Store(time.Now().UnixNano())
			hd.heartbeat(ctx, s, hb)
		}
	}
}

// heartbeat marks the instance alive, ends its sessions the gateway no
// longer reports and stores the counters, denials and probe results it
// carries; a deleted instance loses its stream, a failed store only logs.
func (hd *Handler) heartbeat(ctx context.Context, s *stream, hb *gatewayv1.Heartbeat) {
	g := s.gw
	if err := hd.d.Gateways.Heartbeat(ctx, g.InstanceID, hb.GetVersion(), hb.GetConnectors()); err != nil {
		hd.log.Warn("tunnel: heartbeat", "instance_id", g.InstanceID, "error", err)
		if revoked(err) {
			s.close(authError(err))
			return
		}
	}
	active := make([]uuid.UUID, 0, len(hb.GetActiveSessionIds()))
	for _, v := range hb.GetActiveSessionIds() {
		id, err := uuid.Parse(v)
		if err != nil {
			hd.log.Warn("tunnel: heartbeat with a malformed session id", "instance_id", g.InstanceID)
			continue
		}
		active = append(active, id)
	}
	if _, err := hd.d.Sessions.EndMissing(ctx, g.OrgID, g.InstanceID, active, "gateway_lost"); err != nil {
		hd.log.Warn("tunnel: end missing sessions", "instance_id", g.InstanceID, "error", err)
	}
	if cs := heartbeatCounters(hb.GetCounters()); len(cs) > 0 {
		if err := hd.d.Sessions.UpdateCounters(ctx, g.OrgID, g.InstanceID, cs); err != nil {
			hd.log.Warn("tunnel: update session counters", "instance_id", g.InstanceID, "error", err)
		}
	}
	if len(hb.GetDenials()) > 0 {
		ds := make([]Denial, len(hb.GetDenials()))
		for i, d := range hb.GetDenials() {
			ds[i] = denialOf(d)
		}
		if err := hd.d.Sessions.Deny(ctx, g.OrgID, ds); err != nil {
			hd.log.Warn("tunnel: store denials", "instance_id", g.InstanceID, "error", err)
		}
	}
	if hs := heartbeatHealth(hb.GetHealth()); len(hs) > 0 {
		if err := hd.d.Health.ReportHealth(ctx, g.OrgID, g.InstanceID, hs); err != nil {
			hd.log.Warn("tunnel: store resource health", "instance_id", g.InstanceID, "error", err)
		}
	}
}

func heartbeatCounters(in []*gatewayv1.SessionCounters) []Counters {
	out := make([]Counters, 0, len(in))
	for _, c := range in {
		if id, err := uuid.Parse(c.GetSessionId()); err == nil {
			out = append(out, countersOf(id, c))
		}
	}
	return out
}

func countersOf(id uuid.UUID, c *gatewayv1.SessionCounters) Counters {
	return Counters{SessionID: id, BytesUp: c.GetBytesUp(), BytesDown: c.GetBytesDown(),
		DatagramsUp: c.GetDatagramsUp(), DatagramsDown: c.GetDatagramsDown()}
}

// denialOf converts d; an empty or invalid user id, client IP or time stays unset.
func denialOf(d *gatewayv1.Denial) Denial {
	out := Denial{KeyFingerprint: d.GetKeyFingerprint(), Resource: d.GetResource(), Kind: d.GetKind(),
		Target: d.GetTarget(), Reason: d.GetReason()}
	if id, err := uuid.Parse(d.GetUserId()); err == nil {
		out.UserID = &id
	}
	if ip, err := netip.ParseAddr(d.GetClientIp()); err == nil {
		out.ClientIP = ip.Unmap()
	}
	if d.GetAt().CheckValid() == nil {
		out.At = d.GetAt().AsTime()
	}
	return out
}

// heartbeatHealth converts the probe results; one without a valid time is dropped.
func heartbeatHealth(in []*gatewayv1.ResourceHealth) []ResourceHealth {
	out := make([]ResourceHealth, 0, len(in))
	for _, h := range in {
		id, err := uuid.Parse(h.GetResourceId())
		if err != nil || h.GetCheckedAt().CheckValid() != nil {
			continue
		}
		out = append(out, ResourceHealth{ResourceID: id, OK: h.GetOk(), Error: h.GetError(), CheckedAt: h.GetCheckedAt().AsTime()})
	}
	return out
}
