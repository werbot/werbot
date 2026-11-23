package config_test

import (
	"bytes"
	"os"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/platform/config"
)

func env(m map[string]string) func(string) string { return func(k string) string { return m[k] } }

const (
	testKey  = "AQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQE=" // 32 bytes 0x01
	testKey2 = "AgICAgICAgICAgICAgICAgICAgICAgICAgICAgICAgI=" // 32 bytes 0x02
)

// base returns the minimal valid environment plus overrides.
func base(kv map[string]string) func(string) string {
	m := map[string]string{"WERBOT_DATABASE_URL": "postgres://x", "WERBOT_PUBLIC_URL": "https://werbot.example.com", "WERBOT_SECRET_KEY": testKey}
	for k, v := range kv {
		m[k] = v
	}
	return env(m)
}

func TestLoadDefaults(t *testing.T) {
	c, err := config.Load(base(nil))
	require.NoError(t, err)
	host, err := os.Hostname()
	require.NoError(t, err)
	assert.Equal(t, config.Config{
		Env: "production", HTTPAddr: ":8080", DatabaseURL: "postgres://x", LogLevel: "info", LogFormat: "json", MetricsAddr: "", WebEmbedded: true,
		PublicURL: "https://werbot.example.com", WebURL: "https://werbot.example.com", SMTP: config.SMTP{Port: 587}, SecretKey: bytes.Repeat([]byte{1}, 32),
		DataDir: "./data", GatewayAddr: ":8443", GeoIPURL: config.DefaultGeoIPURL,
		InternalAddr: ":8444", InternalURL: "https://" + host + ":8444",
	}, c)
}

func TestLoadGatewayAddrAndDataDir(t *testing.T) {
	c, err := config.Load(base(nil))
	require.NoError(t, err)
	assert.Equal(t, ":8443", c.GatewayAddr)
	assert.Equal(t, "./data", c.DataDir)
	c, err = config.Load(base(map[string]string{"WERBOT_GATEWAY_ADDR": "127.0.0.1:9443", "WERBOT_DATA_DIR": "/var/lib/werbot"}))
	require.NoError(t, err)
	assert.Equal(t, "127.0.0.1:9443", c.GatewayAddr)
	assert.Equal(t, "/var/lib/werbot", c.DataDir)
}

func TestLoadStage3(t *testing.T) {
	c, err := config.Load(base(map[string]string{"WERBOT_PUBLIC_URL": "https://werbot.example.com/"}))
	require.NoError(t, err)
	assert.Equal(t, "https://werbot.example.com", c.PublicURL)
	assert.Equal(t, "https://werbot.example.com", c.WebURL)
	assert.Nil(t, c.TrustedProxies)
	assert.Equal(t, config.SMTP{Port: 587}, c.SMTP)
	assert.Equal(t, bytes.Repeat([]byte{1}, 32), c.SecretKey)
	assert.Nil(t, c.PreviousSecretKeys)
}

func TestLoadPublicURLRequired(t *testing.T) {
	for _, v := range []string{"", "ftp://x", "not a url"} {
		_, err := config.Load(base(map[string]string{"WERBOT_PUBLIC_URL": v}))
		assert.ErrorContains(t, err, "WERBOT_PUBLIC_URL", v)
	}
}

func TestLoadWebURL(t *testing.T) {
	c, err := config.Load(base(map[string]string{"WERBOT_WEB_URL": "https://ui.example.com/"}))
	require.NoError(t, err)
	assert.Equal(t, "https://ui.example.com", c.WebURL)
	assert.Equal(t, "https://werbot.example.com", c.PublicURL)
	_, err = config.Load(base(map[string]string{"WERBOT_WEB_URL": "ftp://x"}))
	assert.ErrorContains(t, err, "WERBOT_WEB_URL")
}

func TestLoadTrustedProxies(t *testing.T) {
	c, err := config.Load(base(map[string]string{"WERBOT_TRUSTED_PROXIES": "10.0.0.0/8, 192.168.1.1/32"}))
	require.NoError(t, err)
	assert.Len(t, c.TrustedProxies, 2)
	_, err = config.Load(base(map[string]string{"WERBOT_TRUSTED_PROXIES": "10.0.0.1"}))
	assert.ErrorContains(t, err, "WERBOT_TRUSTED_PROXIES")
}

func TestLoadSMTP(t *testing.T) {
	c, err := config.Load(base(map[string]string{"WERBOT_SMTP_HOST": "smtp.x", "WERBOT_SMTP_FROM": "a@x", "WERBOT_SMTP_PORT": "25", "WERBOT_SMTP_USERNAME": "u", "WERBOT_SMTP_PASSWORD": "p"}))
	require.NoError(t, err)
	assert.Equal(t, config.SMTP{Host: "smtp.x", Port: 25, Username: "u", Password: "p", From: "a@x"}, c.SMTP)
	_, err = config.Load(base(map[string]string{"WERBOT_SMTP_HOST": "smtp.x"}))
	assert.ErrorContains(t, err, "WERBOT_SMTP_FROM")
	for _, p := range []string{"70000", "0", "abc"} {
		_, err = config.Load(base(map[string]string{"WERBOT_SMTP_PORT": p}))
		assert.ErrorContains(t, err, "WERBOT_SMTP_PORT", p)
	}
}

func TestLoadSecretKey(t *testing.T) {
	for _, v := range []string{"", "AQEBAQEBAQEBAQEBAQEBAQ==", "not-base64"} {
		_, err := config.Load(base(map[string]string{"WERBOT_SECRET_KEY": v}))
		assert.ErrorContains(t, err, "WERBOT_SECRET_KEY", v)
	}
}

func TestLoadPreviousKeys(t *testing.T) {
	c, err := config.Load(base(map[string]string{"WERBOT_SECRET_KEY_PREVIOUS": testKey2 + "," + testKey}))
	require.NoError(t, err)
	assert.Len(t, c.PreviousSecretKeys, 2)
	_, err = config.Load(base(map[string]string{"WERBOT_SECRET_KEY_PREVIOUS": "x"}))
	assert.ErrorContains(t, err, "WERBOT_SECRET_KEY_PREVIOUS")
}

func TestVarsMatchLoad(t *testing.T) {
	var keys []string
	getenv := func(k string) string {
		keys = append(keys, k)
		return base(nil)(k)
	}
	_, err := config.Load(getenv)
	require.NoError(t, err)
	vars := config.Vars()
	assert.Len(t, slices.Compact(slices.Sorted(slices.Values(vars))), len(vars), "duplicates in Vars")
	assert.ElementsMatch(t, vars, slices.Compact(slices.Sorted(slices.Values(keys))))
}

func TestLoadWebEmbedded(t *testing.T) {
	c, err := config.Load(base(map[string]string{"WERBOT_WEB_EMBEDDED": "false"}))
	require.NoError(t, err)
	assert.False(t, c.WebEmbedded)
	_, err = config.Load(base(map[string]string{"WERBOT_WEB_EMBEDDED": "nope"}))
	assert.ErrorContains(t, err, "WERBOT_WEB_EMBEDDED")
}

func TestLoadLocalPoolAllowLoopback(t *testing.T) {
	for v, want := range map[string]bool{"": false, "false": false, "true": true, "1": true} {
		c, err := config.Load(base(map[string]string{"WERBOT_LOCAL_POOL_ALLOW_LOOPBACK": v}))
		require.NoError(t, err, v)
		assert.Equal(t, want, c.LocalPoolAllowLoopback, v)
	}
	_, err := config.Load(base(map[string]string{"WERBOT_LOCAL_POOL_ALLOW_LOOPBACK": "yes please"}))
	assert.ErrorContains(t, err, "WERBOT_LOCAL_POOL_ALLOW_LOOPBACK")
}

func TestLoadMetricsAddr(t *testing.T) {
	c, err := config.Load(base(map[string]string{"WERBOT_METRICS_ADDR": "127.0.0.1:9090"}))
	require.NoError(t, err)
	assert.Equal(t, "127.0.0.1:9090", c.MetricsAddr)
}

func TestLoadMissingDatabaseURL(t *testing.T) {
	_, err := config.Load(base(map[string]string{"WERBOT_DATABASE_URL": ""}))
	assert.ErrorContains(t, err, "WERBOT_DATABASE_URL")
}

func TestLoadInvalidLogLevel(t *testing.T) {
	_, err := config.Load(base(map[string]string{"WERBOT_LOG_LEVEL": "verbose"}))
	assert.ErrorContains(t, err, "WERBOT_LOG_LEVEL")
}

func TestLoadInvalidEnv(t *testing.T) {
	_, err := config.Load(base(map[string]string{"WERBOT_ENV": "staging"}))
	assert.ErrorContains(t, err, "WERBOT_ENV")
}

func TestLoadLogging(t *testing.T) {
	level, format, err := config.LoadLogging(env(nil))
	require.NoError(t, err)
	assert.Equal(t, "info", level)
	assert.Equal(t, "json", format)
	level, format, err = config.LoadLogging(env(map[string]string{"WERBOT_LOG_LEVEL": "debug", "WERBOT_LOG_FORMAT": "text"}))
	require.NoError(t, err)
	assert.Equal(t, "debug", level)
	assert.Equal(t, "text", format)
	_, _, err = config.LoadLogging(env(map[string]string{"WERBOT_LOG_LEVEL": "verbose"}))
	assert.EqualError(t, err, `WERBOT_LOG_LEVEL: unsupported value "verbose"`)
	_, _, err = config.LoadLogging(env(map[string]string{"WERBOT_LOG_FORMAT": "xml"}))
	assert.EqualError(t, err, `WERBOT_LOG_FORMAT: unsupported value "xml"`)
}

func TestLoadRecordingsS3Empty(t *testing.T) {
	c, err := config.Load(base(nil))
	require.NoError(t, err)
	assert.Equal(t, config.RecordingsS3{}, c.RecordingsS3)
}

func TestLoadRecordingsS3RegionDefault(t *testing.T) {
	c, err := config.Load(base(map[string]string{"WERBOT_RECORDINGS_S3_BUCKET": "rec"}))
	require.NoError(t, err)
	assert.Equal(t, config.RecordingsS3{Bucket: "rec", Region: "us-east-1"}, c.RecordingsS3)
}

func TestLoadRecordingsS3PartialCredentials(t *testing.T) {
	_, err := config.Load(base(map[string]string{"WERBOT_RECORDINGS_S3_BUCKET": "rec", "WERBOT_RECORDINGS_S3_ACCESS_KEY": "ak"}))
	require.ErrorContains(t, err, "WERBOT_RECORDINGS_S3_SECRET_KEY")
	_, err = config.Load(base(map[string]string{"WERBOT_RECORDINGS_S3_BUCKET": "rec", "WERBOT_RECORDINGS_S3_SECRET_KEY": "sk"}))
	require.ErrorContains(t, err, "WERBOT_RECORDINGS_S3_ACCESS_KEY")
}

func TestLoadGeoIPURL(t *testing.T) {
	c, err := config.Load(base(nil))
	require.NoError(t, err)
	assert.Equal(t, "https://download.db-ip.com/free/dbip-country-lite-%s.mmdb.gz", c.GeoIPURL)
	for _, v := range []string{
		"https://geo.example.com/country.mmdb?token=a%2Fb",
		"https://geo.example.com/%s/country.tar.gz",
		"http://127.0.0.1:8000/country.mmdb",
		"http://localhost/country.mmdb",
	} {
		c, err = config.Load(base(map[string]string{"WERBOT_GEOIP_URL": v}))
		require.NoError(t, err, v)
		assert.Equal(t, v, c.GeoIPURL)
	}
	for _, v := range []string{
		"http://geo.example.com/country.mmdb?token=SECRET",
		"https://geo.example.com/%s/%s.mmdb?token=SECRET",
		"ftp://geo.example.com/country.mmdb?token=SECRET",
		"/country.mmdb?token=SECRET",
		"https://geo.example.com/%zz?token=SECRET",
	} {
		_, err = config.Load(base(map[string]string{"WERBOT_GEOIP_URL": v}))
		require.ErrorContains(t, err, "WERBOT_GEOIP_URL", v)
		assert.NotContains(t, err.Error(), "SECRET", v)
	}
}

func TestLoadInternal(t *testing.T) {
	host, err := os.Hostname()
	require.NoError(t, err)
	c, err := config.Load(base(nil))
	require.NoError(t, err)
	assert.Equal(t, ":8444", c.InternalAddr)
	assert.Equal(t, "https://"+host+":8444", c.InternalURL)
	c, err = config.Load(base(map[string]string{"WERBOT_INTERNAL_ADDR": "10.0.0.5:9444"}))
	require.NoError(t, err)
	assert.Equal(t, "https://"+host+":9444", c.InternalURL)
	c, err = config.Load(base(map[string]string{"WERBOT_INTERNAL_URL": "https://s1.internal:8444/"}))
	require.NoError(t, err)
	assert.Equal(t, "https://s1.internal:8444", c.InternalURL)
	for _, v := range []string{"http://s1:8444", "https://", "not a url"} {
		_, err = config.Load(base(map[string]string{"WERBOT_INTERNAL_URL": v}))
		assert.ErrorContains(t, err, "WERBOT_INTERNAL_URL", v)
	}
	for _, v := range []string{"8444", ":0", ":65536", ":x"} {
		_, err = config.Load(base(map[string]string{"WERBOT_INTERNAL_ADDR": v}))
		assert.ErrorContains(t, err, "WERBOT_INTERNAL_ADDR", v)
	}
}
