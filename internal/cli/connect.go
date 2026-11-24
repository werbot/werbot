package cli

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"path/filepath"
	"slices"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/crypto/ssh"

	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
	"github.com/werbot/werbot/internal/clientauth"
	"github.com/werbot/werbot/internal/connectcli"
	"github.com/werbot/werbot/internal/vnet"
)

// pendingGateway stands for the gateway until the resources tell it (RFC 6761 reserved TLD).
const pendingGateway = "pending.invalid"

// runConnect is replaced in tests.
var runConnect = connectcli.Run

// connectCommand keeps the flag set of werbot connect: it parses its own args.
func connectCommand(env Env) *cobra.Command {
	return &cobra.Command{
		Use:                "connect [flags] [<local_port>:]<resource>[:<target_port>] ... | connect --vnet [--vnet-prefix <cidr>]",
		Short:              "Open local ports to resources through the gateway",
		DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			on, prefix, args, err := vnetFlags(args)
			switch {
			case err != nil:
				return err
			case on:
				return connectVNet(cmd.Context(), env, prefix)
			}
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
		if cfg.Gateway, cfg.WSURL, err = sharedGateway(rs, cfg.Specs); err != nil {
			return cfg, err
		}
	}
	if ownHosts {
		cfg.KnownHosts = filepath.Join(clientauth.Dir(home, p.Server), "known_hosts")
		err = writeKnownHosts(cfg.KnownHosts, rs)
	}
	return cfg, err
}

// sharedGateway is the one pool of the resources in specs: its ssh_address
// and its WebSocket URL.
func sharedGateway(rs []*werbotv1.AccessibleResource, specs []connectcli.Spec) (sshAddress, wsURL string, err error) {
	var eps []connectcli.Endpoint
	var names []string
	for _, sp := range specs {
		r, err := find(rs, sp.Resource)
		if err != nil {
			return "", "", err
		}
		if _, _, err := gatewayAddr(r); err != nil && !wsOnly(r) {
			return "", "", err
		}
		if !slices.Contains(eps, endpoint(r)) {
			eps = append(eps, endpoint(r))
		}
		names = append(names, r.GetName())
	}
	if len(eps) > 1 {
		return "", "", fmt.Errorf("resources %s are behind different gateways; use --gateway", strings.Join(names, ", "))
	}
	return eps[0].SSHAddress, eps[0].WSURL, nil
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

// runVNet is replaced in tests.
var runVNet = vnet.Run

// connectVNet runs werbot connect --vnet with the profile of the user who
// ran sudo, when under sudo.
func connectVNet(ctx context.Context, env Env, prefix netip.Prefix) error {
	if home, ok := vnet.SudoHome(env.Getenv); ok {
		getenv := env.Getenv
		env.Getenv = func(k string) string {
			if k == "WERBOT_HOME" {
				return home
			}
			return getenv(k)
		}
	}
	home, p, err := loadProfile(env)
	if err != nil {
		return err
	}
	return runVNet(ctx, vnet.Options{Profile: p, Home: home, Prefix: prefix})
}

// vnetFlags takes --vnet and --vnet-prefix out of args (before "--"); the
// rest goes to connectConfig. --vnet takes no other arguments.
func vnetFlags(args []string) (on bool, prefix netip.Prefix, rest []string, err error) {
	on, value, rest := splitVNetFlags(args)
	prefix, err = vnetPrefix(value)
	switch {
	case err != nil:
	case value != nil && !on:
		err = errors.New("--vnet-prefix needs --vnet")
	case on && len(rest) > 0:
		err = errors.New("--vnet does not take resources")
	}
	return on, prefix, rest, err
}

// splitVNetFlags finds -vnet and -vnet-prefix[=]<value> (one or two dashes);
// value is nil without -vnet-prefix and "" when its value is missing.
func splitVNetFlags(args []string) (on bool, value *string, rest []string) {
	for i := 0; i < len(args); i++ {
		name, val, hasVal := vnetArg(args[i])
		switch {
		case args[i] == "--":
			return on, value, append(rest, args[i:]...)
		case name == "":
			rest = append(rest, args[i])
		case name == "vnet":
			on = true
		default:
			if !hasVal && i+1 < len(args) {
				i++
				val = args[i]
			}
			value = &val
		}
	}
	return on, value, rest
}

// vnetArg splits a -vnet or -vnet-prefix[=<value>] argument; name is "" for
// any other argument (-vnet=<value> included: connectConfig refuses it).
func vnetArg(a string) (name, val string, hasVal bool) {
	name, val, hasVal = strings.Cut(strings.TrimLeft(a, "-"), "=")
	if !strings.HasPrefix(a, "-") || name != "vnet-prefix" && (name != "vnet" || hasVal) {
		return "", "", false
	}
	return name, val, hasVal
}

// vnetPrefix is the --vnet-prefix value, or vnet.DefaultPrefix when nil.
func vnetPrefix(value *string) (netip.Prefix, error) {
	if value == nil {
		return netip.MustParsePrefix(vnet.DefaultPrefix), nil
	}
	p, err := netip.ParsePrefix(*value)
	if err != nil {
		return p, fmt.Errorf("--vnet-prefix: %w", err)
	}
	return p, nil
}
