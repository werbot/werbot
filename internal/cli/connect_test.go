package cli

import (
	"context"
	"io"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
	"github.com/werbot/werbot/internal/connectcli"
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

	h.resources = append(h.resources, &werbotv1.AccessibleResource{Name: "cache", Kinds: []string{"stream"}, SshAddress: "other.example.com:2222"})
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
