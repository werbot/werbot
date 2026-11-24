// Package connectcli is the werbot connect client: it opens local TCP and UDP
// ports that reach Werbot resources through the gateway (SSH, else WebSocket).
// It knows only SSH and the resource@werbot.com answer, not the server or its domains.
package connectcli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"golang.org/x/crypto/ssh"
)

const (
	envGateway = "WERBOT_CONNECT_GATEWAY"
	envLogin   = "WERBOT_CONNECT_LOGIN"

	defaultGatewayPort = "2222"
	usageText          = "usage: werbot connect [flags] [<local_port>:]<resource>[:<target_port>] ..."
)

// ConfigVars lists the WERBOT_CONNECT_* variables in Config field order.
func ConfigVars() []string { return []string{envGateway, envLogin} }

// Config is what werbot connect needs: the gateway host:port and/or its
// WebSocket URL, the Werbot login, an optional private key file, the
// known_hosts file and one Spec per resource.
// Signer, when set, replaces the agent and the key file; it is called for every
// connection to the gateway, so a renewed certificate is picked up on a redial.
type Config struct {
	Gateway, WSURL, Login, Identity, KnownHosts string
	Specs                                       []Spec
	Signer                                      func(ctx context.Context) (ssh.Signer, error)
	// OpenBrowser opens an MFA page (nil: the prompt shows the URL only); Stdin
	// answers MFA prompts (nil: os.Stdin).
	OpenBrowser func(string) error
	Stdin       io.Reader
}

// Spec is one [<local_port>:]<resource>[:<target_port>] argument; 0 means not given.
type Spec struct {
	LocalPort  uint16
	Resource   string
	TargetPort uint16
}

// ParseSpec parses [<local_port>:]<resource>[:<target_port>]. With two parts a
// numeric first part is the local port, otherwise the second is the target port.
func ParseSpec(s string) (Spec, error) {
	parts := strings.Split(s, ":")
	var sp Spec
	var err error
	switch len(parts) {
	case 1:
		sp.Resource = parts[0]
	case 2:
		if _, nerr := strconv.Atoi(parts[0]); nerr == nil {
			sp.Resource = parts[1]
			sp.LocalPort, err = parsePort(s, parts[0])
		} else {
			sp.Resource = parts[0]
			sp.TargetPort, err = parsePort(s, parts[1])
		}
	case 3:
		sp.Resource = parts[1]
		if sp.LocalPort, err = parsePort(s, parts[0]); err == nil {
			sp.TargetPort, err = parsePort(s, parts[2])
		}
	default:
		return Spec{}, fmt.Errorf("%q: want [<local_port>:]<resource>[:<target_port>]", s)
	}
	if err != nil {
		return Spec{}, err
	}
	if sp.Resource == "" {
		return Spec{}, fmt.Errorf("%q: the resource name is empty", s)
	}
	return sp, nil
}

func parsePort(spec, s string) (uint16, error) {
	n, err := strconv.ParseUint(s, 10, 16)
	if err != nil || n == 0 {
		return 0, fmt.Errorf("%q: port %q must be 1-65535", spec, s)
	}
	return uint16(n), nil
}

// LoadConfig reads the WERBOT_CONNECT_* variables through getenv, then applies
// args (flags win) and parses the remaining arguments as Specs. The gateway and
// the login are required; a ws:// or wss:// gateway is the WebSocket URL, a
// host's port defaults to 2222; known_hosts defaults to ~/.ssh/known_hosts.
func LoadConfig(args []string, getenv func(string) string) (Config, error) {
	var c Config
	fs := flag.NewFlagSet("connect", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.StringVar(&c.Gateway, "gateway", getenv(envGateway), "gateway host[:port] (port 2222 by default) or ws[s]://host[:port][/path]")
	fs.StringVar(&c.Login, "login", getenv(envLogin), "Werbot login")
	fs.StringVar(&c.Identity, "i", "", "private key file (default: ssh-agent from SSH_AUTH_SOCK, else ~/.ssh/id_ed25519)")
	fs.StringVar(&c.KnownHosts, "known-hosts", "", "known_hosts file (default ~/.ssh/known_hosts)")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) { // parse errors are returned, help is printed
			fs.SetOutput(os.Stderr)
			_, _ = fmt.Fprintln(os.Stderr, usageText)
			fs.PrintDefaults()
		}
		return c, fmt.Errorf("connect flags: %w", err)
	}
	switch {
	case c.Gateway == "":
		return c, fmt.Errorf("the gateway is required: set --gateway or %s", envGateway)
	case c.Login == "":
		return c, fmt.Errorf("the login is required: set --login or %s", envLogin)
	case fs.NArg() == 0:
		return c, errors.New(usageText) //nolint:staticcheck // a usage line ends with "..."
	}
	if err := c.parseGateway(); err != nil {
		return c, err
	}
	for _, a := range fs.Args() {
		sp, err := ParseSpec(a)
		if err != nil {
			return c, err
		}
		c.Specs = append(c.Specs, sp)
	}
	if c.KnownHosts == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return c, fmt.Errorf("known_hosts: %w; set --known-hosts", err)
		}
		c.KnownHosts = filepath.Join(home, ".ssh", "known_hosts")
	}
	return c, nil
}

// parseGateway moves a ws:// or wss:// gateway to WSURL; a host[:port] gets
// port 2222 when it has none.
func (c *Config) parseGateway() error {
	if strings.Contains(c.Gateway, "://") {
		u, err := url.Parse(c.Gateway)
		if err != nil || u.Host == "" || u.User != nil || (u.Scheme != "ws" && u.Scheme != "wss") {
			return errors.New("gateway: want ws://host[:port][/path] or wss://host[:port][/path]")
		}
		c.WSURL, c.Gateway = c.Gateway, ""
		return nil
	}
	c.Gateway = withPort(c.Gateway, defaultGatewayPort)
	_, port, err := net.SplitHostPort(c.Gateway)
	if err != nil {
		return fmt.Errorf("gateway %q: %w", c.Gateway, err)
	}
	if _, err := parsePort(c.Gateway, port); err != nil {
		return fmt.Errorf("gateway: %w", err)
	}
	return nil
}
