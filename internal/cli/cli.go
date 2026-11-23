// Package cli holds the werbot client commands (connect, and the login family).
// It depends only on client-side packages, never on the server or its domains.
package cli

import (
	"errors"
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

// Commands returns the client commands of the root command. A token the
// server refuses or a refused refresh ends any of them with ErrLoginRequired.
func Commands(env Env) []*cobra.Command {
	cmds := []*cobra.Command{
		loginCommand(env), logoutCommand(env), statusCommand(env), lsCommand(env),
		sshCommand(env), sshConfigCommand(env), connectCommand(env),
		newRequestCommand(env), newRequestsCommand(env), newDecideCommand(env, true), newDecideCommand(env, false),
		newMCPCommand(env), newAgentsCommand(env),
	}
	agent := newAgentCommand(env)
	requireLogin(cmds...)
	requireLogin(agent.Commands()...)
	return append(cmds, agent)
}

// requireLogin ends every command in cmds with loginRequired.
func requireLogin(cmds ...*cobra.Command) {
	for _, c := range cmds {
		run := c.RunE
		c.RunE = func(cmd *cobra.Command, args []string) error { return loginRequired(run(cmd, args)) }
	}
}

// loginRequired maps an Unauthenticated answer or a refused refresh to
// clientauth.ErrLoginRequired; other errors pass as they are.
func loginRequired(err error) error {
	if connect.CodeOf(err) == connect.CodeUnauthenticated || errors.Is(err, clientauth.ErrLoginRequired) {
		return clientauth.ErrLoginRequired
	}
	return err
}

// loadProfile returns the profile home and the current profile of a human:
// an agent profile ends with clientauth.ErrAgentProfile.
func loadProfile(env Env) (string, *clientauth.Profile, error) {
	home, p, err := loadAnyProfile(env)
	if err == nil && p.Agent != "" {
		return "", nil, clientauth.ErrAgentProfile
	}
	return home, p, err
}

// loadAnyProfile is loadProfile for the commands an agent profile may use:
// status, logout, mcp and agent run.
func loadAnyProfile(env Env) (string, *clientauth.Profile, error) {
	home, err := clientauth.Home(env.Getenv)
	if err != nil {
		return "", nil, err
	}
	p, err := clientauth.Current(home, env.Getenv)
	return home, p, err
}
