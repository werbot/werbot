package connectortest_test

import (
	"context"
	"crypto/rand"
	"testing"

	"github.com/werbot/werbot/internal/gateway"
	"github.com/werbot/werbot/internal/gateway/connectortest"
)

func TestEchoPassesContract(t *testing.T) {
	connectortest.Run(t, connectortest.Echo{}, connectortest.Target{
		Resource: gateway.Resource{Name: "echo1", Type: "echo"},
		Secret:   gateway.Secret{Kind: "password", Value: []byte("werbot-secret-" + rand.Text())},
	})
}

// udpEcho is a fake datagram connector: it sends every datagram back to its flow.
type udpEcho struct{}

func (udpEcho) Type() string          { return "udpecho" }
func (udpEcho) Kinds() []gateway.Kind { return []gateway.Kind{gateway.KindDatagram} }
func (udpEcho) Serve(ctx context.Context, s *gateway.Session) error {
	defer s.Datagram.Close()
	defer context.AfterFunc(ctx, func() { _ = s.Datagram.Close() })()
	buf := make([]byte, 2048)
	for {
		flow, n, err := s.Datagram.Recv(buf)
		if err != nil {
			return nil //nolint:nilerr // the client end closed: a normal end
		}
		if err := s.Datagram.Send(flow, buf[:n]); err != nil {
			return nil //nolint:nilerr // the client end closed: a normal end
		}
	}
}

func TestRunDatagramFlows(t *testing.T) {
	connectortest.Run(t, udpEcho{}, connectortest.Target{
		Resource: gateway.Resource{Name: "dns1", Type: "udpecho"},
		Secret:   gateway.Secret{Kind: "password", Value: []byte("werbot-secret-" + rand.Text())},
		Datagram: "192.0.2.53:53",
	})
}
