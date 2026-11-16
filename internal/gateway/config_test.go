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

	for _, tc := range []struct{ k, v string }{{envProxy, "maybe"}, {envMaxSess, "0"}, {envDrain, "-1s"}} {
		_, err = LoadConfig(nil, env(map[string]string{tc.k: tc.v}))
		require.ErrorContains(t, err, tc.k)
	}
}

func TestConfigVarsMatchLoadConfig(t *testing.T) {
	var asked []string
	_, err := LoadConfig(nil, func(k string) string { asked = append(asked, k); return "" })
	require.NoError(t, err)
	slices.Sort(asked)
	asked = slices.Compact(asked)
	want := ConfigVars()
	require.Len(t, want, 10)
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
