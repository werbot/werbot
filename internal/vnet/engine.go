//go:build linux || darwin

package vnet

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"maps"
	"net/netip"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"golang.zx2c4.com/wireguard/tun"
	"gvisor.dev/gvisor/pkg/buffer"
	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/link/channel"
	"gvisor.dev/gvisor/pkg/tcpip/network/ipv4"
	"gvisor.dev/gvisor/pkg/tcpip/stack"
	"gvisor.dev/gvisor/pkg/tcpip/transport/tcp"
	"gvisor.dev/gvisor/pkg/tcpip/transport/udp"
)

// Dialer reaches a resource port through the gateway.
type Dialer interface {
	// DialTCP opens a direct-tcpip channel to the resource; host is its name.
	DialTCP(ctx context.Context, r Resource, port uint16) (io.ReadWriteCloser, error)
	// DialDatagram opens a datagram@werbot.com channel; it carries sshdgram frames.
	DialDatagram(ctx context.Context, r Resource, port uint16) (io.ReadWriteCloser, error)
	io.Closer
}

// Assignment is the address of a resource name for one protocol ("tcp" or "udp").
type Assignment struct {
	Name  string
	Addr  netip.Addr
	Proto string
}

const (
	nicID = 1
	// ponytail: MTU 1500 like the TUN; raise it (Teleport uses 16 KiB) if throughput falls short.
	mtu         = 1500
	tunOffset   = 16 // headroom for the virtio header of Linux TUN
	dialTimeout = 10 * time.Second
)

// engine is a userspace TCP/IP stack on a TUN device: packets from the
// interface address reach resources through the Dialer, DNS is answered
// at DNSAddr.
type engine struct {
	dev      tun.Device
	ep       *channel.Endpoint
	stack    *stack.Stack
	prefix   netip.Prefix
	iface    netip.Addr
	dnsAddr  netip.Addr
	resolver *resolver
	d        Dialer
	ctx      context.Context
	cancel   context.CancelFunc
	wg       sync.WaitGroup
	closeTUN sync.Once

	mu     sync.Mutex
	closed bool
	table  map[netip.Addr]Resource
	active map[io.Closer]netip.Addr // open connections and channels by resource address
	udp    map[udpKey]*udpChan
}

func newEngine(dev tun.Device, prefix netip.Prefix, suffix string, d Dialer) (*engine, error) {
	if _, err := base(prefix); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(context.Background())
	e := &engine{
		dev: dev, ep: channel.New(512, mtu, ""), prefix: prefix.Masked(),
		iface: IfaceAddr(prefix), dnsAddr: DNSAddr(prefix), resolver: newResolver(suffix), d: d,
		ctx: ctx, cancel: cancel,
		table: map[netip.Addr]Resource{}, active: map[io.Closer]netip.Addr{}, udp: map[udpKey]*udpChan{},
	}
	e.stack = stack.New(stack.Options{
		NetworkProtocols:   []stack.NetworkProtocolFactory{ipv4.NewProtocol},
		TransportProtocols: []stack.TransportProtocolFactory{tcp.NewProtocol, udp.NewProtocol},
	})
	if err := e.setupNIC(); err != nil {
		cancel()
		e.stack.Destroy()
		return nil, err
	}
	e.wg.Go(e.inbound)
	e.wg.Go(e.outbound)
	e.wg.Go(func() {
		for range dev.Events() { // the device stalls when nobody drains its events
		}
	})
	return e, nil
}

func (e *engine) setupNIC() error {
	if err := e.stack.CreateNIC(nicID, e.ep); err != nil {
		return fmt.Errorf("vnet nic: %s", err)
	}
	if err := e.stack.SetPromiscuousMode(nicID, true); err != nil {
		return fmt.Errorf("vnet promiscuous: %s", err)
	}
	if err := e.stack.SetSpoofing(nicID, true); err != nil {
		return fmt.Errorf("vnet spoofing: %s", err)
	}
	subnet := tcpip.AddressWithPrefix{Address: tcpip.AddrFrom4(e.prefix.Addr().As4()), PrefixLen: e.prefix.Bits()}.Subnet()
	e.stack.AddRoute(tcpip.Route{Destination: subnet, NIC: nicID})
	e.stack.SetTransportProtocolHandler(tcp.ProtocolNumber, tcp.NewForwarder(e.stack, 0, 1024, e.onTCP).HandlePacket)
	e.stack.SetTransportProtocolHandler(udp.ProtocolNumber, udp.NewForwarder(e.stack, e.onUDP).HandlePacket)
	return nil
}

// inbound moves packets from the TUN into the stack; only IPv4 packets from
// the interface address pass, so forwarded traffic of other hosts never does.
func (e *engine) inbound() {
	bufs := make([][]byte, e.dev.BatchSize())
	for i := range bufs {
		bufs[i] = make([]byte, tunOffset+65535)
	}
	sizes := make([]int, len(bufs))
	var dropped int
	var logged time.Time
	for {
		n, err := e.dev.Read(bufs, sizes, tunOffset)
		for i := range n {
			p := bufs[i][tunOffset : tunOffset+sizes[i]]
			if !e.fromIface(p) {
				if dropped++; time.Since(logged) >= time.Minute {
					slog.Debug("vnet dropped packets from foreign sources", "count", dropped)
					dropped, logged = 0, time.Now()
				}
				continue
			}
			pkt := stack.NewPacketBuffer(stack.PacketBufferOptions{Payload: buffer.MakeWithData(p)})
			e.ep.InjectInbound(ipv4.ProtocolNumber, pkt)
			pkt.DecRef()
		}
		if errors.Is(err, tun.ErrTooManySegments) {
			continue
		}
		if err != nil {
			if e.ctx.Err() == nil {
				slog.Warn("vnet tun read", "err", err)
			}
			return
		}
	}
}

func (e *engine) fromIface(p []byte) bool {
	return len(p) >= 20 && p[0]>>4 == 4 && netip.AddrFrom4([4]byte(p[12:16])) == e.iface
}

// outbound writes the stack's packets to the TUN with tunOffset bytes of headroom.
func (e *engine) outbound() {
	buf := make([]byte, tunOffset+65535)
	for {
		pkt := e.ep.ReadContext(e.ctx)
		if pkt == nil {
			return
		}
		n := tunOffset
		for _, s := range pkt.AsSlices() {
			n += copy(buf[n:], s)
		}
		pkt.DecRef()
		_, _ = e.dev.Write([][]byte{buf[:n]}, tunOffset)
	}
}

// SetResources maps the resources with a stream or datagram kind to their
// addresses and names. Removed resources lose their name, new connections to
// their addresses are reset and open ones are closed.
func (e *engine) SetResources(rs []Resource) ([]Assignment, error) {
	rs = slices.DeleteFunc(slices.Clone(rs), func(r Resource) bool { return protos(r) == nil })
	ids := make([]uuid.UUID, len(rs))
	for i, r := range rs {
		ids[i] = r.ID
	}
	addrs, err := Allocate(e.prefix, ids)
	if err != nil {
		return nil, err
	}
	table, names := map[netip.Addr]Resource{}, map[string]netip.Addr{}
	var out []Assignment
	for _, r := range rs {
		a := addrs[r.ID]
		table[a] = r
		names[strings.ToLower(r.Name)+"."+e.resolver.suffix] = a
		for _, p := range protos(r) {
			out = append(out, Assignment{Name: r.Name, Addr: a, Proto: p})
		}
	}
	e.resolver.set(names)
	e.mu.Lock()
	var gone []io.Closer
	for c, a := range e.active {
		if table[a].ID != e.table[a].ID {
			gone = append(gone, c)
			delete(e.active, c)
		}
	}
	e.table = table
	e.mu.Unlock()
	for _, c := range gone {
		_ = c.Close()
	}
	return out, nil
}

func protos(r Resource) []string {
	var out []string
	if slices.Contains(r.Kinds, "stream") {
		out = append(out, "tcp")
	}
	if slices.Contains(r.Kinds, "datagram") {
		out = append(out, "udp")
	}
	return out
}

// stop ends the forwarders: it closes the open connections and channels and
// the Dialer, so nothing dials or renews the certificate after it. The TUN
// stays open.
func (e *engine) stop() error {
	e.mu.Lock()
	if e.closed {
		e.mu.Unlock()
		return nil
	}
	e.closed = true
	gone := slices.Collect(maps.Keys(e.active))
	clear(e.active)
	e.mu.Unlock()
	e.cancel()
	for _, c := range gone {
		_ = c.Close()
	}
	return e.d.Close()
}

// Close stops the engine and closes the TUN device.
func (e *engine) Close() error {
	err := e.stop()
	e.closeTUN.Do(func() {
		err = errors.Join(err, e.dev.Close())
		e.ep.Close()
		e.stack.Close()
		e.stack.Wait()
		e.wg.Wait()
	})
	return err
}
