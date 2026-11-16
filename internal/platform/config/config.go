// Package config loads process configuration from WERBOT_* environment variables.
package config

import (
	"errors"
	"fmt"
	"strconv"
)

// Config is the validated server configuration.
type Config struct {
	Env         string // WERBOT_ENV: "production" (default) | "development"
	HTTPAddr    string // WERBOT_HTTP_ADDR, default ":8080"
	DatabaseURL string // WERBOT_DATABASE_URL, required
	LogLevel    string // WERBOT_LOG_LEVEL: debug|info|warn|error, default "info"
	LogFormat   string // WERBOT_LOG_FORMAT: json|text, default "json"
	MetricsAddr string // WERBOT_METRICS_ADDR, default "" (metrics disabled)
	WebEmbedded bool   // WERBOT_WEB_EMBEDDED, default true
}

// Load reads and validates configuration using getenv.
func Load(getenv func(string) string) (Config, error) {
	c := Config{
		Env:         or(getenv("WERBOT_ENV"), "production"),
		HTTPAddr:    or(getenv("WERBOT_HTTP_ADDR"), ":8080"),
		DatabaseURL: getenv("WERBOT_DATABASE_URL"),
		LogLevel:    or(getenv("WERBOT_LOG_LEVEL"), "info"),
		LogFormat:   or(getenv("WERBOT_LOG_FORMAT"), "json"),
		MetricsAddr: getenv("WERBOT_METRICS_ADDR"),
		WebEmbedded: true,
	}
	if c.DatabaseURL == "" {
		return Config{}, errors.New("WERBOT_DATABASE_URL: required")
	}
	if err := oneOf("WERBOT_ENV", c.Env, "production", "development"); err != nil {
		return Config{}, err
	}
	if err := oneOf("WERBOT_LOG_LEVEL", c.LogLevel, "debug", "info", "warn", "error"); err != nil {
		return Config{}, err
	}
	if err := oneOf("WERBOT_LOG_FORMAT", c.LogFormat, "json", "text"); err != nil {
		return Config{}, err
	}
	if v := getenv("WERBOT_WEB_EMBEDDED"); v != "" {
		b, err := strconv.ParseBool(v)
		if err != nil {
			return Config{}, fmt.Errorf("WERBOT_WEB_EMBEDDED: unsupported value %q", v)
		}
		c.WebEmbedded = b
	}
	return c, nil
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
