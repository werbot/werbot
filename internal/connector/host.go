package connector

import (
	"net/netip"
	"strings"
	"unicode"

	connectorv1 "github.com/werbot/werbot/gen/werbot/connector/v1"
)

// ValidHost reports whether h is a bare hostname or IP: 1–253 bytes, no spaces,
// slashes, brackets or control characters; a colon only in an unzoned IPv6 literal.
func ValidHost(h string) bool {
	if h == "" || len(h) > 253 || strings.ContainsAny(h, " /[]") || strings.ContainsFunc(h, unicode.IsControl) {
		return false
	}
	if strings.Contains(h, ":") {
		a, err := netip.ParseAddr(h)
		return err == nil && a.Zone() == ""
	}
	return true
}

// ValidLogin reports whether l is a login of at most 64 bytes without spaces or control characters.
func ValidLogin(l string) bool {
	return len(l) <= 64 && !strings.Contains(l, " ") && !strings.ContainsFunc(l, unicode.IsControl)
}

// CheckPorts converts wire ranges and validates them with ValidatePorts.
func CheckPorts(ps []*connectorv1.PortRange) ([]PortRange, error) {
	rs, err := PortsFromProto(ps)
	if err != nil {
		return nil, err
	}
	return rs, ValidatePorts(rs)
}
