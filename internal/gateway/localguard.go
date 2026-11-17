package gateway

import (
	"context"
	"fmt"
	"net"
	"net/netip"
	"slices"
	"syscall"
	"time"
)

// localGuard keeps the embedded gateway (Config.DenyLocalTargets) off the server's
// own host: loopback, link-local (cloud metadata), multicast and unspecified
// addresses, and whatever the server's database hosts resolve to. Other addresses
// of the host and its LAN stay reachable; restrict them with endpoints and bindings.
type localGuard struct {
	deny  bool
	hosts []string // Config.DenyHosts, resolved on every check
}

// dialer is Session.Dial: with deny, every attempt checks its resolved address, so
// a name that resolves (or rebinds) to a refused address is refused too.
func (g localGuard) dialer() func(ctx context.Context, network, address string) (net.Conn, error) {
	d := &net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}
	if g.deny {
		d.ControlContext = g.control
	}
	return d.DialContext
}

// control is a net.Dialer ControlContext that refuses with ErrTargetNotAllowed.
func (g localGuard) control(ctx context.Context, _, address string, _ syscall.RawConn) error {
	ap, err := netip.ParseAddrPort(address)
	if err != nil {
		return fmt.Errorf("%w: %s", ErrTargetNotAllowed, address)
	}
	return g.allowIPs(ctx, ap.Addr())
}

// allowHost is the check of Open, before StartSession: every address host resolves
// to must pass. A host that does not resolve is left to the dial.
func (g localGuard) allowHost(ctx context.Context, host string) error {
	if !g.deny {
		return nil
	}
	ips, err := net.DefaultResolver.LookupNetIP(ctx, "ip", host)
	if err != nil {
		return nil //nolint:nilerr // the dial fails the same way and its check still runs
	}
	return g.allowIPs(ctx, ips...)
}

// allowIPs refuses any of ips that is local or an address of the database hosts,
// which it resolves once.
func (g localGuard) allowIPs(ctx context.Context, ips ...netip.Addr) error {
	var own []netip.Addr
	for _, h := range g.hosts {
		as, err := net.DefaultResolver.LookupNetIP(ctx, "ip", h)
		if err != nil { // fail closed: the target could be the database
			return fmt.Errorf("%w: resolve the server database host %s: %w", ErrTargetNotAllowed, h, err)
		}
		for _, a := range as {
			own = append(own, a.Unmap().WithZone(""))
		}
	}
	for _, ip := range ips {
		ip = ip.Unmap().WithZone("")
		if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsMulticast() || ip.IsUnspecified() {
			return fmt.Errorf("%w: %s is a local address", ErrTargetNotAllowed, ip)
		}
		if slices.Contains(own, ip) {
			return fmt.Errorf("%w: %s is the server database host", ErrTargetNotAllowed, ip)
		}
	}
	return nil
}
