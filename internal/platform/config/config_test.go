package config_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/platform/config"
)

func env(m map[string]string) func(string) string { return func(k string) string { return m[k] } }

func TestLoadDefaults(t *testing.T) {
	c, err := config.Load(env(map[string]string{"WERBOT_DATABASE_URL": "postgres://x"}))
	require.NoError(t, err)
	assert.Equal(t, config.Config{Env: "production", HTTPAddr: ":8080", DatabaseURL: "postgres://x", LogLevel: "info", LogFormat: "json", MetricsAddr: "", WebEmbedded: true}, c)
}

func TestLoadWebEmbedded(t *testing.T) {
	c, err := config.Load(env(map[string]string{"WERBOT_DATABASE_URL": "postgres://x", "WERBOT_WEB_EMBEDDED": "false"}))
	require.NoError(t, err)
	assert.False(t, c.WebEmbedded)
	_, err = config.Load(env(map[string]string{"WERBOT_DATABASE_URL": "postgres://x", "WERBOT_WEB_EMBEDDED": "nope"}))
	assert.ErrorContains(t, err, "WERBOT_WEB_EMBEDDED")
}

func TestLoadMetricsAddr(t *testing.T) {
	c, err := config.Load(env(map[string]string{"WERBOT_DATABASE_URL": "postgres://x", "WERBOT_METRICS_ADDR": "127.0.0.1:9090"}))
	require.NoError(t, err)
	assert.Equal(t, "127.0.0.1:9090", c.MetricsAddr)
}

func TestLoadMissingDatabaseURL(t *testing.T) {
	_, err := config.Load(env(nil))
	assert.ErrorContains(t, err, "WERBOT_DATABASE_URL")
}

func TestLoadInvalidLogLevel(t *testing.T) {
	_, err := config.Load(env(map[string]string{"WERBOT_DATABASE_URL": "postgres://x", "WERBOT_LOG_LEVEL": "verbose"}))
	assert.ErrorContains(t, err, "WERBOT_LOG_LEVEL")
}

func TestLoadInvalidEnv(t *testing.T) {
	_, err := config.Load(env(map[string]string{"WERBOT_DATABASE_URL": "postgres://x", "WERBOT_ENV": "staging"}))
	assert.ErrorContains(t, err, "WERBOT_ENV")
}
