package cli

import (
	"fmt"
	"strconv"
	"text/tabwriter"

	"connectrpc.com/connect"
	"github.com/spf13/cobra"

	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
	"github.com/werbot/werbot/gen/werbot/v1/werbotv1connect"
)

func newAgentsCommand(env Env) *cobra.Command {
	cmd := &cobra.Command{
		Use: "agents", Short: "List the agents you may run", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			_, p, err := loadProfile(env)
			if err != nil {
				return err
			}
			res, err := werbotv1connect.NewAgentServiceClient(p.HTTPClient(), p.Server).
				ListAgents(cmd.Context(), connect.NewRequest(&werbotv1.ListAgentsRequest{}))
			if err != nil {
				return err
			}
			w := tabwriter.NewWriter(env.Stdout, 0, 0, 2, ' ', 0)
			_, _ = fmt.Fprintln(w, "NAME\tSTATUS\tMAX ACTIONS\tMAX DURATION\tMAX CONCURRENT\tACTIVE RUNS")
			for _, a := range res.Msg.GetAgents() {
				l, d := a.GetLimits(), a.GetLimits().GetMaxDuration().AsDuration()
				_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%d\n", a.GetName(), a.GetStatus(),
					orDefault(l.GetMaxActions() == 0, strconv.Itoa(int(l.GetMaxActions()))), orDefault(d == 0, d.String()),
					orDefault(l.GetMaxConcurrent() == 0, strconv.Itoa(int(l.GetMaxConcurrent()))), a.GetActiveRuns())
			}
			return w.Flush()
		},
	}
	cmd.AddCommand(newAgentProfileCommand(env))
	return cmd
}

// orDefault is text, or "default" when the limit is unset (0).
func orDefault(unset bool, text string) string {
	if unset {
		return "default"
	}
	return text
}
