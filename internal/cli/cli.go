// Package cli holds the werbot client commands (connect, and the login family).
// It depends only on client-side packages, never on the server or its domains.
package cli

import (
	"io"

	"connectrpc.com/connect"
	"github.com/spf13/cobra"

	"github.com/werbot/werbot/internal/clientauth"
)

// Env is what the client commands read and write, so tests can replace it.
type Env struct {
	Getenv      func(string) string
	Stdin       io.Reader
	Stdout      io.Writer
	Stderr      io.Writer
	OpenBrowser func(string) error
}

// Commands returns the client commands of the root command.
func Commands(env Env) []*cobra.Command {
	return []*cobra.Command{
		loginCommand(env), logoutCommand(env), statusCommand(env), lsCommand(env),
		sshCommand(env), sshConfigCommand(env), connectCommand(env),
	}
}

// loadProfile returns the profile home and the current profile.
func loadProfile(env Env) (string, *clientauth.Profile, error) {
	home, err := clientauth.Home(env.Getenv)
	if err != nil {
		return "", nil, err
	}
	p, err := clientauth.Current(home, env.Getenv)
	return home, p, err
}

// newClient builds a Connect client that sends the profile's Bearer token.
func newClient[T any](p *clientauth.Profile, ctor func(connect.HTTPClient, string, ...connect.ClientOption) T) T {
	return ctor(p.HTTPClient(), p.Server)
}
