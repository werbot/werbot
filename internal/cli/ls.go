package cli

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"text/tabwriter"

	"connectrpc.com/connect"
	"github.com/spf13/cobra"

	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
	"github.com/werbot/werbot/gen/werbot/v1/werbotv1connect"
	"github.com/werbot/werbot/internal/clientauth"
)

// listResources lists the resources the profile's user can open or request.
func listResources(ctx context.Context, p *clientauth.Profile) ([]*werbotv1.AccessibleResource, error) {
	res, err := werbotv1connect.NewResourceServiceClient(p.HTTPClient(), p.Server).
		ListAccessibleResources(ctx, connect.NewRequest(&werbotv1.ListAccessibleResourcesRequest{}))
	if err != nil {
		return nil, err
	}
	return res.Msg.GetResources(), nil
}

// accessible lists the resources the profile's user can open right now.
func accessible(ctx context.Context, p *clientauth.Profile) ([]*werbotv1.AccessibleResource, error) {
	rs, err := listResources(ctx, p)
	if err != nil {
		return nil, err
	}
	return slices.DeleteFunc(rs, func(r *werbotv1.AccessibleResource) bool { return !r.GetAllowed() }), nil
}

func lsCommand(env Env) *cobra.Command {
	return &cobra.Command{
		Use: "ls", Short: "List the resources you can open", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			_, p, err := loadProfile(env)
			if err != nil {
				return err
			}
			rs, err := listResources(cmd.Context(), p)
			if err != nil {
				return err
			}
			if len(rs) == 0 {
				_, err = fmt.Fprintln(env.Stdout, "No accessible resources")
				return err
			}
			w := tabwriter.NewWriter(env.Stdout, 0, 0, 2, ' ', 0)
			_, _ = fmt.Fprintln(w, "NAME\tTYPE\tKINDS\tGATEWAY\tACCESS")
			for _, r := range rs {
				access := "request"
				if r.GetAllowed() {
					access = "yes"
				}
				_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", r.GetName(), r.GetType(), strings.Join(r.GetKinds(), ","), r.GetSshAddress(), access)
			}
			return w.Flush()
		},
	}
}
