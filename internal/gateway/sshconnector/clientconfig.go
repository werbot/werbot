package sshconnector

import (
	"errors"
	"fmt"
	"net"
	"time"

	gossh "golang.org/x/crypto/ssh"

	"github.com/werbot/werbot/internal/connector"
	"github.com/werbot/werbot/internal/gateway"
)

// clientConfig builds the client config of one session. The host key goes to
// pinKey (trust on first use through the server); errors never carry the secret.
func clientConfig(login string, sec gateway.Secret, pinKey func(alg string, key []byte) error, timeout time.Duration) (*gossh.ClientConfig, error) {
	if len(sec.Value) == 0 {
		return nil, errors.New("ssh: empty credential")
	}
	var auth gossh.AuthMethod
	switch connector.CredentialKind(sec.Kind) {
	case connector.CredentialPassword:
		auth = gossh.Password(string(sec.Value))
	case connector.CredentialPrivateKey:
		signer, err := gossh.ParsePrivateKey(sec.Value)
		if err != nil {
			return nil, fmt.Errorf("ssh: parse private key: %w", err)
		}
		auth = gossh.PublicKeys(signer)
	default:
		return nil, fmt.Errorf("ssh: credential kind %q is not supported", sec.Kind)
	}
	return &gossh.ClientConfig{
		User: login,
		Auth: []gossh.AuthMethod{auth},
		HostKeyCallback: func(_ string, _ net.Addr, key gossh.PublicKey) error {
			return pinKey(key.Type(), key.Marshal())
		},
		Timeout: timeout,
	}, nil
}
