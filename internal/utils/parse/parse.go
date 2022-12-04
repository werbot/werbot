package parse

import (
	"net"
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
