package gateway

import (
	"errors"
	"net"
	"strconv"
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

// dialHost is the host a TargetDialer connector dials: the endpoint host, or the
// requested host when the endpoint allows any target (or has no host).
func dialHost(res Resource, requested string) string {
	ep, _, _ := res.Endpoint()
	if ep.AnyTarget || ep.Host == "" {
		return requested
	}
	return ep.Host
}
