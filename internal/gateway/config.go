// Package gateway is the SSH gateway runtime: configuration, enrollment and identity.
package gateway

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"math"
	"net/netip"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

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
	SpoolMax      int64         // recording spool size cap in bytes
	MaxDuration   time.Duration // session length limit when the resource sets none; 0: no limit
	// InsecureEnroll allows an http:// ServerURL to a non-loopback host (flag only:
	// enrollment then sends the join token and the pool host key in cleartext).
	InsecureEnroll bool

	WSAddr         string         // empty: the WebSocket input is disabled
	WSTLSCert      string         // PEM file path; set together with WSTLSKey
	WSTLSKey       string         // PEM file path
	TrustedProxies []netip.Prefix // may set X-Forwarded-For on the WebSocket input only

	TunnelURL string // not from env: overrides the tunnel_url of enrollment (embedded gateway)
	// DenyLocalTargets (not from env) makes Session.Dial refuse loopback, link-local
	// and unspecified addresses: the embedded gateway runs inside the server process.
	DenyLocalTargets bool
	// DenyHosts (not from env) are the server's own hosts, its database: with
	// DenyLocalTargets every address they resolve to is refused too.
	DenyHosts []string
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
	envSpoolMax = "WERBOT_GATEWAY_SPOOL_MAX"
	envMaxDur   = "WERBOT_GATEWAY_MAX_DURATION"
)

// DefaultSpoolMax is the spool size cap when none is configured: 1 GiB.
const DefaultSpoolMax int64 = 1 << 30

// ConfigVars lists the WERBOT_GATEWAY_* variables in Config field order.
func ConfigVars() []string {
	return []string{envServer, envToken, envDataDir, envSSHAddr, envProxy, envName, envIdle, envHTTPAddr, envDrain, envMaxSess, envSpoolMax, envMaxDur, envWSAddr, envWSTLSCert, envWSTLSKey, envTrustedProxies}
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
	if c.MaxDuration, err = envDuration(envMaxDur, env[envMaxDur], 12*time.Hour); err != nil {
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
	spoolMax := fs.String("spool-max", orDefault(env[envSpoolMax], strconv.FormatInt(DefaultSpoolMax, 10)), "recording spool size cap: bytes or KiB/MiB/GiB")
	fs.DurationVar(&c.MaxDuration, "max-duration", c.MaxDuration, "session length limit when the resource sets none; 0 disables")
	fs.BoolVar(&c.InsecureEnroll, "insecure-enroll", false, "allow an http:// server URL to a non-loopback host (trusted lab networks only)")
	trusted := c.wsFlags(fs, env)
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
	if c.SpoolMax, err = parseByteSize(*spoolMax); err != nil {
		return c, fmt.Errorf("%s: %w", envSpoolMax, err)
	}
	if c.JoinToken == "" {
		c.JoinToken = env[envToken]
	}
	if err := c.checkWS(*trusted); err != nil {
		return c, err
	}
	return c, c.validate()
}

// validate checks the parsed settings.
func (c *Config) validate() error {
	if c.Name == "" {
		return fmt.Errorf("%s is empty and the OS hostname is unavailable", envName)
	}
	if c.MaxSessions < 1 || c.MaxSessions > maxSessionIDs {
		return fmt.Errorf("%s must be between 1 and %d", envMaxSess, maxSessionIDs)
	}
	if c.DrainTimeout < 0 {
		return fmt.Errorf("%s must be >= 0", envDrain)
	}
	if c.IdleTimeout < 0 {
		return fmt.Errorf("%s must be >= 0", envIdle)
	}
	if c.MaxDuration < 0 {
		return fmt.Errorf("%s must be >= 0", envMaxDur)
	}
	return nil
}

// checkServerURL allows an https:// server URL, and http:// only to a loopback
// host or with insecure: enrollment sends the join token and receives the pool
// host key, and the CA pin protects only the tunnel.
func checkServerURL(raw string, insecure bool) error {
	if raw == "" {
		return nil
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return fmt.Errorf("%s: want http(s)://host[:port]", envServer)
	}
	if u.Scheme == "https" || insecure || isLoopback(u.Hostname()) {
		return nil
	}
	return fmt.Errorf("%s: http:// sends the join token and the pool host key in cleartext; use https://, a loopback host or --insecure-enroll on a trusted network", envServer)
}

// isLoopback reports whether host is localhost, 127.0.0.0/8 or ::1.
func isLoopback(host string) bool {
	if host == "localhost" {
		return true
	}
	ip, err := netip.ParseAddr(host)
	return err == nil && ip.IsLoopback()
}

// parseByteSize parses a positive byte count with an optional KiB, MiB or GiB suffix.
func parseByteSize(s string) (int64, error) {
	mult := int64(1)
	for suffix, m := range map[string]int64{"KiB": 1 << 10, "MiB": 1 << 20, "GiB": 1 << 30} {
		if num, ok := strings.CutSuffix(s, suffix); ok {
			s, mult = num, m
			break
		}
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid size: want bytes or KiB/MiB/GiB")
	}
	if n <= 0 || n > math.MaxInt64/mult {
		return 0, errors.New("size must be positive and fit int64")
	}
	return n * mult, nil
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

// errNoIdentity means dataDir has never held a complete identity.
var errNoIdentity = errors.New("no instance identity")

// identityError reports why dataDir holds no usable instance identity: none was saved,
// its gateway.json is corrupt, or its client certificate does not load or has expired.
// nil means enrolled.
func identityError(dataDir string) error {
	if _, err := os.Stat(filepath.Join(dataDir, stateFile)); err != nil {
		return errNoIdentity
	}
	st, cs, err := loadIdentity(dataDir)
	if err != nil {
		return err
	}
	if st.GatewayID == uuid.Nil || st.InstanceID == uuid.Nil {
		return fmt.Errorf("%s has no gateway_id or instance_id", stateFile)
	}
	if exp := cs.Leaf().NotAfter; !time.Now().Before(exp) {
		return fmt.Errorf("gateway certificate expired at %s", exp.UTC().Format(time.RFC3339))
	}
	return nil
}

// Enrolled reports whether dataDir holds a usable instance identity: a corrupt
// gateway.json or an expired or unloadable certificate counts as not enrolled, so the
// instance enrolls again.
func Enrolled(dataDir string) bool { return identityError(dataDir) == nil }
