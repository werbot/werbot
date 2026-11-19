// Package config loads process configuration from WERBOT_* environment variables.
package config

import (
	"encoding/base64"
	"errors"
	"fmt"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
)

// SMTP is the outgoing mail configuration; an empty Host selects the log mailer.
type SMTP struct {
	Host               string
	Port               int
	Username, Password string
	From               string
}

// RecordingsS3 selects the recordings store: an empty Bucket means the file system under DataDir/recordings.
type RecordingsS3 struct {
	Endpoint  string // WERBOT_RECORDINGS_S3_ENDPOINT; "" -> standard AWS
	Bucket    string // WERBOT_RECORDINGS_S3_BUCKET; "" -> file system DataDir/recordings
	Region    string // WERBOT_RECORDINGS_S3_REGION; "us-east-1" when Bucket is set and Region empty
	AccessKey string // WERBOT_RECORDINGS_S3_ACCESS_KEY
	SecretKey string // WERBOT_RECORDINGS_S3_SECRET_KEY; both keys or neither
}

const secretKeyLen = 32

// DefaultGeoIPURL is the monthly DB-IP Country Lite database.
const DefaultGeoIPURL = "https://download.db-ip.com/free/dbip-country-lite-%s.mmdb.gz"

// Config is the validated server configuration.
type Config struct {
	Env         string // WERBOT_ENV: "production" (default) | "development"
	HTTPAddr    string // WERBOT_HTTP_ADDR, default ":8080"
	DatabaseURL string // WERBOT_DATABASE_URL, required
	LogLevel    string // WERBOT_LOG_LEVEL: debug|info|warn|error, default "info"
	LogFormat   string // WERBOT_LOG_FORMAT: json|text, default "json"
	MetricsAddr string // WERBOT_METRICS_ADDR, default "" (metrics disabled)
	WebEmbedded bool   // WERBOT_WEB_EMBEDDED, default true

	PublicURL          string         // WERBOT_PUBLIC_URL, required: http(s) address of the server, no trailing "/"
	WebURL             string         // WERBOT_WEB_URL: UI address used in email links; empty -> PublicURL
	TrustedProxies     []netip.Prefix // WERBOT_TRUSTED_PROXIES: comma-separated CIDRs; empty -> nil
	SMTP               SMTP           // WERBOT_SMTP_*: HOST empty -> log mailer; PORT default 587; FROM required with HOST
	SecretKey          []byte         // WERBOT_SECRET_KEY, required: base64, 32 bytes
	PreviousSecretKeys [][]byte       // WERBOT_SECRET_KEY_PREVIOUS: comma-separated base64 keys, 32 bytes each

	DataDir     string // WERBOT_DATA_DIR, default "./data": embedded gateway state
	GatewayAddr string // WERBOT_GATEWAY_ADDR, default ":8443": mTLS listener of the gateway tunnel
	// LocalPoolAllowLoopback (WERBOT_LOCAL_POOL_ALLOW_LOOPBACK, default false) lets the
	// embedded gateway dial loopback, link-local and unspecified target addresses.
	LocalPoolAllowLoopback bool

	RecordingsS3 RecordingsS3 // WERBOT_RECORDINGS_S3_*

	// GeoIPURL (WERBOT_GEOIP_URL, default DefaultGeoIPURL) is the country
	// database download; "%s" is the month as 2006-01. It may hold a token.
	GeoIPURL string
}

// Vars lists every environment variable Load reads, in contract order.
func Vars() []string {
	return []string{
		"WERBOT_ENV", "WERBOT_HTTP_ADDR", "WERBOT_DATABASE_URL", "WERBOT_LOG_LEVEL", "WERBOT_LOG_FORMAT",
		"WERBOT_METRICS_ADDR", "WERBOT_WEB_EMBEDDED", "WERBOT_PUBLIC_URL", "WERBOT_WEB_URL", "WERBOT_TRUSTED_PROXIES",
		"WERBOT_SMTP_HOST", "WERBOT_SMTP_PORT", "WERBOT_SMTP_USERNAME", "WERBOT_SMTP_PASSWORD", "WERBOT_SMTP_FROM",
		"WERBOT_SECRET_KEY", "WERBOT_SECRET_KEY_PREVIOUS", "WERBOT_DATA_DIR", "WERBOT_GATEWAY_ADDR",
		"WERBOT_LOCAL_POOL_ALLOW_LOOPBACK",
		"WERBOT_RECORDINGS_S3_ENDPOINT", "WERBOT_RECORDINGS_S3_BUCKET", "WERBOT_RECORDINGS_S3_REGION",
		"WERBOT_RECORDINGS_S3_ACCESS_KEY", "WERBOT_RECORDINGS_S3_SECRET_KEY", "WERBOT_GEOIP_URL",
	}
}

// Load reads and validates configuration using getenv.
func Load(getenv func(string) string) (Config, error) {
	c := Config{
		Env:         or(getenv("WERBOT_ENV"), "production"),
		HTTPAddr:    or(getenv("WERBOT_HTTP_ADDR"), ":8080"),
		DatabaseURL: getenv("WERBOT_DATABASE_URL"),
		MetricsAddr: getenv("WERBOT_METRICS_ADDR"),
		WebEmbedded: true,
		DataDir:     or(getenv("WERBOT_DATA_DIR"), "./data"),
		GatewayAddr: or(getenv("WERBOT_GATEWAY_ADDR"), ":8443"),
		GeoIPURL:    or(getenv("WERBOT_GEOIP_URL"), DefaultGeoIPURL),
	}
	if c.DatabaseURL == "" {
		return Config{}, errors.New("WERBOT_DATABASE_URL: required")
	}
	if err := oneOf("WERBOT_ENV", c.Env, "production", "development"); err != nil {
		return Config{}, err
	}
	var err error
	if c.LogLevel, c.LogFormat, err = LoadLogging(getenv); err != nil {
		return Config{}, err
	}
	if v := getenv("WERBOT_WEB_EMBEDDED"); v != "" {
		b, err := strconv.ParseBool(v)
		if err != nil {
			return Config{}, fmt.Errorf("WERBOT_WEB_EMBEDDED: unsupported value %q", v)
		}
		c.WebEmbedded = b
	}
	if v := getenv("WERBOT_LOCAL_POOL_ALLOW_LOOPBACK"); v != "" {
		if c.LocalPoolAllowLoopback, err = strconv.ParseBool(v); err != nil {
			return Config{}, fmt.Errorf("WERBOT_LOCAL_POOL_ALLOW_LOOPBACK: unsupported value %q", v)
		}
	}
	if c.PublicURL, err = httpURL("WERBOT_PUBLIC_URL", getenv("WERBOT_PUBLIC_URL")); err != nil {
		return Config{}, err
	}
	c.WebURL = c.PublicURL
	if v := getenv("WERBOT_WEB_URL"); v != "" {
		if c.WebURL, err = httpURL("WERBOT_WEB_URL", v); err != nil {
			return Config{}, err
		}
	}
	if c.TrustedProxies, err = ParseTrustedProxies(getenv("WERBOT_TRUSTED_PROXIES")); err != nil {
		return Config{}, err
	}
	if c.SMTP, err = loadSMTP(getenv); err != nil {
		return Config{}, err
	}
	if c.SecretKey, err = decodeKey("WERBOT_SECRET_KEY", getenv("WERBOT_SECRET_KEY")); err != nil {
		return Config{}, err
	}
	if v := getenv("WERBOT_SECRET_KEY_PREVIOUS"); v != "" {
		for s := range strings.SplitSeq(v, ",") {
			k, err := decodeKey("WERBOT_SECRET_KEY_PREVIOUS", strings.TrimSpace(s))
			if err != nil {
				return Config{}, err
			}
			c.PreviousSecretKeys = append(c.PreviousSecretKeys, k)
		}
	}
	if c.RecordingsS3, err = loadRecordingsS3(getenv); err != nil {
		return Config{}, err
	}
	if err := checkGeoIPURL(c.GeoIPURL); err != nil {
		return Config{}, err
	}
	return c, nil
}

func loadRecordingsS3(getenv func(string) string) (RecordingsS3, error) {
	r := RecordingsS3{
		Endpoint:  getenv("WERBOT_RECORDINGS_S3_ENDPOINT"),
		Bucket:    getenv("WERBOT_RECORDINGS_S3_BUCKET"),
		Region:    getenv("WERBOT_RECORDINGS_S3_REGION"),
		AccessKey: getenv("WERBOT_RECORDINGS_S3_ACCESS_KEY"),
		SecretKey: getenv("WERBOT_RECORDINGS_S3_SECRET_KEY"),
	}
	switch {
	case r.AccessKey != "" && r.SecretKey == "":
		return RecordingsS3{}, errors.New("WERBOT_RECORDINGS_S3_SECRET_KEY: required when WERBOT_RECORDINGS_S3_ACCESS_KEY is set")
	case r.SecretKey != "" && r.AccessKey == "":
		return RecordingsS3{}, errors.New("WERBOT_RECORDINGS_S3_ACCESS_KEY: required when WERBOT_RECORDINGS_S3_SECRET_KEY is set")
	}
	if r.Bucket != "" && r.Region == "" {
		r.Region = "us-east-1"
	}
	return r, nil
}

// LoadLogging reads and validates WERBOT_LOG_LEVEL (debug|info|warn|error,
// default "info") and WERBOT_LOG_FORMAT (json|text, default "json").
func LoadLogging(getenv func(string) string) (level, format string, err error) {
	level, format = or(getenv("WERBOT_LOG_LEVEL"), "info"), or(getenv("WERBOT_LOG_FORMAT"), "json")
	if err := oneOf("WERBOT_LOG_LEVEL", level, "debug", "info", "warn", "error"); err != nil {
		return "", "", err
	}
	if err := oneOf("WERBOT_LOG_FORMAT", format, "json", "text"); err != nil {
		return "", "", err
	}
	return level, format, nil
}

// ParseTrustedProxies parses comma-separated CIDRs; empty input gives nil.
func ParseTrustedProxies(s string) ([]netip.Prefix, error) {
	if s == "" {
		return nil, nil
	}
	var out []netip.Prefix
	for part := range strings.SplitSeq(s, ",") {
		p, err := netip.ParsePrefix(strings.TrimSpace(part))
		if err != nil {
			return nil, fmt.Errorf("WERBOT_TRUSTED_PROXIES: %w", err)
		}
		out = append(out, p)
	}
	return out, nil
}

func loadSMTP(getenv func(string) string) (SMTP, error) {
	s := SMTP{
		Host:     getenv("WERBOT_SMTP_HOST"),
		Port:     587,
		Username: getenv("WERBOT_SMTP_USERNAME"),
		Password: getenv("WERBOT_SMTP_PASSWORD"),
		From:     getenv("WERBOT_SMTP_FROM"),
	}
	if v := getenv("WERBOT_SMTP_PORT"); v != "" {
		port, err := strconv.Atoi(v)
		if err != nil || port < 1 || port > 65535 {
			return SMTP{}, fmt.Errorf("WERBOT_SMTP_PORT: want 1-65535, got %q", v)
		}
		s.Port = port
	}
	if s.Host != "" && s.From == "" {
		return SMTP{}, errors.New("WERBOT_SMTP_FROM: required when WERBOT_SMTP_HOST is set")
	}
	return s, nil
}

// httpURL validates an http(s)://host URL and trims the trailing "/".
func httpURL(name, v string) (string, error) {
	if v == "" {
		return "", fmt.Errorf("%s: required", name)
	}
	u, err := url.Parse(v)
	if err != nil {
		return "", fmt.Errorf("%s: %w", name, err)
	}
	if (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return "", fmt.Errorf("%s: want http(s)://host, got %q", name, v)
	}
	return strings.TrimSuffix(v, "/"), nil
}

// checkGeoIPURL wants https://, or http:// to a loopback host, and at most
// one "%s". Errors omit the URL: it may hold a token.
func checkGeoIPURL(v string) error {
	if strings.Count(v, "%s") > 1 {
		return errors.New(`WERBOT_GEOIP_URL: at most one "%s"`)
	}
	u, err := url.Parse(strings.Replace(v, "%s", "2006-01", 1))
	if err != nil || u.Host == "" || (u.Scheme != "https" && (u.Scheme != "http" || !isLoopback(u.Hostname()))) {
		return errors.New("WERBOT_GEOIP_URL: want https://host, or http:// to a loopback host")
	}
	return nil
}

// isLoopback reports whether host is localhost, 127.0.0.0/8 or ::1.
func isLoopback(host string) bool {
	if host == "localhost" {
		return true
	}
	ip, err := netip.ParseAddr(host)
	return err == nil && ip.IsLoopback()
}

func decodeKey(name, v string) ([]byte, error) {
	if v == "" {
		return nil, fmt.Errorf("%s: required", name)
	}
	k, err := base64.StdEncoding.DecodeString(v)
	if err != nil {
		return nil, fmt.Errorf("%s: invalid base64: %w", name, err)
	}
	if len(k) != secretKeyLen {
		return nil, fmt.Errorf("%s: want %d bytes, got %d", name, secretKeyLen, len(k))
	}
	return k, nil
}

func or(v, def string) string {
	if v == "" {
		return def
	}
	return v
}

func oneOf(name, v string, allowed ...string) error {
	for _, a := range allowed {
		if v == a {
			return nil
		}
	}
	return fmt.Errorf("%s: unsupported value %q", name, v)
}
