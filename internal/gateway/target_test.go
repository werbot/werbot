package gateway

import (
	"net"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDenyLocalTargets(t *testing.T) {
	for _, addr := range []string{
		"127.0.0.1:5432", "127.1.2.3:5432", "[::1]:5432", "[::ffff:127.0.0.1]:5432",
		"169.254.169.254:80", "169.254.1.1:80", "[fe80::1]:22", "[fe80::1%lo0]:22", "0.0.0.0:22", "[::]:22",
	} {
		require.ErrorIs(t, denyLocal("tcp", addr, nil), ErrTargetNotAllowed, addr)
	}
	for _, addr := range []string{"10.0.0.5:5432", "192.0.2.1:53", "[2001:db8::1]:443"} {
		require.NoError(t, denyLocal("tcp", addr, nil), addr)
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer ln.Close() //nolint:errcheck // test listener
	deny := newDialer(true)
	for _, addr := range []string{ln.Addr().String(), "localhost:1"} {
		c, err := deny(t.Context(), "tcp", addr)
		require.ErrorIs(t, err, ErrTargetNotAllowed, addr)
		require.Nil(t, c)
	}
	c, err := deny(t.Context(), "udp", "127.0.0.1:53")
	require.ErrorIs(t, err, ErrTargetNotAllowed, "udp too")
	require.Nil(t, c)

	c, err = newDialer(false)(t.Context(), "tcp", ln.Addr().String())
	require.NoError(t, err, "the flag off allows loopback")
	require.NoError(t, c.Close())
}
