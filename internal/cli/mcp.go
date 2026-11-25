package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/werbot/werbot/internal/clientauth"
	"github.com/werbot/werbot/internal/mcpserver"
)

// sharedWarning is the one stderr line of the shared mode (owner decision O2).
const sharedWarning = "warning: the agent runs as your OS user and can read your werbot profile in %s; isolate it with \"werbot agents profile create\"\n"

// agentMode picks the agent of a profile. Shared mode (the sponsor's profile)
// warns once on stderr and returns flag; an agent profile returns its own
// agent and refuses another flag.
func agentMode(env Env, home string, p *clientauth.Profile, flag string) (string, error) {
	if p.Agent == "" {
		_, _ = fmt.Fprintf(env.Stderr, sharedWarning, home)
		return flag, nil
	}
	if flag != "" && flag != p.Agent {
		return "", fmt.Errorf("this profile is bound to agent %q", p.Agent)
	}
	return p.Agent, nil
}

func newMCPCommand(env Env) *cobra.Command {
	var agent, reason string
	cmd := &cobra.Command{
		Use: "mcp [--agent <name>] [--reason <text>]", Short: "Serve MCP on stdio as an agent of your account", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			home, p, err := loadAnyProfile(env)
			if err != nil {
				return err
			}
			if agent, err = agentMode(env, home, p, agent); err != nil {
				return err
			}
			return mcpserver.Run(cmd.Context(), env.Stdin, env.Stdout, mcpserver.Options{Profile: p, Agent: agent, Reason: reason})
		},
	}
	cmd.Flags().StringVar(&agent, "agent", "", "agent name (default: the MCP client name)")
	cmd.Flags().StringVar(&reason, "reason", "", "why the agent runs, shown to approvers")
	return cmd
}
