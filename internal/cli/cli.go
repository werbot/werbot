// Package cli holds the werbot client commands (connect, and the login family).
// It depends only on client-side packages, never on the server or its domains.
package cli

import (
	"io"

	"github.com/spf13/cobra"

	"github.com/werbot/werbot/internal/connectcli"
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
	return []*cobra.Command{connectCommand(env)}
}

// connectCommand keeps the flag set of werbot connect: it parses its own args.
func connectCommand(env Env) *cobra.Command {
	return &cobra.Command{
		Use:                "connect [flags] [<local_port>:]<resource>[:<target_port>] ...",
		Short:              "Open local ports to resources through the gateway",
		DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := connectcli.LoadConfig(args, env.Getenv)
			if err != nil {
				return err
			}
			return connectcli.Run(cmd.Context(), cfg, env.Stdout, env.Stderr)
		},
	}
}
