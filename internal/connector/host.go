package connector

import (
	"strings"

	connectorv1 "github.com/werbot/werbot/gen/werbot/connector/v1"
)

// ValidHost reports whether h is a usable target host: 1–253 bytes, no spaces or slashes.
func ValidHost(h string) bool {
	return h != "" && len(h) <= 253 && !strings.ContainsAny(h, " /")
}

// CheckPorts converts wire ranges and validates them with ValidatePorts.
func CheckPorts(ps []*connectorv1.PortRange) ([]PortRange, error) {
	rs, err := PortsFromProto(ps)
	if err != nil {
		return nil, err
	}
	return rs, ValidatePorts(rs)
}
