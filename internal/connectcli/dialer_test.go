package connectcli

import (
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var _ Dialer = Dial

func TestDial(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer ln.Close() //nolint:errcheck // test listener

	c, err := Dial(t.Context(), Endpoint{SSHAddress: ln.Addr().String()})
	require.NoError(t, err)
	require.NoError(t, c.Close())

	_, err = Dial(t.Context(), Endpoint{})
	assert.EqualError(t, err, "gateway address is not set")
}
