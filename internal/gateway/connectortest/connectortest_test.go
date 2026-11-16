package connectortest_test

import (
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
