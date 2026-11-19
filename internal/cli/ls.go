package cli

import (
	"context"
	"fmt"
	"strings"
	"text/tabwriter"

	"connectrpc.com/connect"
	"github.com/spf13/cobra"

	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
	"github.com/werbot/werbot/gen/werbot/v1/werbotv1connect"
	"github.com/werbot/werbot/internal/clientauth"
)

// accessible lists the resources the profile's user can open.
func accessible(ctx context.Context, p *clientauth.Profile) ([]*werbotv1.AccessibleResource, error) {
	res, err := newClient(p, werbotv1connect.NewResourceServiceClient).
		ListAccessibleResources(ctx, connect.NewRequest(&werbotv1.ListAccessibleResourcesRequest{}))
	if err != nil {
		return nil, err
	}
	return res.Msg.GetResources(), nil
}

func lsCommand(env Env) *cobra.Command {
	return &cobra.Command{
		Use: "ls", Short: "List the resources you can open", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			_, p, err := loadProfile(env)
			if err != nil {
				return err
			}
			rs, err := accessible(cmd.Context(), p)
			if err != nil {
				return err
			}
			if len(rs) == 0 {
				_, err = fmt.Fprintln(env.Stdout, "No accessible resources")
				return err
			}
			w := tabwriter.NewWriter(env.Stdout, 0, 0, 2, ' ', 0)
			_, _ = fmt.Fprintln(w, "NAME\tTYPE\tKINDS\tGATEWAY")
			for _, r := range rs {
				_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", r.GetName(), r.GetType(), strings.Join(r.GetKinds(), ","), r.GetSshAddress())
			}
			return w.Flush()
		},
	}
}
