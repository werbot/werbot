//go:build unix

package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAgentProfileDirPermissions(t *testing.T) {
	h := newHarness(t)
	f := useAgentServer(t, h)
	sponsorOps(t, h, f)
	dir := filepath.Join(t.TempDir(), "home")
	require.NoError(t, os.Mkdir(dir, 0o755)) //nolint:gosec // loose on purpose: an existing empty directory is tightened

	require.NoError(t, h.run("agents", "profile", "create", "ops", "--dir", dir))

	assertMode := func(path string, want os.FileMode) {
		st, err := os.Stat(path)
		require.NoError(t, err)
		assert.Equal(t, want, st.Mode().Perm(), path)
	}
	assertMode(dir, 0o700)
	require.NoError(t, filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil || path == dir {
			return err
		}
		if d.IsDir() {
			assertMode(path, 0o700)
		} else {
			assertMode(path, 0o600)
		}
		return nil
	}))
}

// A directory that cannot be written revokes the profile just created.
func TestAgentProfileCreateRevokesOnWriteError(t *testing.T) {
	h := newHarness(t)
	f := useAgentServer(t, h)
	sponsorOps(t, h, f)
	parent := t.TempDir()
	require.NoError(t, os.Chmod(parent, 0o500))       //nolint:gosec // read-only on purpose
	t.Cleanup(func() { _ = os.Chmod(parent, 0o700) }) //nolint:gosec // restore for cleanup
	if os.WriteFile(filepath.Join(parent, "probe"), nil, 0o600) == nil {
		t.Skip("directory permissions are not enforced (root)")
	}

	require.Error(t, h.run("agents", "profile", "create", "ops", "--dir", filepath.Join(parent, "home")))
	created := f.createdProfiles()
	require.Len(t, created, 1)
	assert.Len(t, f.pick(func() []string { return f.revoked }), 1, "the unusable profile is revoked")
}

// WERBOT_HOME=<dir> works for the agent commands: the profile refresh token
// is refreshed, then exchanged for the one agent.
func TestAgentProfileDirRunsAgent(t *testing.T) {
	h := newHarness(t)
	f := useAgentServer(t, h)
	sponsorOps(t, h, f)
	dir := filepath.Join(t.TempDir(), "home")
	require.NoError(t, h.run("agents", "profile", "create", "ops", "--dir", dir))

	h.vars["WERBOT_HOME"] = dir
	require.NoError(t, h.run("agent", "run", "--", "true"))
	forms := f.exchangeForms()
	require.Len(t, forms, 2)
	assert.Equal(t, "refresh_token", forms[0].Get("grant_type"))
	assert.Equal(t, profileToken, forms[0].Get("refresh_token"))
	assert.Equal(t, "agent:ops", forms[1].Get("scope"))
	assert.Equal(t, "werbot-api", forms[1].Get("audience"))
}
