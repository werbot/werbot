// Package credential stores the secrets of resources: encrypted, write-only.
// Only Reveal returns plaintext, and only the gateway path calls it.
package credential

import (
	"fmt"

	"golang.org/x/crypto/ssh"

	"github.com/werbot/werbot/internal/connector"
	"github.com/werbot/werbot/internal/platform/apperr"
)

const (
	maxPassword   = 1024
	maxPrivateKey = 16384
)

// Secret is a decrypted credential.
type Secret struct {
	Kind  connector.CredentialKind
	Value []byte
}

func invalid(msg string) error { return fmt.Errorf("%w: %s", apperr.ErrInvalidArgument, msg) }

// validate checks value against kind without ever echoing it.
func validate(kind connector.CredentialKind, value []byte) error {
	if len(value) == 0 {
		return invalid("credential value is empty")
	}
	switch kind {
	case connector.CredentialPassword:
		if len(value) > maxPassword {
			return invalid("password is longer than 1024 bytes")
		}
	case connector.CredentialPrivateKey:
		if len(value) > maxPrivateKey {
			return invalid("private key is longer than 16384 bytes")
		}
		if _, err := ssh.ParseRawPrivateKey(value); err != nil {
			return invalid("private key is not a valid unencrypted key")
		}
	default:
		return invalid("unknown credential kind")
	}
	return nil
}
