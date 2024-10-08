package netutil

import (
	"fmt"
	"net"
	"net/netip"
)

// IP is parse ip to string
func IP(addr string) string {
	ips, err := net.LookupIP(addr)
	if err != nil || len(ips) == 0 {
		return ""
	}
	return ips[0].String()
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
	return netip.IPv4Unspecified().String()
}

// InternalIPv6 get internal IPv6
func InternalIPv6() string {
	return netip.IPv6Unspecified().String()
}

// FreePort returns a free port.
func FreePort() (port int, err error) {
	addr, err := net.ResolveTCPAddr("tcp", "localhost:0")
	if err == nil {
		var l *net.TCPListener
		if l, err = net.ListenTCP("tcp", addr); err == nil {
			defer l.Close()
			return l.Addr().(*net.TCPAddr).Port, nil
		}
	}
	return
}

// IPWithMask is ...
func IPWithMask(ip string) (string, error) {
	_, ipNet, err := net.ParseCIDR(ip)
	if err == nil {
		return ipNet.String(), nil
	}

	ipAddr := net.ParseIP(ip)
	if ipAddr == nil {
		return "", fmt.Errorf("invalid IP address: %s", ip)
	}

	var defaultMask net.IPMask
	if ipAddr.To4() != nil {
		defaultMask = net.CIDRMask(32, 32)
	} else {
		defaultMask = net.CIDRMask(128, 128)
	}

	ipNet = &net.IPNet{
		IP:   ipAddr,
		Mask: defaultMask,
	}
	return ipNet.String(), nil
}

// IsReservedIP is ...
func IsReservedIP(ip string) bool {
	reservedNetworks := []string{
		"0.0.0.0/8",
		"10.0.0.0/8",
		"100.64.0.0/10",
		"127.0.0.0/8",
		"169.254.0.0/16",
		"172.16.0.0/12",
		"192.0.0.0/29",
		"192.0.2.0/24",
		"192.88.99.0/24",
		"192.168.0.0/16",
		"198.18.0.0/15",
		"198.51.100.0/24",
		"203.0.113.0/24",
		"224.0.0.0/4",
		"240.0.0.0/4",
		"100::/64",
		"2001:1::/32",
		"2001:2::/31",
		"2001:4::/30",
		"2001:8::/29",
		"2001:10::/28",
		"2001:20::/27",
		"2001:40::/26",
		"2001:80::/25",
		"2001:100::/24",
		"2001:db8::/32",
		"fc00::/7",
		"fe80::/10",
		"ff00::/8",
	}

	for _, network := range reservedNetworks {
		_, reservedNet, err := net.ParseCIDR(network)
		if err != nil {
			return false
		}

		_, inputNet, err := net.ParseCIDR(ip)
		if err == nil {
			if reservedNet.Contains(inputNet.IP) || inputNet.Contains(reservedNet.IP) {
				return true
			}
			continue
		}

		parsedIP := net.ParseIP(ip)
		if parsedIP == nil {
			return false
		}

		if reservedNet.Contains(parsedIP) {
			return true
		}
	}

	return false
}
