package connectcli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/werbot/werbot/internal/platform/sshdgram"
)

const maxFlows = 32 // client flows per datagram channel, as the gateway allows

// flowIdle is how long a flow without traffic keeps its slot in the table.
var flowIdle = 2 * time.Minute

// udpRelay carries the datagrams of a local UDP port over one
// datagram@werbot.com channel. Each client address is a flow with its own id;
// replies of a flow go back to its address. The channel opens on the first
// datagram and again after it closed (a new session with an empty table).
type udpRelay struct {
	b *binding

	mu    sync.Mutex
	ch    ssh.Channel
	flows map[string]*flow // by client address
	byID  map[uint32]*flow
	next  uint32

	// after a failed open, datagrams are dropped until retryAt; the wait
	// doubles from backoffFirst to backoffMax and a text is printed once
	wait     time.Duration
	retryAt  time.Time
	lastText string
}

type flow struct {
	id   uint32
	addr net.Addr
	seen time.Time // last datagram either way
}

func newUDPRelay(b *binding) *udpRelay {
	return &udpRelay{b: b, flows: map[string]*flow{}, byID: map[uint32]*flow{}}
}

// serve reads local datagrams until the socket closes. A datagram of a new
// address when the table is full is dropped, as the gateway drops it.
func (r *udpRelay) serve(ctx context.Context) {
	var wg sync.WaitGroup
	defer wg.Wait()
	buf := make([]byte, sshdgram.MaxPayload)
	for {
		n, addr, err := r.b.pc.ReadFrom(buf)
		if errors.Is(err, net.ErrClosed) {
			break
		}
		if err != nil { // Windows reports an ICMP unreachable of an earlier reply here
			continue
		}
		ch := r.channel(ctx, &wg, addr)
		if ch == nil {
			continue
		}
		id, ok := r.flowFor(addr, time.Now())
		if !ok {
			continue
		}
		if err := sshdgram.WriteFrame(ch, id, buf[:n]); err != nil {
			r.drop(ch)
		}
	}
	r.mu.Lock()
	ch := r.ch
	r.mu.Unlock()
	if ch != nil {
		r.drop(ch)
	}
}

// channel is the open channel, or a new one opened through the link; nil
// while a failed open backs off. Only serve calls it.
func (r *udpRelay) channel(ctx context.Context, wg *sync.WaitGroup, from net.Addr) ssh.Channel {
	r.mu.Lock()
	ch := r.ch
	r.mu.Unlock()
	if ch != nil || time.Now().Before(r.retryAt) {
		return ch
	}
	ch, reqs, err := r.b.openChannel(ctx, sshdgram.ChannelType, from)
	if err != nil {
		if ctx.Err() == nil {
			r.wait = min(max(2*r.wait, backoffFirst), backoffMax)
			r.retryAt = time.Now().Add(r.wait)
			if text := r.b.openText(err); text != r.lastText {
				r.lastText = text
				_, _ = fmt.Fprintln(r.b.stderr, text)
			}
		}
		return nil
	}
	r.wait, r.lastText = 0, ""
	r.mu.Lock()
	r.ch = ch
	r.mu.Unlock()
	wg.Go(func() { ssh.DiscardRequests(reqs) })
	wg.Go(func() { _, _ = io.Copy(r.b.stderr, ch.Stderr()) })
	wg.Go(func() { r.read(ch) })
	return ch
}

// read sends the replies of ch to their flows' addresses until ch ends.
func (r *udpRelay) read(ch ssh.Channel) {
	defer r.drop(ch)
	buf := make([]byte, sshdgram.MaxPayload)
	for {
		id, n, err := sshdgram.ReadFrame(ch, buf)
		if err != nil && !errors.Is(err, io.ErrShortBuffer) {
			return
		}
		r.mu.Lock()
		f := r.byID[id]
		if f != nil {
			f.seen = time.Now()
		}
		r.mu.Unlock()
		if f != nil && err == nil {
			_, _ = r.b.pc.WriteTo(buf[:n], f.addr)
		}
	}
}

// drop closes ch and, if it is the current channel, empties the flow table.
func (r *udpRelay) drop(ch ssh.Channel) {
	r.mu.Lock()
	if r.ch == ch {
		r.ch = nil
		clear(r.flows)
		clear(r.byID)
	}
	r.mu.Unlock()
	_ = ch.Close()
}

// flowFor is the flow id of addr, new if needed. A full table first frees the
// flows idle for flowIdle; still full, ok is false.
func (r *udpRelay) flowFor(addr net.Addr, now time.Time) (id uint32, ok bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if f := r.flows[addr.String()]; f != nil {
		f.seen = now
		return f.id, true
	}
	if len(r.flows) >= maxFlows {
		for k, f := range r.flows {
			if now.Sub(f.seen) >= flowIdle {
				delete(r.flows, k)
				delete(r.byID, f.id)
			}
		}
		if len(r.flows) >= maxFlows {
			return 0, false
		}
	}
	r.next++
	f := &flow{id: r.next, addr: addr, seen: now}
	r.flows[addr.String()] = f
	r.byID[f.id] = f
	return f.id, true
}
