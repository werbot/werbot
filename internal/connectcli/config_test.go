package connectcli

import (
	"errors"
	"flag"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseSpec(t *testing.T) {
	for in, want := range map[string]Spec{
		"pg":            {0, "pg", 0},
		"15432:pg":      {15432, "pg", 0},
		"15432:pg:5433": {15432, "pg", 5433},
		"pg:5433":       {0, "pg", 5433},
		"web:8001":      {0, "web", 8001},
	} {
		got, err := ParseSpec(in)
		require.NoError(t, err, in)
		assert.Equal(t, want, got, in)
	}
	for _, in := range []string{"", ":pg", "a:b:c:d", "pg:70000", "pg:0", "0:pg", "x:pg:1", "1::2", "pg:"} {
		_, err := ParseSpec(in)
		assert.Error(t, err, in)
	}
}

func TestDefaultLocalPort(t *testing.T) {
	for target, want := range map[uint16]uint16{23: 10023, 53: 10053, 1023: 11023, 1024: 1024, 5432: 5432} {
		assert.Equal(t, want, DefaultLocalPort(target), target)
	}
}

func TestLoadConfig(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	env := func(m map[string]string) func(string) string { return func(k string) string { return m[k] } }

	_, err := LoadConfig([]string{"pg"}, env(nil))
	assert.ErrorContains(t, err, "WERBOT_CONNECT_GATEWAY")
	_, err = LoadConfig([]string{"--gateway", "gw", "pg"}, env(nil))
	assert.ErrorContains(t, err, "WERBOT_CONNECT_LOGIN")
	_, err = LoadConfig(nil, env(map[string]string{"WERBOT_CONNECT_GATEWAY": "gw", "WERBOT_CONNECT_LOGIN": "alice"}))
	assert.ErrorContains(t, err, "usage: werbot connect")
	_, err = LoadConfig([]string{"--gateway", "gw:x", "--login", "a", "pg"}, env(nil))
	assert.Error(t, err)
	_, err = LoadConfig([]string{"-h"}, env(nil))
	assert.True(t, errors.Is(err, flag.ErrHelp), err)

	c, err := LoadConfig([]string{"pg", "10053:dns:53"}, env(map[string]string{"WERBOT_CONNECT_GATEWAY": "gw.example.com", "WERBOT_CONNECT_LOGIN": "alice"}))
	require.NoError(t, err)
	assert.Equal(t, Config{Gateway: "gw.example.com:2222", Login: "alice", KnownHosts: filepath.Join(home, ".ssh", "known_hosts"),
		Specs: []Spec{{0, "pg", 0}, {10053, "dns", 53}}}, c)

	c, err = LoadConfig([]string{"--gateway", "::1", "--login", "bob", "-i", "/k", "--known-hosts", "/kh", "pg"},
		env(map[string]string{"WERBOT_CONNECT_GATEWAY": "ignored", "WERBOT_CONNECT_LOGIN": "ignored"}))
	require.NoError(t, err)
	assert.Equal(t, Config{Gateway: "[::1]:2222", Login: "bob", Identity: "/k", KnownHosts: "/kh", Specs: []Spec{{0, "pg", 0}}}, c)

	c, err = LoadConfig([]string{"--gateway", "[::1]:22", "--login", "bob", "pg"}, env(nil))
	require.NoError(t, err)
	assert.Equal(t, "[::1]:22", c.Gateway)
	assert.Equal(t, []string{"WERBOT_CONNECT_GATEWAY", "WERBOT_CONNECT_LOGIN"}, ConfigVars())
}
