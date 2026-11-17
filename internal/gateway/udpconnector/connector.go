// Package udpconnector is the gateway connector of the "udp" resource type:
// datagrams of the SSH front's datagram@werbot.com channel, one UDP socket per
// client flow to a target port of the resource. Datagrams are counted, not recorded.
package udpconnector

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"google.golang.org/protobuf/encoding/protojson"

	udpv1 "github.com/werbot/werbot/gen/werbot/connector/udp/v1"
	"github.com/werbot/werbot/internal/connector/udp"
	"github.com/werbot/werbot/internal/gateway"
	"github.com/werbot/werbot/internal/platform/metrics"
)

// ponytail: fixed limits, raise them into the resource config if a user needs more.
const (
	maxFlows    = 32   // concurrent flows (target sockets) of one session
	maxDatagram = 8192 // largest datagram relayed; a larger one is dropped
)

const droppedName, droppedHelp = "werbot_gateway_datagrams_dropped_total", "Datagrams dropped by the udp connector, by reason."

var bufPool = sync.Pool{New: func() any { b := make([]byte, maxDatagram); return &b }}

func init() { gateway.RegisterConnector(Connector{}) }

// Connector serves "udp" sessions.
type Connector struct{}

// Type is "udp".
func (Connector) Type() string { return "udp" }

// Kinds is datagram only.
func (Connector) Kinds() []gateway.Kind { return []gateway.Kind{gateway.KindDatagram} }

// DialsTarget marks the connector as dialing through Session.Dial.
func (Connector) DialsTarget() {}

// flow is the target socket of one client flow; last is the time of its latest datagram, either way.
type flow struct {
	conn net.Conn
	last atomic.Int64 // UnixNano
}

func (f *flow) touch() { f.last.Store(time.Now().UnixNano()) }

type session struct {
	s                  *gateway.Session
	ctx                context.Context
	g                  *gateway.Guard
	addr               string
	idle               time.Duration
	dropFlows, dropBig *metrics.Counter

	mu    sync.Mutex
	flows map[uint32]*flow
}

// Serve relays the datagrams of the client to the resource host at the port of
// s.Target, one socket per flow, until the client ends or ctx is canceled (not
// an error). The Recv loop never sends to the client: each flow's reader does,
// so a client that stops reading blocks only its flow readers, which Serve
// does not wait for. A panic ends the session and re-panics here for the pipeline.
func (Connector) Serve(ctx context.Context, s *gateway.Session) error {
	sctx, stop := context.WithCancel(ctx)
	defer stop()
	u, err := newSession(sctx, s)
	if err != nil {
		return err
	}
	u.g = gateway.NewGuard(stop, s.Secret)
	defer context.AfterFunc(sctx, u.closeAll)()
	u.g.Go(u.recv)
	err = u.g.Wait()
	u.closeAll()
	if ctx.Err() != nil {
		return nil
	}
	return err
}

func newSession(ctx context.Context, s *gateway.Session) (*session, error) {
	_, port, err := net.SplitHostPort(s.Target)
	if err != nil {
		return nil, gateway.ErrTargetNotAllowed
	}
	cfg := &udpv1.Config{}
	if err := protojson.Unmarshal(s.Resource.Config, cfg); err != nil || udp.Validate(cfg) != nil {
		return nil, errors.New("udp: resource config")
	}
	reg := s.Metrics
	if reg == nil {
		reg = metrics.NewRegistry()
	}
	return &session{
		s: s, ctx: ctx, addr: net.JoinHostPort(cfg.Host, port), idle: cfg.FlowIdle.AsDuration(),
		dropFlows: reg.Counter(droppedName, droppedHelp, "reason", "flows"),
		dropBig:   reg.Counter(droppedName, droppedHelp, "reason", "size"),
		flows:     map[uint32]*flow{},
	}, nil
}

// closeAll ends the client side and every flow socket; it unblocks recv and the readers.
func (u *session) closeAll() {
	_ = u.s.Datagram.Close()
	u.mu.Lock()
	defer u.mu.Unlock()
	for id, f := range u.flows {
		_ = f.conn.Close()
		delete(u.flows, id)
	}
}

func (u *session) recv() error {
	bp := bufPool.Get().(*[]byte)
	defer bufPool.Put(bp)
	for {
		id, n, err := u.s.Datagram.Recv(*bp)
		switch {
		case errors.Is(err, io.ErrShortBuffer):
			u.dropBig.Inc()
			continue
		case err == nil:
		case u.ctx.Err() != nil, errors.Is(err, io.EOF), errors.Is(err, net.ErrClosed), errors.Is(err, io.ErrClosedPipe):
			return nil
		default:
			return fmt.Errorf("udp: client: %w", err)
		}
		f, err := u.flow(id)
		if err != nil {
			return err
		}
		if f == nil {
			u.dropFlows.Inc()
			continue
		}
		f.touch()
		// ECONNREFUSED (an earlier datagram found the port closed) and any other write error drop this datagram only.
		_, _ = f.conn.Write((*bp)[:n])
	}
}

// flow returns the flow id, dialing it on first use; nil means the flow limit is reached.
func (u *session) flow(id uint32) (*flow, error) {
	u.mu.Lock()
	defer u.mu.Unlock()
	if f, ok := u.flows[id]; ok {
		return f, nil
	}
	if len(u.flows) >= maxFlows {
		return nil, nil
	}
	conn, err := u.s.Dial(u.ctx, "udp", u.addr)
	if err != nil {
		if errors.Is(err, gateway.ErrTargetNotAllowed) {
			return nil, fmt.Errorf("udp: dial target: %w", gateway.ErrTargetNotAllowed)
		}
		if oe, ok := errors.AsType[*net.OpError](err); ok {
			err = oe.Err
		}
		return nil, fmt.Errorf("udp: dial target: %w", err)
	}
	f := &flow{conn: conn}
	f.touch()
	u.flows[id] = f
	u.g.Detach(func() error { return u.read(id, f) })
	return f, nil
}

// remove closes the socket of f and forgets the flow, so its next datagram opens a new one.
func (u *session) remove(id uint32, f *flow) {
	u.mu.Lock()
	if u.flows[id] == f {
		delete(u.flows, id)
	}
	u.mu.Unlock()
	_ = f.conn.Close()
}

// read sends what the target answers to the client until the flow is idle for
// flow_idle or its socket closes. Detached: Send may block on a client that does not read.
func (u *session) read(id uint32, f *flow) error {
	bp := bufPool.Get().(*[]byte)
	defer bufPool.Put(bp)
	defer u.remove(id, f)
	for {
		_ = f.conn.SetReadDeadline(time.Unix(0, f.last.Load()).Add(u.idle))
		n, err := f.conn.Read(*bp)
		switch {
		case err == nil:
			f.touch()
			if err := u.s.Datagram.Send(id, (*bp)[:n]); err != nil {
				if u.ctx.Err() != nil || errors.Is(err, net.ErrClosed) || errors.Is(err, io.ErrClosedPipe) || errors.Is(err, io.EOF) {
					return nil
				}
				return fmt.Errorf("udp: client: %w", err)
			}
		case errors.Is(err, syscall.ECONNREFUSED):
		case errors.Is(err, os.ErrDeadlineExceeded):
			if time.Since(time.Unix(0, f.last.Load())) >= u.idle {
				return nil
			}
		default:
			return nil // socket closed: the session ended or the flow was removed
		}
	}
}
