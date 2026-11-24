package connector

import (
	"errors"
	"fmt"
	"math"
	"slices"
	"strings"

	"google.golang.org/protobuf/encoding/protojson"

	connectorv1 "github.com/werbot/werbot/gen/werbot/connector/v1"
)

// maxPortRanges bounds the port list of a resource.
const maxPortRanges = 16

// PortRange is a target port or an inclusive range; EndPort 0 means one port.
type PortRange struct {
	Port    uint16 `json:"port"`
	EndPort uint16 `json:"end_port"`
}

func (r PortRange) last() uint16 {
	if r.EndPort == 0 {
		return r.Port
	}
	return r.EndPort
}

// Contains reports whether p lies in the range.
func (r PortRange) Contains(p uint16) bool { return p >= r.Port && p <= r.last() }

// ValidatePorts checks 1–16 ranges: ports from 1, EndPort 0 or ≥ Port, no overlap.
func ValidatePorts(rs []PortRange) error {
	if len(rs) == 0 || len(rs) > maxPortRanges {
		return fmt.Errorf("ports: need 1 to %d ranges", maxPortRanges)
	}
	for i, r := range rs {
		if r.Port == 0 || (r.EndPort != 0 && r.EndPort < r.Port) {
			return fmt.Errorf("ports: invalid range %d", i)
		}
		for _, o := range rs[:i] {
			if r.Port <= o.last() && o.Port <= r.last() {
				return fmt.Errorf("ports: range %d overlaps", i)
			}
		}
	}
	return nil
}

// PortsFromProto converts wire ranges; a port above 65535 is an error.
func PortsFromProto(ps []*connectorv1.PortRange) ([]PortRange, error) {
	out := make([]PortRange, len(ps))
	for i, p := range ps {
		if p.GetPort() > math.MaxUint16 || p.GetEndPort() > math.MaxUint16 {
			return nil, fmt.Errorf("ports: range %d out of range", i)
		}
		//nolint:gosec // both values checked above
		out[i] = PortRange{Port: uint16(p.GetPort()), EndPort: uint16(p.GetEndPort())}
	}
	return out, nil
}

// Endpoint is where a stream or datagram resource may connect.
type Endpoint struct {
	Host      string
	Ports     []PortRange // target ports; the first port of each range is the TCP health check
	AnyTarget bool        // any host:port (ssh with port forwarding)
	NoTarget  bool        // no stream or datagram target; Ports stay for health (ssh without port forwarding)
	NoHealth  bool
	// RemoteForward permits remote forwards (tcpip-forward) on the target's
	// loopback: ssh with remote_forwarding.
	RemoteForward bool
}

// Allow reports whether host:port is a permitted target of the resource.
func (e Endpoint) Allow(resourceName, host string, port uint16) bool {
	if e.NoTarget {
		return false
	}
	if e.AnyTarget {
		return true
	}
	if host == "" || (!strings.EqualFold(host, e.Host) && !strings.EqualFold(host, resourceName)) {
		return false
	}
	return slices.ContainsFunc(e.Ports, func(r PortRange) bool { return r.Contains(port) })
}

// Records reports whether sessions of kind k are recorded.
func (t Type) Records(k Kind) bool { return slices.Contains(t.Recorded, k) }

// EndpointOf decodes and validates configJSON and returns its endpoint;
// false means the type has none.
func (t Type) EndpointOf(configJSON []byte) (Endpoint, bool, error) {
	if t.Endpoint == nil {
		return Endpoint{}, false, nil
	}
	cfg := t.NewConfig()
	if err := protojson.Unmarshal(configJSON, cfg); err != nil {
		return Endpoint{}, false, errors.New(t.Name + ": invalid config")
	}
	if err := t.Validate(cfg); err != nil {
		return Endpoint{}, false, err
	}
	return t.Endpoint(cfg), true, nil
}
