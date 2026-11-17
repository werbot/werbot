package gateway

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"strconv"
	"syscall"
	"time"
)

// ErrTargetNotAllowed is a stream or datagram target outside the resource's
// endpoint, or a local address refused by Session.Dial.
var ErrTargetNotAllowed = errors.New("target is not allowed")

// allowTarget checks a stream or datagram target against the resource's endpoint.
// A target without a host or with port 0 is refused: port 0 would pass every
// port-bound access binding.
func allowTarget(res Resource, target string) (string, uint16, error) {
	host, port, err := net.SplitHostPort(target)
	if err != nil || host == "" {
		return "", 0, ErrTargetNotAllowed
	}
	n, err := strconv.ParseUint(port, 10, 16)
	if err != nil || n == 0 {
		return "", 0, ErrTargetNotAllowed
	}
	ep, ok, err := res.Endpoint()
	if err != nil || !ok || !ep.Allow(res.Name, host, uint16(n)) {
		return "", 0, ErrTargetNotAllowed
	}
	return host, uint16(n), nil
}

// newDialer is Session.Dial: denyLocal refuses loopback, link-local and unspecified
// targets. The check runs on the resolved address of every attempt, so a name that
// resolves (or rebinds) to such an address is refused too.
func newDialer(deny bool) func(ctx context.Context, network, address string) (net.Conn, error) {
	d := &net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}
	if deny {
		d.Control = denyLocal
	}
	return d.DialContext
}

// denyLocal is a net.Dialer Control that refuses local addresses with ErrTargetNotAllowed.
func denyLocal(_, address string, _ syscall.RawConn) error {
	ap, err := netip.ParseAddrPort(address)
	if err != nil {
		return fmt.Errorf("%w: %s", ErrTargetNotAllowed, address)
	}
	ip := ap.Addr().Unmap()
	if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified() {
		return fmt.Errorf("%w: %s is a local address", ErrTargetNotAllowed, ip.WithZone(""))
	}
	return nil
}
