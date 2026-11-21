package cli

import (
	"context"
	"errors"
	"fmt"
	"text/tabwriter"
	"time"

	"connectrpc.com/connect"
	"github.com/spf13/cobra"
	"google.golang.org/protobuf/types/known/durationpb"

	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
	"github.com/werbot/werbot/gen/werbot/v1/werbotv1connect"
	"github.com/werbot/werbot/internal/clientauth"
)

// requestPollInterval is how often werbot request asks whether a pending request was decided; tests lower it.
var requestPollInterval = 5 * time.Second

// cancelTimeout bounds the Cancel call sent after Ctrl-C.
const cancelTimeout = 5 * time.Second

func requestClient(p *clientauth.Profile) werbotv1connect.AccessRequestServiceClient {
	return werbotv1connect.NewAccessRequestServiceClient(p.HTTPClient(), p.Server)
}

// serverText turns an API error into the server's own message; an Unauthenticated
// answer stays as it is so the command ends with ErrLoginRequired.
func serverText(err error) error {
	if ce := new(connect.Error); errors.As(err, &ce) && ce.Code() != connect.CodeUnauthenticated {
		return errors.New(ce.Message())
	}
	return err
}

func newRequestCommand(env Env) *cobra.Command {
	var (
		dur        time.Duration
		reason     string
		breakGlass bool
	)
	cmd := &cobra.Command{
		Use: "request <resource>", Short: "Ask for access to a resource and wait for the decision", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if breakGlass && reason == "" {
				return errors.New("--break-glass needs --reason")
			}
			_, p, err := loadProfile(env)
			if err != nil {
				return err
			}
			rs, err := listResources(cmd.Context(), p)
			if err != nil {
				return err
			}
			r, err := find(rs, args[0])
			if err != nil || !r.GetRequestable() {
				return fmt.Errorf("resource %s is not requestable", args[0])
			}
			in := &werbotv1.CreateRequestRequest{ResourceId: r.GetId(), Reason: reason, BreakGlass: breakGlass}
			if dur > 0 {
				in.Duration = durationpb.New(dur)
			}
			c := requestClient(p)
			res, err := c.CreateRequest(cmd.Context(), connect.NewRequest(in))
			if err != nil {
				return serverText(err)
			}
			return awaitDecision(cmd.Context(), env, c, r.GetName(), res.Msg.GetRequest())
		},
	}
	cmd.Flags().DurationVar(&dur, "for", 0, "how long the access should last (default: the organization default)")
	cmd.Flags().StringVar(&reason, "reason", "", "why you need the access")
	cmd.Flags().BoolVar(&breakGlass, "break-glass", false, "emergency access without approval, reviewed afterwards (needs --reason)")
	return cmd
}

// awaitDecision prints the outcome of ar, polling while it is pending.
// Ctrl-C cancels the request and ends with success, as werbot connect does.
func awaitDecision(ctx context.Context, env Env, c werbotv1connect.AccessRequestServiceClient, resource string, ar *werbotv1.AccessRequest) error {
	if ar.GetStatus() == "pending" {
		if _, err := fmt.Fprintf(env.Stdout, "Request %s is waiting for approval: %s\n", ar.GetId(), ar.GetUrl()); err != nil {
			return err
		}
	}
	tick := time.NewTicker(requestPollInterval)
	defer tick.Stop()
	for ar.GetStatus() == "pending" {
		select {
		case <-ctx.Done():
			cctx, stop := context.WithTimeout(context.WithoutCancel(ctx), cancelTimeout)
			defer stop()
			if _, err := c.Cancel(cctx, connect.NewRequest(&werbotv1.CancelRequest{Id: ar.GetId()})); err != nil {
				return serverText(err)
			}
			_, err := fmt.Fprintf(env.Stdout, "Request %s canceled\n", ar.GetId())
			return err
		case <-tick.C:
		}
		res, err := c.GetRequest(ctx, connect.NewRequest(&werbotv1.GetRequestRequest{Id: ar.GetId()}))
		if err != nil {
			if ctx.Err() != nil {
				continue // Ctrl-C during the poll: the next select cancels
			}
			return serverText(err)
		}
		ar = res.Msg.GetRequest()
	}
	switch ar.GetStatus() {
	case "approved":
		_, err := fmt.Fprintf(env.Stdout, "Access to %s granted until %s\n", resource, ar.GetExpiresAt().AsTime().UTC().Format(time.RFC3339))
		return err
	case "denied":
		return fmt.Errorf("Request %s was denied: %s", ar.GetId(), ar.GetComment()) //nolint:staticcheck // printed to the user as a sentence
	default:
		return fmt.Errorf("Request %s %s", ar.GetId(), ar.GetStatus()) //nolint:staticcheck // "expired" (or "canceled" elsewhere)
	}
}

func newRequestsCommand(env Env) *cobra.Command {
	var toApprove bool
	cmd := &cobra.Command{
		Use: "requests", Short: "List access requests", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			_, p, err := loadProfile(env)
			if err != nil {
				return err
			}
			scope := werbotv1.RequestScope_REQUEST_SCOPE_MINE
			if toApprove {
				scope = werbotv1.RequestScope_REQUEST_SCOPE_TO_APPROVE
			}
			res, err := requestClient(p).ListRequests(cmd.Context(), connect.NewRequest(&werbotv1.ListRequestsRequest{Scope: scope}))
			if err != nil {
				return serverText(err)
			}
			w := tabwriter.NewWriter(env.Stdout, 0, 0, 2, ' ', 0)
			_, _ = fmt.Fprintln(w, "ID\tSTATUS\tRESOURCE\tREQUESTER\tDURATION\tEXPIRES\tREASON")
			for _, r := range res.Msg.GetRequests() {
				// the decision deadline of a pending request, the grant end of an approved access request
				expires := "-"
				if r.GetExpiresAt() != nil && (r.GetStatus() == "pending" || r.GetStatus() == "approved" && r.GetKind() == "access") {
					expires = r.GetExpiresAt().AsTime().UTC().Format(time.RFC3339)
				}
				_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n", r.GetId(), r.GetStatus(), r.GetResourceName(),
					r.GetRequesterLogin(), r.GetDuration().AsDuration(), expires, r.GetReason())
			}
			return w.Flush()
		},
	}
	cmd.Flags().BoolVar(&toApprove, "to-approve", false, "only pending requests you can decide")
	return cmd
}

func newDecideCommand(env Env, approve bool) *cobra.Command {
	verb, done := "deny", "denied"
	if approve {
		verb, done = "approve", "approved"
	}
	var comment string
	cmd := &cobra.Command{
		Use: verb + " <id>", Short: verb + " an access request", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			_, p, err := loadProfile(env)
			if err != nil {
				return err
			}
			c := requestClient(p)
			if approve {
				_, err = c.Approve(cmd.Context(), connect.NewRequest(&werbotv1.ApproveRequest{Id: args[0], Comment: comment}))
			} else {
				_, err = c.Deny(cmd.Context(), connect.NewRequest(&werbotv1.DenyRequest{Id: args[0], Comment: comment}))
			}
			if err != nil {
				return serverText(err)
			}
			_, err = fmt.Fprintf(env.Stdout, "Request %s %s\n", args[0], done)
			return err
		},
	}
	cmd.Flags().StringVar(&comment, "comment", "", "a note for the requester")
	return cmd
}
