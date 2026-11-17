package connectcli

import (
	"errors"
	"fmt"
	"io/fs"
	"net"
	"os"
	"path/filepath"
	"sync"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"golang.org/x/crypto/ssh/knownhosts"
)

// errHostKeyMismatch is a gateway host key unlike the one in known_hosts.
var errHostKeyMismatch = errors.New("host key mismatch")

// authMethod is the key in identity, else the ssh-agent at SSH_AUTH_SOCK if it
// answers, else ~/.ssh/id_ed25519. The agent is only signed with, never forwarded. The closer
// releases the agent connection.
func authMethod(identity string) (ssh.AuthMethod, func(), error) {
	if identity == "" {
		if sock := os.Getenv("SSH_AUTH_SOCK"); sock != "" {
			// a stale socket (after a re-login, in tmux) falls back to the key file, as in OpenSSH
			if conn, err := net.Dial("unix", sock); err == nil { //nolint:gosec // SSH_AUTH_SOCK is the user's own agent
				return ssh.PublicKeysCallback(agent.NewClient(conn).Signers), func() { _ = conn.Close() }, nil
			}
		}
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, nil, fmt.Errorf("no key: %w; use -i", err)
		}
		identity = filepath.Join(home, ".ssh", "id_ed25519")
	}
	b, err := os.ReadFile(identity) //nolint:gosec // the user's own key file
	if err != nil {
		return nil, nil, fmt.Errorf("key: %w", err)
	}
	signer, err := ssh.ParsePrivateKey(b)
	if _, ok := errors.AsType[*ssh.PassphraseMissingError](err); ok {
		return nil, nil, fmt.Errorf("key %s is encrypted; load it into ssh-agent", identity)
	}
	if err != nil {
		return nil, nil, fmt.Errorf("key %s: %w", identity, err)
	}
	return ssh.PublicKeys(signer), func() {}, nil
}

// knownHostsMu serializes known_hosts reads and appends of this process.
var knownHostsMu sync.Mutex

// hostKeyCallback checks the gateway key against the known_hosts file at path
// like OpenSSH StrictHostKeyChecking=accept-new: an unknown host is appended
// (directory 0700, file 0600), a different key fails. The file is read again
// under the lock before each append, so concurrent connections add a host once.
func hostKeyCallback(path string) ssh.HostKeyCallback {
	return func(host string, remote net.Addr, key ssh.PublicKey) error {
		knownHostsMu.Lock()
		defer knownHostsMu.Unlock()
		check, err := knownhosts.New(path)
		if errors.Is(err, fs.ErrNotExist) {
			check, err = knownhosts.New()
		}
		if err != nil {
			return fmt.Errorf("known_hosts: %w", err)
		}
		err = check(host, remote, key)
		ke, ok := errors.AsType[*knownhosts.KeyError](err)
		switch {
		case !ok:
			return err
		case len(ke.Want) > 0:
			return fmt.Errorf("%w for %s", errHostKeyMismatch, host)
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			return fmt.Errorf("known_hosts: %w", err)
		}
		f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600) //nolint:gosec // the user's own file
		if err != nil {
			return fmt.Errorf("known_hosts: %w", err)
		}
		_, err = fmt.Fprintln(f, knownhosts.Line([]string{knownhosts.Normalize(host)}, key))
		if cerr := f.Close(); err == nil {
			err = cerr
		}
		if err != nil {
			return fmt.Errorf("known_hosts: %w", err)
		}
		return nil
	}
}
