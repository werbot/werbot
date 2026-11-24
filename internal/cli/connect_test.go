package cli

import (
	"context"
	"io"
	"net/netip"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
	"github.com/werbot/werbot/internal/connectcli"
	"github.com/werbot/werbot/internal/vnet"
)

// captureConnect replaces runConnect and returns what it was given.
func captureConnect(t *testing.T) *connectcli.Config {
	t.Helper()
	var got connectcli.Config
	old := runConnect
	runConnect = func(_ context.Context, cfg connectcli.Config, _, _ io.Writer) error { got = cfg; return nil }
	t.Cleanup(func() { runConnect = old })
	return &got
}

func TestConnectUsesProfile(t *testing.T) {
	h := newHarness(t)
	got := captureConnect(t)
	require.NoError(t, h.run("connect", "db"))
	assert.Equal(t, "gw.example.com:2222", got.Gateway)
	assert.Equal(t, "alice", got.Login)
	assert.NotNil(t, got.Signer)
	assert.Equal(t, filepath.Join(h.dir, "known_hosts"), got.KnownHosts)
	assert.FileExists(t, got.KnownHosts)

	h.resources = append(h.resources, &werbotv1.AccessibleResource{Name: "cache", Allowed: true, Kinds: []string{"stream"}, SshAddress: "other.example.com:2222"})
	err := h.run("connect", "db", "cache")
	assert.EqualError(t, err, "resources db, cache are behind different gateways; use --gateway")

	require.NoError(t, h.run("connect", "--gateway", "own.example.com:22", "db", "cache"))
	assert.Equal(t, "own.example.com:22", got.Gateway)
}

// Flags given on the command line are not replaced by the profile.
func TestConnectKeepsGivenFlags(t *testing.T) {
	h := newHarness(t)
	got := captureConnect(t)
	require.NoError(t, h.run("connect", "-i", "key", "--known-hosts=kh", "db"))
	assert.Nil(t, got.Signer)
	assert.Equal(t, "key", got.Identity)
	assert.Equal(t, "kh", got.KnownHosts)
}

func TestConnectWithoutProfile(t *testing.T) {
	h := newHarness(t)
	h.logout(t)
	assert.ErrorContains(t, h.run("connect", "db"), "WERBOT_CONNECT_GATEWAY")
}

func TestHasFlag(t *testing.T) {
	assert.True(t, hasFlag([]string{"--known-hosts=x"}, "known-hosts"))
	assert.True(t, hasFlag([]string{"-i", "k"}, "i"))
	assert.False(t, hasFlag([]string{"i", "--", "-i"}, "i"), "a resource named i, and args after --")
	assert.False(t, hasFlag([]string{"-in"}, "i"))
}

// The pool's WebSocket URL reaches connect with its ssh_address, or alone.
func TestConnectUsesPoolWSURL(t *testing.T) {
	h := newHarness(t)
	got := captureConnect(t)
	h.resources[0].WsUrl = "wss://gw.example.com/v1/ssh"
	require.NoError(t, h.run("connect", "db"))
	assert.Equal(t, "gw.example.com:2222", got.Gateway)
	assert.Equal(t, "wss://gw.example.com/v1/ssh", got.WSURL)

	h.resources[0].SshAddress = ""
	require.NoError(t, h.run("connect", "db"))
	assert.Empty(t, got.Gateway)
	assert.Equal(t, "wss://gw.example.com/v1/ssh", got.WSURL)
	raw, err := os.ReadFile(got.KnownHosts)
	require.NoError(t, err)
	assert.Contains(t, string(raw), "[gw.example.com]:443 ssh-ed25519 ")
}

// captureVNet replaces runVNet and returns the options it was given.
func captureVNet(t *testing.T) *vnet.Options {
	t.Helper()
	var got vnet.Options
	old := runVNet
	runVNet = func(_ context.Context, opts vnet.Options) error { got = opts; return nil }
	t.Cleanup(func() { runVNet = old })
	return &got
}

func TestConnectVNetFlags(t *testing.T) {
	h := newHarness(t)
	got := captureVNet(t)

	require.NoError(t, h.run("connect", "--vnet"))
	assert.Equal(t, netip.MustParsePrefix("100.64.0.0/10"), got.Prefix)
	assert.Equal(t, h.home, got.Home)
	require.NotNil(t, got.Profile)
	assert.Equal(t, "alice", got.Profile.Login)
	assert.NotNil(t, got.Open, "without sudo an MFA URL opens in the browser")
	h.vars["SUDO_UID"] = "501"
	require.NoError(t, h.run("connect", "--vnet"))
	assert.Nil(t, got.Open, "under sudo an MFA URL is printed only")
	delete(h.vars, "SUDO_UID")

	require.NoError(t, h.run("connect", "--vnet-prefix=10.10.0.0/16", "--vnet"))
	assert.Equal(t, netip.MustParsePrefix("10.10.0.0/16"), got.Prefix)
	require.NoError(t, h.run("connect", "--vnet", "-vnet-prefix", "10.20.0.0/16"))
	assert.Equal(t, netip.MustParsePrefix("10.20.0.0/16"), got.Prefix)

	assert.EqualError(t, h.run("connect", "--vnet", "db"), "--vnet does not take resources")
	assert.EqualError(t, h.run("connect", "--vnet", "--gateway", "gw:22"), "--vnet does not take resources")
	assert.ErrorContains(t, h.run("connect", "--vnet", "--vnet-prefix", "bad"), "--vnet-prefix")
	assert.ErrorContains(t, h.run("connect", "--vnet", "--vnet-prefix"), "--vnet-prefix")
	assert.EqualError(t, h.run("connect", "--vnet-prefix", "10.0.0.0/8", "db"), "--vnet-prefix needs --vnet")

	conn := captureConnect(t)
	*got = vnet.Options{}
	require.NoError(t, h.run("connect", "--gateway", "gw.example.com", "vnet"), "a resource named vnet is not the flag")
	assert.Equal(t, "vnet", conn.Specs[0].Resource)
	assert.ErrorContains(t, h.run("connect", "--vnet=true"), "flag provided but not defined: -vnet")
	assert.Nil(t, got.Profile, "connect --vnet never ran")
}
