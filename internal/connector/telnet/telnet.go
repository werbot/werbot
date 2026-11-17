// Package telnet registers the "telnet" resource type.
package telnet

import (
	"errors"
	"fmt"
	"strings"

	"google.golang.org/protobuf/proto"

	telnetv1 "github.com/werbot/werbot/gen/werbot/connector/telnet/v1"
	"github.com/werbot/werbot/internal/connector"
)

// TypeName is the registry name of the type.
const TypeName = "telnet"

func init() {
	connector.Register(connector.Type{
		Name:        TypeName,
		Category:    connector.CategoryServers,
		NewConfig:   func() proto.Message { return &telnetv1.Config{} },
		Validate:    Validate,
		Credentials: []connector.CredentialKind{connector.CredentialPassword},
		Actions:     []string{"telnet.connect"},
		Level:       "L2",
		Kinds:       []connector.Kind{connector.KindTerminal, connector.KindStream},
		Recorded:    []connector.Kind{connector.KindTerminal, connector.KindStream},
		Endpoint: func(m proto.Message) connector.Endpoint {
			c := m.(*telnetv1.Config)
			port := uint16(c.Port) //nolint:gosec // Validate caps the port at 65535
			if port == 0 {
				port = 23
			}
			return connector.Endpoint{Host: c.Host, Ports: []connector.PortRange{{Port: port}}}
		},
	})
}

// Validate checks a *telnetv1.Config and sets the default port.
func Validate(m proto.Message) error {
	c, ok := m.(*telnetv1.Config)
	if !ok {
		return fmt.Errorf("telnet: unexpected config type %T", m)
	}
	if !connector.ValidHost(c.Host) {
		return errors.New("telnet: invalid host")
	}
	if c.Port == 0 {
		c.Port = 23
	}
	if c.Port > 65535 {
		return errors.New("telnet: invalid port")
	}
	if len(c.Login) > 64 || strings.Contains(c.Login, " ") {
		return errors.New("telnet: invalid login")
	}
	return nil
}
