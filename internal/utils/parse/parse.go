package parse

import (
	"net"
)

// ParseIP is parse ip to string
func ParseIP(addr string) string {
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
