package connectortest_test

import (
	"crypto/rand"
	"net"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/gateway"
	"github.com/werbot/werbot/internal/gateway/connectortest"
)

func secret() gateway.Secret {
	return gateway.Secret{Kind: "password", Value: []byte("werbot-secret-" + rand.Text())}
}

func TestEchoPassesContract(t *testing.T) {
	connectortest.Run(t, connectortest.Echo{}, connectortest.Target{
		Resource: gateway.Resource{Name: "echo1", Type: "echo"}, Secret: secret(),
	})
}

// A TargetDialer that reaches a real TCP target only through Session.Dial passes.
func TestRunTargetDialer(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	l, accepts := connectortest.CountAccepts(l)
	t.Cleanup(func() { _ = l.Close() })
	go echoTCP(l)
	connectortest.Run(t, tcpPipe{}, connectortest.Target{
		Resource: gateway.Resource{Name: "db1", Type: "tcppipe"}, Secret: secret(),
		Stream: l.Addr().String(), Accepts: accepts,
	})
}

// Datagrams of several flows, an empty one and a maximum-size one come back in their flow.
func TestRunDatagramFlows(t *testing.T) {
	pc, err := net.ListenPacket("udp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = pc.Close() })
	go echoUDP(pc)
	connectortest.Run(t, udpEcho{}, connectortest.Target{
		Resource: gateway.Resource{Name: "dns1", Type: "udpecho"}, Secret: secret(),
		Datagram: pc.LocalAddr().String(),
	})
}
