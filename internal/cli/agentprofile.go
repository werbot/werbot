package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"connectrpc.com/connect"
	"github.com/spf13/cobra"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
	"github.com/werbot/werbot/gen/werbot/v1/werbotv1connect"
	"github.com/werbot/werbot/internal/clientauth"
)

func newAgentProfileCommand(env Env) *cobra.Command {
	cmd := &cobra.Command{Use: "profile", Short: "Manage agent profiles for another OS user or a container"}
	cmd.AddCommand(newProfileCreateCommand(env), newProfileListCommand(env), newProfileRevokeCommand(env))
	requireLogin(cmd.Commands()...)
	return cmd
}

func newProfileCreateCommand(env Env) *cobra.Command {
	var dir, label string
	var ttl time.Duration
	cmd := &cobra.Command{
		Use:   "create <agent> --dir <path> [--ttl <duration>] [--label <text>]",
		Short: "Write a profile directory that can only start runs of one agent", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			_, p, err := loadProfile(env)
			if err != nil {
				return err
			}
			if err := checkEmptyDir(dir); err != nil {
				return err
			}
			if label == "" {
				label, _ = os.Hostname()
			}
			return createProfile(cmd.Context(), env, p, args[0], dir, label, ttl)
		},
	}
	cmd.Flags().StringVar(&dir, "dir", "", "profile directory to write; missing or empty (required)")
	cmd.Flags().DurationVar(&ttl, "ttl", 0, "profile lifetime, 1h to 90d (default: the organization's CLI session limit)")
	cmd.Flags().StringVar(&label, "label", "", "what the profile is for (default: this host name)")
	_ = cmd.MarkFlagRequired("dir")
	return cmd
}

// createProfile makes the profile on the server and writes dir; a profile
// that cannot be written is revoked again.
func createProfile(ctx context.Context, env Env, p *clientauth.Profile, name, dir, label string, ttl time.Duration) error {
	client := werbotv1connect.NewAgentProfileServiceClient(p.HTTPClient(), p.Server)
	agent, err := sponsoredAgent(ctx, p, name)
	if err != nil {
		return err
	}
	res, err := client.CreateAgentProfile(ctx, connect.NewRequest(&werbotv1.CreateAgentProfileRequest{
		AgentId: agent.GetId(), Ttl: durationpb.New(ttl), Label: label,
	}))
	if err != nil {
		return err
	}
	id := res.Msg.GetProfile().GetId()
	if err := writeAgentHome(dir, p, agent.GetName(), res.Msg.GetRefreshToken()); err != nil {
		_, rerr := client.RevokeAgentProfile(context.WithoutCancel(ctx), connect.NewRequest(&werbotv1.RevokeAgentProfileRequest{Id: id}))
		return errors.Join(err, rerr)
	}
	_, err = fmt.Fprintf(env.Stderr, "Agent profile %s for %s written to %s\nRun as the agent user: WERBOT_HOME=%s werbot mcp\nHand the directory over: sudo chown -R <user> %s\n",
		id, agent.GetName(), dir, dir, dir)
	return err
}

// findAgent is the caller's agent called name, or nil.
func findAgent(ctx context.Context, p *clientauth.Profile, name string) (*werbotv1.Agent, error) {
	list, err := werbotv1connect.NewAgentServiceClient(p.HTTPClient(), p.Server).
		ListAgents(ctx, connect.NewRequest(&werbotv1.ListAgentsRequest{}))
	if err != nil {
		return nil, err
	}
	for _, a := range list.Msg.GetAgents() {
		if a.GetName() == name && a.GetSponsorUserId() == p.UserID.String() {
			return a, nil
		}
	}
	return nil, nil
}

// sponsoredAgent is findAgent, creating the agent when it is missing.
func sponsoredAgent(ctx context.Context, p *clientauth.Profile, name string) (*werbotv1.Agent, error) {
	a, err := findAgent(ctx, p, name)
	if a != nil || err != nil {
		return a, err
	}
	res, err := werbotv1connect.NewAgentServiceClient(p.HTTPClient(), p.Server).
		CreateAgent(ctx, connect.NewRequest(&werbotv1.CreateAgentRequest{Name: name}))
	if err != nil {
		return nil, err
	}
	return res.Msg.GetAgent(), nil
}

// checkEmptyDir accepts a missing or empty directory.
func checkEmptyDir(dir string) error {
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if len(entries) > 0 {
		return fmt.Errorf("%s is not empty; pick a new directory", dir)
	}
	return nil
}

// writeAgentHome writes the agent profile of p into dir (0700, files 0600):
// the server, the agent and the profile refresh token, nothing of the sponsor's
// tokens, key or certificate.
func writeAgentHome(dir string, p *clientauth.Profile, agent, refreshToken string) error {
	if err := checkEmptyDir(dir); err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	if err := os.Chmod(dir, 0o700); err != nil { //nolint:gosec // a directory needs the execute bit
		return err
	}
	ap := &clientauth.Profile{Server: p.Server, OrgID: p.OrgID, UserID: p.UserID, Login: p.Login, Agent: agent, RefreshToken: refreshToken}
	return ap.Save(dir)
}

func newProfileListCommand(env Env) *cobra.Command {
	return &cobra.Command{
		Use: "list [<agent>]", Short: "List the live agent profiles", Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			_, p, err := loadProfile(env)
			if err != nil {
				return err
			}
			req := &werbotv1.ListAgentProfilesRequest{}
			if len(args) == 1 {
				a, err := findAgent(cmd.Context(), p, args[0])
				if err != nil {
					return err
				}
				if a == nil {
					return fmt.Errorf("agent %q not found", args[0])
				}
				req.AgentId = a.GetId()
			}
			res, err := werbotv1connect.NewAgentProfileServiceClient(p.HTTPClient(), p.Server).
				ListAgentProfiles(cmd.Context(), connect.NewRequest(req))
			if err != nil {
				return err
			}
			w := tabwriter.NewWriter(env.Stdout, 0, 0, 2, ' ', 0)
			_, _ = fmt.Fprintln(w, "ID\tAGENT\tLABEL\tCREATED\tLAST USED\tEXPIRES")
			for _, a := range res.Msg.GetProfiles() {
				_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n", a.GetId(), a.GetAgentName(), a.GetLabel(),
					stamp(a.GetCreatedAt(), "-"), stamp(a.GetLastUsedAt(), "never"), stamp(a.GetExpiresAt(), "-"))
			}
			return w.Flush()
		},
	}
}

// stamp is the RFC 3339 time of t, or none when t is unset.
func stamp(t *timestamppb.Timestamp, none string) string {
	if t == nil {
		return none
	}
	return t.AsTime().Format(time.RFC3339)
}

func newProfileRevokeCommand(env Env) *cobra.Command {
	return &cobra.Command{
		Use: "revoke <id>", Short: "Revoke an agent profile", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			_, p, err := loadProfile(env)
			if err != nil {
				return err
			}
			if _, err := werbotv1connect.NewAgentProfileServiceClient(p.HTTPClient(), p.Server).
				RevokeAgentProfile(cmd.Context(), connect.NewRequest(&werbotv1.RevokeAgentProfileRequest{Id: args[0]})); err != nil {
				return err
			}
			_, err = fmt.Fprintf(env.Stdout, "Revoked agent profile %s\n", args[0])
			return err
		},
	}
}
