package gateway

import (
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func env(m map[string]string) func(string) string { return func(k string) string { return m[k] } }

func TestLoadConfig(t *testing.T) {
	host, _ := os.Hostname()
	c, err := LoadConfig(nil, env(nil))
	require.NoError(t, err)
	require.Equal(t, "./gateway-data", c.DataDir)
	require.Equal(t, ":2222", c.SSHAddr)
	require.Empty(t, c.HTTPAddr)
	require.Equal(t, 5*time.Minute, c.IdleTimeout)
	require.Equal(t, 30*time.Second, c.DrainTimeout)
	require.Equal(t, 10000, c.MaxSessions)
	require.Equal(t, host, c.Name)

	c, err = LoadConfig([]string{"--ssh-addr", ":4000"}, env(map[string]string{envSSHAddr: ":3000"}))
	require.NoError(t, err)
	require.Equal(t, ":4000", c.SSHAddr)

	c, err = LoadConfig([]string{"--drain-timeout", "0"}, env(nil))
	require.NoError(t, err)
	require.Zero(t, c.DrainTimeout)

	for _, tc := range []struct{ k, v string }{{envProxy, "maybe"}, {envMaxSess, "0"}, {envMaxSess, "10001"}, {envDrain, "-1s"}} {
		_, err = LoadConfig(nil, env(map[string]string{tc.k: tc.v}))
		require.ErrorContains(t, err, tc.k)
	}
}

func TestLoadConfigSpoolMax(t *testing.T) {
	for _, tc := range []struct {
		args []string
		env  string
		want int64
	}{
		{nil, "", 1073741824},
		{nil, "1048576", 1048576},
		{nil, "512MiB", 536870912},
		{nil, "64KiB", 65536},
		{[]string{"--spool-max", "2GiB"}, "1GiB", 2147483648},
	} {
		c, err := LoadConfig(tc.args, env(map[string]string{envSpoolMax: tc.env}))
		require.NoError(t, err)
		require.Equal(t, tc.want, c.SpoolMax)
	}
	for _, bad := range []string{"0", "-1KiB", "1GB", "1.5GiB", "x", "9223372036854775807GiB"} {
		_, err := LoadConfig(nil, env(map[string]string{envSpoolMax: bad}))
		require.ErrorContains(t, err, envSpoolMax, bad)
	}
}

func TestLoadConfigMaxDuration(t *testing.T) {
	for v, want := range map[string]time.Duration{"": 12 * time.Hour, "0": 0, "90m": 90 * time.Minute} {
		c, err := LoadConfig(nil, env(map[string]string{envMaxDur: v}))
		require.NoError(t, err, v)
		require.Equal(t, want, c.MaxDuration, v)
	}
	c, err := LoadConfig([]string{"--max-duration", "1h"}, env(map[string]string{envMaxDur: "2h"}))
	require.NoError(t, err)
	require.Equal(t, time.Hour, c.MaxDuration)
	for _, v := range []string{"-1h", "soon"} {
		_, err = LoadConfig(nil, env(map[string]string{envMaxDur: v}))
		require.ErrorContains(t, err, "WERBOT_GATEWAY_MAX_DURATION", v)
	}
}

func TestLoadConfigJoinToken(t *testing.T) {
	c, err := LoadConfig(nil, env(map[string]string{envToken: "from-env"}))
	require.NoError(t, err)
	require.Equal(t, "from-env", c.JoinToken)
	c, err = LoadConfig([]string{"--join-token", "from-flag"}, env(map[string]string{envToken: "from-env"}))
	require.NoError(t, err)
	require.Equal(t, "from-flag", c.JoinToken)
}

func TestConfigVarsMatchLoadConfig(t *testing.T) {
	var asked []string
	_, err := LoadConfig(nil, func(k string) string { asked = append(asked, k); return "" })
	require.NoError(t, err)
	slices.Sort(asked)
	asked = slices.Compact(asked)
	want := ConfigVars()
	require.Len(t, want, 16)
	slices.Sort(want)
	require.Equal(t, want, asked)
}

func testToken(fp string) string {
	return "wjt_" + strings.Repeat("a", 43) + "." + fp
}

func TestParseJoinTokenTrimsSpace(t *testing.T) {
	fp := strings.Repeat("ab", 32)
	got, err := ParseJoinToken("  " + testToken(fp) + "\n")
	require.NoError(t, err)
	require.Equal(t, fp, got)

	for _, bad := range []string{"wjt_abc", "xyz_" + strings.Repeat("a", 43) + "." + fp, testToken(fp[:63])} {
		_, err := ParseJoinToken(bad)
		require.Error(t, err)
		require.NotContains(t, err.Error(), bad)
	}
}

func TestLoadConfigRejectsBadInput(t *testing.T) {
	_, err := LoadConfig([]string{"stray"}, env(nil))
	require.ErrorContains(t, err, "unexpected arguments")
	// a non-empty name is always set (env or hostname), so empty-name is reachable only via --name ""
	_, err = LoadConfig([]string{"--name", ""}, env(nil))
	require.ErrorContains(t, err, envName)
}

// The scheme is checked at enrollment (TestEnrollRejectsCleartextServer), not at
// start: an enrolled instance does not use the server URL.
func TestLoadConfigServerScheme(t *testing.T) {
	for _, u := range []string{"", "https://werbot.example", "http://127.0.0.1:8080", "http://werbot.example", "ftp://werbot.example"} {
		c, err := LoadConfig(nil, env(map[string]string{envServer: u}))
		require.NoError(t, err, u)
		require.Equal(t, u, c.ServerURL)
	}
	c, err := LoadConfig([]string{"--insecure-enroll"}, env(map[string]string{envServer: "http://server:8080"}))
	require.NoError(t, err)
	require.True(t, c.InsecureEnroll)
}
