package cli

import (
	"context"
	"errors"
	"fmt"
	"net"
	"path/filepath"
	"slices"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/crypto/ssh"

	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
	"github.com/werbot/werbot/internal/clientauth"
	"github.com/werbot/werbot/internal/connectcli"
)

// pendingGateway stands for the gateway until the resources tell it (RFC 6761 reserved TLD).
const pendingGateway = "pending.invalid"

// runConnect is replaced in tests.
var runConnect = connectcli.Run

// connectCommand keeps the flag set of werbot connect: it parses its own args.
func connectCommand(env Env) *cobra.Command {
	return &cobra.Command{
		Use:                "connect [flags] [<local_port>:]<resource>[:<target_port>] ...",
		Short:              "Open local ports to resources through the gateway",
		DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := connectConfig(cmd.Context(), env, args)
			if err != nil {
				return err
			}
			cfg.OpenBrowser, cfg.Stdin = env.OpenBrowser, env.Stdin
			return runConnect(cmd.Context(), cfg, env.Stdout, env.Stderr)
		},
	}
}

// connectConfig is LoadConfig, filled from the profile when there is one: the
// login, the gateway of the resources, the profile certificate and known_hosts.
func connectConfig(ctx context.Context, env Env, args []string) (connectcli.Config, error) {
	home, p, err := loadProfile(env)
	if errors.Is(err, clientauth.ErrNotLoggedIn) {
		return connectcli.LoadConfig(args, env.Getenv)
	}
	if err != nil {
		return connectcli.Config{}, err
	}
	cfg, err := connectcli.LoadConfig(args, func(k string) string {
		switch v := env.Getenv(k); {
		case v != "":
			return v
		case k == "WERBOT_CONNECT_LOGIN":
			return p.Login
		case k == "WERBOT_CONNECT_GATEWAY":
			return pendingGateway
		}
		return ""
	})
	if err != nil {
		return cfg, err
	}
	if !hasFlag(args, "i") {
		cfg.Signer = func(ctx context.Context) (ssh.Signer, error) { return p.Signer(ctx, home) }
	}
	pending, ownHosts := hostOf(cfg.Gateway) == pendingGateway, !hasFlag(args, "known-hosts")
	if !pending && !ownHosts {
		return cfg, nil
	}
	rs, err := accessible(ctx, p)
	if err != nil {
		return cfg, err
	}
	if pending {
		if cfg.Gateway, err = sharedGateway(rs, cfg.Specs); err != nil {
			return cfg, err
		}
	}
	if ownHosts {
		cfg.KnownHosts = filepath.Join(clientauth.Dir(home, p.Server), "known_hosts")
		err = writeKnownHosts(cfg.KnownHosts, rs)
	}
	return cfg, err
}

// sharedGateway is the one ssh_address of the resources in specs.
func sharedGateway(rs []*werbotv1.AccessibleResource, specs []connectcli.Spec) (string, error) {
	var addrs, names []string
	for _, sp := range specs {
		r, err := find(rs, sp.Resource)
		if err != nil {
			return "", err
		}
		if _, _, err := gatewayAddr(r); err != nil {
			return "", err
		}
		if !slices.Contains(addrs, r.GetSshAddress()) {
			addrs = append(addrs, r.GetSshAddress())
		}
		names = append(names, r.GetName())
	}
	if len(addrs) > 1 {
		return "", fmt.Errorf("resources %s are behind different gateways; use --gateway", strings.Join(names, ", "))
	}
	return addrs[0], nil
}

func hostOf(addr string) string {
	h, _, _ := net.SplitHostPort(addr)
	return h
}

// hasFlag reports whether args before "--" hold -name or --name, with or without "=value".
func hasFlag(args []string, name string) bool {
	for _, a := range args {
		if a == "--" {
			return false
		}
		if a, ok := strings.CutPrefix(a, "-"); ok {
			a = strings.TrimPrefix(a, "-")
			if a == name || strings.HasPrefix(a, name+"=") {
				return true
			}
		}
	}
	return false
}
