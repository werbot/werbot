package netutil

import (
	"net"
	"net/netip"
)

// IP is parse ip to string
func IP(addr string) string {
	ips, err := net.LookupIP(addr)
	if err == nil && len(ips) > 0 {
		return ips[0].String()
	}
	return ""
}

// InternalIP get internal IP
func InternalIP() string {
	if addr := netip.IPv4Unspecified().String(); addr != "" {
		return addr
	}

	return netip.IPv6Unspecified().String()
}

// InternalIPv4 get internal IPv4
func InternalIPv4() string {
	addr := netip.IPv4Unspecified()

	if addr.IsValid() {
		return addr.String()
	}
	return ""
}

// InternalIPv6 get internal IPv6
func InternalIPv6() string {
	addr := netip.IPv6Unspecified()

	if addr.IsValid() {
		return addr.String()
	}
	return ""
}
