package vnet

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"hash/fnv"
	"net/netip"
	"slices"

	"github.com/google/uuid"
)

// DefaultPrefix is the carrier-grade NAT range used for resource addresses.
const DefaultPrefix = "100.64.0.0/10"

// TTL is the lifetime in seconds of A records.
const TTL = 60

// Resource is one accessible resource as vnet sees it.
type Resource struct {
	ID                         uuid.UUID
	Name                       string
	Kinds                      []string
	SSHAddress, WSURL, HostKey string
}

// reserved counts the network address, IfaceAddr, DNSAddr and broadcast.
const reserved = 4

func base(prefix netip.Prefix) (uint32, error) {
	p := prefix.Masked()
	if !p.Addr().Is4() || p.Bits() > 32-3 {
		return 0, fmt.Errorf("vnet prefix %s must be IPv4 and at least a /29", prefix)
	}
	return binary.BigEndian.Uint32(p.Addr().AsSlice()), nil
}

func addr(v uint32) netip.Addr {
	return netip.AddrFrom4([4]byte(binary.BigEndian.AppendUint32(nil, v)))
}

// IfaceAddr is the address of the virtual interface: network + 1.
func IfaceAddr(prefix netip.Prefix) netip.Addr {
	b, _ := base(prefix)
	return addr(b + 1)
}

// DNSAddr is the address of the DNS responder: network + 2.
func DNSAddr(prefix netip.Prefix) netip.Addr {
	b, _ := base(prefix)
	return addr(b + 2)
}

// Allocate gives every id an address from network + 3. The same set of ids always
// gets the same addresses, whatever its order.
func Allocate(prefix netip.Prefix, ids []uuid.UUID) (map[uuid.UUID]netip.Addr, error) {
	b, err := base(prefix)
	if err != nil {
		return nil, err
	}
	n := uint32(1)<<(32-prefix.Bits()) - reserved
	sorted := slices.SortedFunc(slices.Values(ids), func(a, b uuid.UUID) int { return bytes.Compare(a[:], b[:]) })
	sorted = slices.Compact(sorted)
	if uint64(len(sorted)) > uint64(n) {
		return nil, errors.New("vnet prefix has no room for all resources")
	}
	out := make(map[uuid.UUID]netip.Addr, len(sorted))
	taken := make(map[uint32]bool, len(sorted))
	for _, id := range sorted {
		h := fnv.New32a()
		_, _ = h.Write(id[:])
		i := h.Sum32() % n
		for taken[i] {
			i = (i + 1) % n
		}
		taken[i] = true
		out[id] = addr(b + 3 + i)
	}
	return out, nil
}
