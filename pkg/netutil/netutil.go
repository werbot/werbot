package netutil

import (
	"net"
	"net/netip"
)

// IP is parse ip to string
func IP(addr string) string {
	ip, _, err := net.SplitHostPort(addr)
	if err == nil {
		return ip
	}

	ip2 := net.ParseIP(addr)
	if ip2 == nil {
		return ""
	}

	return ip2.String()
}

// InternalIP get internal IP
func InternalIP() string {
	addr := netip.IPv4Unspecified()
	if addr.IsValid() {
		return addr.String()
	}

	addr = netip.IPv6Unspecified()
	if addr.IsValid() {
		return addr.String()
	}

	return ""
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
