package cli

import (
	"context"
	"strings"

	"github.com/spf13/cobra"

	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
	"github.com/werbot/werbot/internal/connectcli"
)

func newProxyCommand(env Env) *cobra.Command {
	return &cobra.Command{
		Use:   "proxy <resource | host[:port] | ws[s]://host[:port][/path]>",
		Short: "Connect stdin and stdout to a gateway, as an OpenSSH ProxyCommand",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ep, err := resolveProxyTarget(cmd.Context(), args[0], func(ctx context.Context) ([]*werbotv1.AccessibleResource, error) {
				_, p, err := loadProfile(env)
				if err != nil {
					return nil, err
				}
				return listResources(ctx, p)
			})
			if err != nil {
				return err
			}
			return connectcli.Proxy(cmd.Context(), ep, env.Stdin, env.Stdout)
		},
	}
}

// resolveProxyTarget is the gateway of arg: a ws:// or wss:// URL as it is
// (list, which needs the profile, is not called), the pool of the accessible
// resource called arg, else arg as host[:port] (port 2222 by default).
func resolveProxyTarget(ctx context.Context, arg string, list func(context.Context) ([]*werbotv1.AccessibleResource, error)) (connectcli.Endpoint, error) {
	if strings.HasPrefix(arg, "ws://") || strings.HasPrefix(arg, "wss://") {
		return connectcli.Endpoint{WSURL: arg}, nil
	}
	rs, err := list(ctx)
	if err != nil {
		return connectcli.Endpoint{}, err
	}
	if r, err := find(rs, arg); err == nil {
		return endpoint(r), nil
	}
	return connectcli.Endpoint{SSHAddress: connectcli.HostKeyAddr(connectcli.Endpoint{SSHAddress: arg})}, nil
}

// endpoint is the gateway pool of r.
func endpoint(r *werbotv1.AccessibleResource) connectcli.Endpoint {
	return connectcli.Endpoint{SSHAddress: r.GetSshAddress(), WSURL: r.GetWsUrl()}
}

// wsOnly reports that the pool of r is reachable over WebSocket only.
func wsOnly(r *werbotv1.AccessibleResource) bool {
	return r.GetSshAddress() == "" && r.GetWsUrl() != ""
}

// proxyCommand is the OpenSSH ProxyCommand "<exe> proxy <arg>": both words
// quoted for the shell of goos and every % doubled, as ssh expands %-tokens.
// A Windows path never holds '"'; in a URL it is percent-encoded instead.
func proxyCommand(exe, arg, goos string) string {
	quote := shellQuote
	if goos == "windows" {
		quote = func(s string) string { return `"` + strings.ReplaceAll(s, `"`, "%22") + `"` }
	}
	return strings.ReplaceAll(quote(exe)+" proxy "+quote(arg), "%", "%%")
}
