//go:build linux || darwin

package vnet

import (
	"context"
	"io"
	"net/netip"
	"slices"
	"time"

	"gvisor.dev/gvisor/pkg/tcpip/adapters/gonet"
	"gvisor.dev/gvisor/pkg/tcpip/transport/tcp"
	"gvisor.dev/gvisor/pkg/tcpip/transport/udp"
	"gvisor.dev/gvisor/pkg/waiter"
)

const dnsPort = 53

// enter counts a forwarding goroutine in e.wg; false once the engine is closed.
func (e *engine) enter() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.closed {
		return false
	}
	e.wg.Add(1)
	return true
}

// lookup is the resource at a with kind, if any.
func (e *engine) lookup(a netip.Addr, kind string) (Resource, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	r, ok := e.table[a]
	return r, ok && slices.Contains(r.Kinds, kind)
}

// hold registers cs as open for the resource at a; false (nothing
// registered) when a no longer maps to that resource.
func (e *engine) hold(a netip.Addr, r Resource, cs ...io.Closer) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.closed || e.table[a].ID != r.ID {
		return false
	}
	for _, c := range cs {
		e.active[c] = a
	}
	return true
}

func (e *engine) release(cs ...io.Closer) {
	e.mu.Lock()
	defer e.mu.Unlock()
	for _, c := range cs {
		delete(e.active, c)
	}
}

// onTCP serves a new connection (its own goroutine): DNS at DNSAddr:53, a
// stream resource through the Dialer, anything else gets a reset.
func (e *engine) onTCP(req *tcp.ForwarderRequest) {
	if !e.enter() {
		req.Complete(true)
		return
	}
	defer e.wg.Done()
	id := req.ID()
	dst := netip.AddrFrom4(id.LocalAddress.As4())
	if dst == e.dnsAddr && id.LocalPort == dnsPort {
		if c := accept(req); c != nil {
			serveDNSTCP(c, e.resolver)
		}
		return
	}
	r, ok := e.lookup(dst, "stream")
	if !ok {
		req.Complete(true)
		return
	}
	ctx, cancel := context.WithTimeout(e.ctx, dialTimeout)
	up, err := e.d.DialTCP(ctx, r, id.LocalPort)
	cancel()
	if err != nil {
		req.Complete(true)
		return
	}
	c := accept(req)
	if c == nil || !e.hold(dst, r, c, up) {
		_ = up.Close()
		if c != nil {
			_ = c.Close()
		}
		return
	}
	defer e.release(c, up)
	pipe(c, up)
}

// accept completes the handshake; nil after a reset.
func accept(req *tcp.ForwarderRequest) *gonet.TCPConn {
	var wq waiter.Queue
	ep, err := req.CreateEndpoint(&wq)
	if err != nil {
		req.Complete(true)
		return nil
	}
	req.Complete(false)
	return gonet.NewTCPConn(&wq, ep)
}

// pipe copies both ways, passing each EOF on as a half-close, and closes
// both once both directions ended.
func pipe(a, b io.ReadWriteCloser) {
	done := make(chan struct{})
	go func() {
		_, _ = io.Copy(b, a)
		closeWrite(b)
		close(done)
	}()
	_, _ = io.Copy(a, b)
	closeWrite(a)
	<-done
	_ = a.Close()
	_ = b.Close()
}

func closeWrite(c io.Closer) {
	if cw, ok := c.(interface{ CloseWrite() error }); ok {
		_ = cw.CloseWrite()
	} else {
		_ = c.Close()
	}
}

// onUDP handles the first datagram of a flow (on the packet path, so it
// must not block): DNS at DNSAddr:53, a datagram resource through its
// channel, anything else is dropped.
func (e *engine) onUDP(req *udp.ForwarderRequest) {
	id := req.ID()
	dst := netip.AddrFrom4(id.LocalAddress.As4())
	if dst == e.dnsAddr && id.LocalPort == dnsPort {
		if c := udpConn(req); c != nil && !e.goTracked(func() { serveDNSUDP(c, e.resolver) }) {
			_ = c.Close()
		}
		return
	}
	e.addFlow(req, udpKey{dst, id.LocalPort})
}

func udpConn(req *udp.ForwarderRequest) *gonet.UDPConn {
	var wq waiter.Queue
	ep, err := req.CreateEndpoint(&wq)
	if err != nil {
		return nil
	}
	return gonet.NewUDPConn(&wq, ep)
}

// goTracked runs fn in a goroutine counted in e.wg; false once closed.
func (e *engine) goTracked(fn func()) bool {
	if !e.enter() {
		return false
	}
	go func() {
		defer e.wg.Done()
		fn()
	}()
	return true
}

// serveDNSUDP answers the queries of one client socket until it stays idle.
func serveDNSUDP(c *gonet.UDPConn, r *resolver) {
	defer c.Close()
	buf := make([]byte, 65535)
	for {
		_ = c.SetReadDeadline(time.Now().Add(tcpIdle))
		n, err := c.Read(buf)
		if err != nil {
			return
		}
		if resp, ok := r.answer(buf[:n]); ok {
			_, _ = c.Write(resp)
		}
	}
}
