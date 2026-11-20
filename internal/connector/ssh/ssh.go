// Package ssh registers the "ssh" resource type.
package ssh

import (
	"errors"
	"fmt"

	"google.golang.org/protobuf/proto"

	sshv1 "github.com/werbot/werbot/gen/werbot/connector/ssh/v1"
	"github.com/werbot/werbot/internal/connector"
)

// TypeName is the registry name of the type.
const TypeName = "ssh"

func init() {
	connector.Register(connector.Type{
		Name:        TypeName,
		Category:    connector.CategoryServers,
		NewConfig:   func() proto.Message { return &sshv1.Config{} },
		Validate:    Validate,
		Credentials: []connector.CredentialKind{connector.CredentialPassword, connector.CredentialPrivateKey},
		Actions:     []string{"ssh.connect", "ssh.exec"}, // connect first: callers use Actions[0]
		Level:       "L2",
		Kinds:       []connector.Kind{connector.KindTerminal, connector.KindExec, connector.KindStream},
		Recorded:    []connector.Kind{connector.KindTerminal, connector.KindExec},
		Endpoint: func(m proto.Message) connector.Endpoint {
			c := m.(*sshv1.Config)
			port := uint16(c.Port) //nolint:gosec // Validate caps the port at 65535
			if port == 0 {
				port = 22
			}
			return connector.Endpoint{Host: c.Host, Ports: []connector.PortRange{{Port: port}}, AnyTarget: c.PortForwarding, NoTarget: !c.PortForwarding}
		},
	})
}

// Validate checks a *sshv1.Config and sets the default port.
func Validate(m proto.Message) error {
	c, ok := m.(*sshv1.Config)
	if !ok {
		return fmt.Errorf("ssh: unexpected config type %T", m)
	}
	if !connector.ValidHost(c.Host) {
		return errors.New("ssh: invalid host")
	}
	if c.Port == 0 {
		c.Port = 22
	}
	if c.Port > 65535 {
		return errors.New("ssh: invalid port")
	}
	if c.Login == "" || !connector.ValidLogin(c.Login) {
		return errors.New("ssh: invalid login")
	}
	return nil
}
