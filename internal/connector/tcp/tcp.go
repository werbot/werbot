// Package tcp registers the "tcp" resource type.
package tcp

import (
	"errors"
	"fmt"

	"google.golang.org/protobuf/proto"

	tcpv1 "github.com/werbot/werbot/gen/werbot/connector/tcp/v1"
	"github.com/werbot/werbot/internal/connector"
)

// TypeName is the registry name of the type.
const TypeName = "tcp"

func init() {
	connector.Register(connector.Type{
		Name:      TypeName,
		Category:  connector.CategoryServers,
		NewConfig: func() proto.Message { return &tcpv1.Config{} },
		Validate:  Validate,
		Actions:   []string{"tcp.connect"},
		Level:     "L1",
		Kinds:     []connector.Kind{connector.KindStream},
		Endpoint: func(m proto.Message) connector.Endpoint {
			c := m.(*tcpv1.Config)
			// An unconverted range leaves no ports, so Allow denies everything.
			ports, _ := connector.PortsFromProto(c.Ports)
			return connector.Endpoint{Host: c.Host, Ports: ports}
		},
	})
}

// Validate checks a *tcpv1.Config.
func Validate(m proto.Message) error {
	c, ok := m.(*tcpv1.Config)
	if !ok {
		return fmt.Errorf("tcp: unexpected config type %T", m)
	}
	if !connector.ValidHost(c.Host) {
		return errors.New("tcp: invalid host")
	}
	if _, err := connector.CheckPorts(c.Ports); err != nil {
		return fmt.Errorf("tcp: %w", err)
	}
	return nil
}
