// Package udp registers the "udp" resource type.
package udp

import (
	"errors"
	"fmt"
	"time"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/durationpb"

	udpv1 "github.com/werbot/werbot/gen/werbot/connector/udp/v1"
	"github.com/werbot/werbot/internal/connector"
)

// TypeName is the registry name of the type.
const TypeName = "udp"

const (
	defaultFlowIdle = 2 * time.Minute // NAT binding minimum, RFC 4787
	minFlowIdle     = time.Second
	maxFlowIdle     = time.Hour
)

func init() {
	connector.Register(connector.Type{
		Name:      TypeName,
		Category:  connector.CategoryServers,
		NewConfig: func() proto.Message { return &udpv1.Config{} },
		Validate:  Validate,
		Actions:   []string{"udp.connect"},
		Level:     "L1",
		Kinds:     []connector.Kind{connector.KindDatagram},
		Endpoint: func(m proto.Message) connector.Endpoint {
			c := m.(*udpv1.Config)
			// An unconverted range leaves no ports, so Allow denies everything.
			ports, _ := connector.PortsFromProto(c.Ports)
			return connector.Endpoint{Host: c.Host, Ports: ports, NoHealth: true}
		},
	})
}

// Validate checks a *udpv1.Config and sets the default flow_idle.
func Validate(m proto.Message) error {
	c, ok := m.(*udpv1.Config)
	if !ok {
		return fmt.Errorf("udp: unexpected config type %T", m)
	}
	if !connector.ValidHost(c.Host) {
		return errors.New("udp: invalid host")
	}
	if _, err := connector.CheckPorts(c.Ports); err != nil {
		return fmt.Errorf("udp: %w", err)
	}
	if c.FlowIdle == nil || c.FlowIdle.AsDuration() == 0 && c.FlowIdle.CheckValid() == nil {
		c.FlowIdle = durationpb.New(defaultFlowIdle)
		return nil
	}
	if err := c.FlowIdle.CheckValid(); err != nil {
		return errors.New("udp: invalid flow_idle")
	}
	if d := c.FlowIdle.AsDuration(); d < minFlowIdle || d > maxFlowIdle {
		return errors.New("udp: flow_idle must be 1s to 1h")
	}
	return nil
}
