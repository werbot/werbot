// Package gateway is the SSH gateway runtime: configuration, enrollment and identity.
package gateway

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/werbot/werbot/internal/platform/metrics"
)

// Config is the gateway process configuration. Flags override WERBOT_GATEWAY_* env.
type Config struct {
	ServerURL     string
	JoinToken     string
	DataDir       string
	SSHAddr       string
	ProxyProtocol bool
	Name          string // instance hostname in the pool
	IdleTimeout   time.Duration
	HTTPAddr      string // empty: disabled
	DrainTimeout  time.Duration
	MaxSessions   int

	TunnelURL string            // not from env: overrides the tunnel_url of enrollment (embedded gateway)
	Recording RecordingFactory  // not from env: nil means NopRecording
	Metrics   *metrics.Registry // not from env: nil means a private registry
}

const (
	envServer   = "WERBOT_GATEWAY_SERVER_URL"
	envToken    = "WERBOT_GATEWAY_JOIN_TOKEN" //nolint:gosec // env var name, not a credential
	envDataDir  = "WERBOT_GATEWAY_DATA_DIR"
	envSSHAddr  = "WERBOT_GATEWAY_SSH_ADDR"
	envProxy    = "WERBOT_GATEWAY_PROXY_PROTOCOL"
	envName     = "WERBOT_GATEWAY_NAME"
	envIdle     = "WERBOT_GATEWAY_IDLE_TIMEOUT"
	envHTTPAddr = "WERBOT_GATEWAY_HTTP_ADDR"
	envDrain    = "WERBOT_GATEWAY_DRAIN_TIMEOUT"
	envMaxSess  = "WERBOT_GATEWAY_MAX_SESSIONS"
)

// ConfigVars lists the WERBOT_GATEWAY_* variables in Config field order.
func ConfigVars() []string {
	return []string{envServer, envToken, envDataDir, envSSHAddr, envProxy, envName, envIdle, envHTTPAddr, envDrain, envMaxSess}
}

func orDefault(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

// LoadConfig reads env through getenv, then applies args (flags win).
func LoadConfig(args []string, getenv func(string) string) (Config, error) {
	var c Config
	var err error
	// every variable is read unconditionally
	env := map[string]string{}
	for _, k := range ConfigVars() {
		env[k] = getenv(k)
	}
	if c.ProxyProtocol, err = envBool(envProxy, env[envProxy]); err != nil {
		return c, err
	}
	if c.IdleTimeout, err = envDuration(envIdle, env[envIdle], 5*time.Minute); err != nil {
		return c, err
	}
	if c.DrainTimeout, err = envDuration(envDrain, env[envDrain], 30*time.Second); err != nil {
		return c, err
	}
	c.MaxSessions = 10000
	if s := env[envMaxSess]; s != "" {
		if c.MaxSessions, err = strconv.Atoi(s); err != nil {
			return c, fmt.Errorf("%s: %w", envMaxSess, err)
		}
	}
	name := env[envName]
	if name == "" {
		name, _ = os.Hostname() // an error leaves name empty, rejected below
	}

	fs := flag.NewFlagSet("gateway", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.StringVar(&c.ServerURL, "server", env[envServer], "werbot server URL")
	// no env default: --help would print the secret
	fs.StringVar(&c.JoinToken, "join-token", "", "pool join token (default $"+envToken+")")
	fs.StringVar(&c.DataDir, "data-dir", orDefault(env[envDataDir], "./gateway-data"), "identity directory")
	fs.StringVar(&c.SSHAddr, "ssh-addr", orDefault(env[envSSHAddr], ":2222"), "SSH listen address")
	fs.BoolVar(&c.ProxyProtocol, "proxy-protocol", c.ProxyProtocol, "accept PROXY protocol")
	fs.StringVar(&c.Name, "name", name, "instance hostname")
	fs.DurationVar(&c.IdleTimeout, "idle-timeout", c.IdleTimeout, "session idle timeout")
	fs.StringVar(&c.HTTPAddr, "http-addr", env[envHTTPAddr], "readyz/metrics address; empty disables")
	fs.DurationVar(&c.DrainTimeout, "drain-timeout", c.DrainTimeout, "shutdown drain timeout")
	fs.IntVar(&c.MaxSessions, "max-sessions", c.MaxSessions, "concurrent session limit")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) { // parse errors are returned, help is printed
			fs.SetOutput(os.Stderr)
			fs.Usage()
		}
		return c, fmt.Errorf("gateway flags: %w", err)
	}
	if fs.NArg() > 0 {
		return c, fmt.Errorf("unexpected arguments: %q", fs.Args())
	}
	if c.JoinToken == "" {
		c.JoinToken = env[envToken]
	}
	if c.Name == "" {
		return c, fmt.Errorf("%s is empty and the OS hostname is unavailable", envName)
	}
	if c.MaxSessions < 1 || c.MaxSessions > maxSessionIDs {
		return c, fmt.Errorf("%s must be between 1 and %d", envMaxSess, maxSessionIDs)
	}
	if c.DrainTimeout < 0 {
		return c, fmt.Errorf("%s must be >= 0", envDrain)
	}
	if c.IdleTimeout < 0 {
		return c, fmt.Errorf("%s must be >= 0", envIdle)
	}
	return c, nil
}

func envBool(key, v string) (bool, error) {
	if v == "" {
		return false, nil
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return false, fmt.Errorf("%s: %w", key, err)
	}
	return b, nil
}

func envDuration(key, v string, def time.Duration) (time.Duration, error) {
	if v == "" {
		return def, nil
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", key, err)
	}
	return d, nil
}

var joinTokenRE = regexp.MustCompile(`^wjt_([A-Za-z0-9_-]{43})\.([0-9a-f]{64})$`)

// ParseJoinToken validates wjt_<secret>.<CA fingerprint> (surrounding whitespace is ignored)
// and returns the pinned CA fingerprint. The secret is sent as is, so it is not returned.
// Errors never contain the token.
func ParseJoinToken(s string) (caFingerprint string, err error) {
	m := joinTokenRE.FindStringSubmatch(strings.TrimSpace(s))
	if m == nil {
		return "", errors.New("malformed join token: want wjt_<43 chars>.<64 hex>")
	}
	return m[2], nil
}

// Enrolled reports whether dataDir holds an instance identity.
func Enrolled(dataDir string) bool {
	_, err := os.Stat(filepath.Join(dataDir, stateFile))
	return err == nil
}
