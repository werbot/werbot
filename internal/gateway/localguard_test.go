package gateway

import (
	"net"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDenyLocalTargets(t *testing.T) {
	g := localGuard{deny: true}
	ctx := t.Context()
	for _, addr := range []string{
		"127.0.0.1:5432", "127.1.2.3:5432", "[::1]:5432", "[::ffff:127.0.0.1]:5432",
		"169.254.169.254:80", "169.254.1.1:80", "[fe80::1]:22", "[fe80::1%lo0]:22", "0.0.0.0:22", "[::]:22",
		"[ff01::1]:53", "[ff02::fb]:5353", "224.0.0.251:5353", "239.1.1.1:53",
	} {
		require.ErrorIs(t, g.control(ctx, "udp", addr, nil), ErrTargetNotAllowed, addr)
	}
	for _, addr := range []string{"10.0.0.5:5432", "192.0.2.1:53", "[2001:db8::1]:443"} {
		require.NoError(t, g.control(ctx, "tcp", addr, nil), addr)
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer ln.Close() //nolint:errcheck // test listener
	deny := g.dialer()
	for _, addr := range []string{ln.Addr().String(), "localhost:1"} {
		c, err := deny(ctx, "tcp", addr)
		require.ErrorIs(t, err, ErrTargetNotAllowed, addr)
		require.Nil(t, c)
	}
	c, err := deny(ctx, "udp", "127.0.0.1:53")
	require.ErrorIs(t, err, ErrTargetNotAllowed, "udp too")
	require.Nil(t, c)

	c, err = localGuard{}.dialer()(ctx, "tcp", ln.Addr().String())
	require.NoError(t, err, "the flag off allows loopback")
	require.NoError(t, c.Close())
}

// The server's database host is refused at whatever it resolves to; a host that
// does not resolve refuses every target (fail closed).
func TestDenyDatabaseHost(t *testing.T) {
	ctx := t.Context()
	g := localGuard{deny: true, hosts: []string{"10.1.2.3"}}
	require.ErrorIs(t, g.control(ctx, "tcp", "10.1.2.3:5432", nil), ErrTargetNotAllowed)
	require.ErrorIs(t, g.control(ctx, "tcp", "[::ffff:10.1.2.3]:22", nil), ErrTargetNotAllowed)
	require.NoError(t, g.control(ctx, "tcp", "10.1.2.4:5432", nil))
	require.ErrorIs(t, g.allowHost(ctx, "10.1.2.3"), ErrTargetNotAllowed)
	require.NoError(t, g.allowHost(ctx, "10.1.2.4"))

	g.hosts = []string{"db.invalid"}
	require.ErrorIs(t, g.allowIP(ctx, netip.MustParseAddr("10.1.2.4")), ErrTargetNotAllowed)

	g.deny = false
	require.NoError(t, g.allowHost(ctx, "10.1.2.3"), "off: nothing is checked")
}
