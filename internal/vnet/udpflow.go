//go:build linux || darwin

package vnet

import (
	"context"
	"errors"
	"io"
	"maps"
	"net"
	"net/netip"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"gvisor.dev/gvisor/pkg/tcpip/adapters/gonet"
	"gvisor.dev/gvisor/pkg/tcpip/transport/udp"

	"github.com/werbot/werbot/internal/platform/sshdgram"
)

const maxFlows = 32 // flows per datagram channel, as the gateway allows

// flowIdle is how long a flow lives without a datagram either way.
var flowIdle = 2 * time.Minute

type udpKey struct {
	addr netip.Addr
	port uint16
}

// udpChan is the datagram channel of one resource port. Each client socket
// is a flow with its own id; the channel opens with its first flow and
// closes with its last.
type udpChan struct {
	e     *engine
	key   udpKey
	ready chan struct{} // closed when the dial ended; rw stays nil after a failure
	wmu   sync.Mutex    // a frame is two writes

	// guarded by e.mu
	rw     io.ReadWriteCloser
	flows  map[uint32]*udpFlow
	next   uint32
	closed bool
}

type udpFlow struct {
	id   uint32
	c    *gonet.UDPConn
	seen atomic.Int64 // unix nanoseconds of the last datagram either way
}

func (f *udpFlow) touch()            { f.seen.Store(time.Now().UnixNano()) }
func (f *udpFlow) idleAt() time.Time { return time.Unix(0, f.seen.Load()).Add(flowIdle) }

// addFlow makes the first datagram of a client socket a flow of the channel
// of its resource port; over maxFlows it is dropped.
func (e *engine) addFlow(req *udp.ForwarderRequest, k udpKey) {
	e.mu.Lock()
	defer e.mu.Unlock()
	r, ok := e.table[k.addr]
	ch := e.udp[k]
	if e.closed || !ok || !slices.Contains(r.Kinds, "datagram") || (ch != nil && len(ch.flows) >= maxFlows) {
		return
	}
	c := udpConn(req)
	if c == nil {
		return
	}
	if ch == nil {
		ch = &udpChan{e: e, key: k, ready: make(chan struct{}), flows: map[uint32]*udpFlow{}}
		e.udp[k], e.active[ch] = ch, k.addr
		e.wg.Add(1)
		go func() { defer e.wg.Done(); ch.run(r) }()
	}
	ch.next++
	f := &udpFlow{id: ch.next, c: c}
	f.touch()
	ch.flows[f.id] = f
	e.wg.Add(1)
	go func() { defer e.wg.Done(); ch.serve(f) }()
}

// run dials the channel and sends its frames to their flows until it ends.
func (ch *udpChan) run(r Resource) {
	defer ch.Close()
	rw := ch.dial(r)
	if rw == nil {
		return
	}
	buf := make([]byte, sshdgram.MaxPayload)
	for {
		id, n, err := sshdgram.ReadFrame(rw, buf)
		if err != nil && !errors.Is(err, io.ErrShortBuffer) {
			return
		}
		ch.e.mu.Lock()
		f := ch.flows[id]
		ch.e.mu.Unlock()
		if f != nil && err == nil {
			f.touch()
			_, _ = f.c.Write(buf[:n])
		}
	}
}

// dial opens the channel for the waiting flows; nil after a failure or when
// the channel was closed meanwhile.
func (ch *udpChan) dial(r Resource) io.ReadWriteCloser {
	defer close(ch.ready)
	ctx, cancel := context.WithTimeout(ch.e.ctx, dialTimeout)
	rw, err := ch.e.d.DialDatagram(ctx, r, ch.key.port)
	cancel()
	if err != nil {
		return nil
	}
	ch.e.mu.Lock()
	closed := ch.closed
	if !closed {
		ch.rw = rw
	}
	ch.e.mu.Unlock()
	if closed {
		_ = rw.Close()
		return nil
	}
	return rw
}

// serve sends the datagrams of flow f into the channel until f idles out.
func (ch *udpChan) serve(f *udpFlow) {
	defer ch.remove(f)
	<-ch.ready
	ch.e.mu.Lock()
	rw := ch.rw
	ch.e.mu.Unlock()
	if rw == nil {
		return
	}
	buf := make([]byte, sshdgram.MaxPayload)
	for {
		_ = f.c.SetReadDeadline(f.idleAt())
		n, err := f.c.Read(buf)
		if ne, ok := errors.AsType[net.Error](err); ok && ne.Timeout() && time.Now().Before(f.idleAt()) {
			continue // a reply came meanwhile
		}
		if err != nil {
			return
		}
		f.touch()
		ch.wmu.Lock()
		err = sshdgram.WriteFrame(rw, f.id, buf[:n])
		ch.wmu.Unlock()
		if err != nil {
			return
		}
	}
}

// remove ends flow f; the last flow closes the channel.
func (ch *udpChan) remove(f *udpFlow) {
	_ = f.c.Close()
	ch.e.mu.Lock()
	delete(ch.flows, f.id)
	var rw io.Closer
	var flows []*udpFlow
	if len(ch.flows) == 0 {
		rw, flows = ch.detach()
	}
	ch.e.mu.Unlock()
	closeAll(rw, flows)
}

// Close ends the channel and all its flows.
func (ch *udpChan) Close() error {
	ch.e.mu.Lock()
	rw, flows := ch.detach()
	ch.e.mu.Unlock()
	closeAll(rw, flows)
	return nil
}

// detach takes the channel out of the engine; e.mu is held.
func (ch *udpChan) detach() (io.Closer, []*udpFlow) {
	if ch.closed {
		return nil, nil
	}
	ch.closed = true
	if ch.e.udp[ch.key] == ch {
		delete(ch.e.udp, ch.key)
	}
	delete(ch.e.active, ch)
	return ch.rw, slices.Collect(maps.Values(ch.flows))
}

func closeAll(rw io.Closer, flows []*udpFlow) {
	if rw != nil {
		_ = rw.Close()
	}
	for _, f := range flows {
		_ = f.c.Close()
	}
}
