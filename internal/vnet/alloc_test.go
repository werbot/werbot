package vnet

import (
	"net/netip"
	"slices"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func ids(n int) []uuid.UUID {
	out := make([]uuid.UUID, n)
	for i := range out {
		out[i] = uuid.NewSHA1(uuid.NameSpaceOID, []byte{byte(i), byte(i >> 8)})
	}
	return out
}

func TestAllocateDeterministic(t *testing.T) {
	p := netip.MustParsePrefix(DefaultPrefix)
	in := ids(500)
	a, err := Allocate(p, in)
	require.NoError(t, err)
	rev := slices.Clone(in)
	slices.Reverse(rev)
	b, err := Allocate(p, rev)
	require.NoError(t, err)
	require.Equal(t, a, b)

	seen := map[netip.Addr]bool{}
	for _, ip := range a {
		require.True(t, p.Contains(ip))
		require.NotContains(t, []netip.Addr{netip.MustParseAddr("100.64.0.0"), IfaceAddr(p), DNSAddr(p), netip.MustParseAddr("100.127.255.255")}, ip)
		require.False(t, seen[ip])
		seen[ip] = true
	}

	more, err := Allocate(p, append(slices.Clone(in), uuid.New()))
	require.NoError(t, err)
	same := 0
	for id, ip := range a {
		if more[id] == ip {
			same++
		}
	}
	require.GreaterOrEqual(t, same, len(a)-1) // one new id may shift at most one older id
}

func TestAllocateCapacity(t *testing.T) {
	p := netip.MustParsePrefix("100.64.0.0/29")
	got, err := Allocate(p, ids(4))
	require.NoError(t, err)
	require.Len(t, got, 4)
	addrs := map[netip.Addr]bool{}
	for _, ip := range got {
		addrs[ip] = true
		require.True(t, ip.Compare(netip.MustParseAddr("100.64.0.3")) >= 0 && ip.Compare(netip.MustParseAddr("100.64.0.6")) <= 0, ip.String())
	}
	require.Len(t, addrs, 4)
	_, err = Allocate(p, ids(5))
	require.Error(t, err)
}

func TestDNSAddr(t *testing.T) {
	p := netip.MustParsePrefix(DefaultPrefix)
	require.Equal(t, "100.64.0.1", IfaceAddr(p).String())
	require.Equal(t, "100.64.0.2", DNSAddr(p).String())
}
