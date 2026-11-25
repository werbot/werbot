//go:build linux || darwin

package vnet

import (
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Without SUDO_UID nothing changes: drop and regain are no-ops.
func TestDropPrivilegesWithoutSudo(t *testing.T) {
	euid, egid := os.Geteuid(), os.Getegid()
	regain, err := dropPrivileges(func(string) string { return "" })
	require.NoError(t, err)
	assert.Equal(t, euid, os.Geteuid())
	assert.Equal(t, egid, os.Getegid())
	require.NoError(t, regain())
	assert.Equal(t, euid, os.Geteuid())
}

func TestDropPrivilegesBadSudoIDs(t *testing.T) {
	_, err := dropPrivileges(func(k string) string { return map[string]string{"SUDO_UID": "x", "SUDO_GID": "1"}[k] })
	require.ErrorContains(t, err, "SUDO_UID")
	_, err = dropPrivileges(func(k string) string { return map[string]string{"SUDO_UID": "1", "SUDO_GID": ""}[k] })
	require.ErrorContains(t, err, "SUDO_GID")
}

// Under sudo, files written after the drop belong to the sudo user, the root
// groups are gone, and regain gives root back for the OS cleanup. CI runs it
// from a test binary under sudo (Ruling C16).
func TestDropPrivilegesKeepsOwnership(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("root only: CI runs the vnet test binary under sudo")
	}
	nobody, err := user.Lookup("nobody")
	require.NoError(t, err)
	env := map[string]string{"SUDO_UID": nobody.Uid, "SUDO_GID": nobody.Gid}
	uid, _ := strconv.Atoi(nobody.Uid)
	gid, _ := strconv.Atoi(nobody.Gid)

	shared, err := os.MkdirTemp("", "vnet-drop-") // t.TempDir's parent is closed to nobody
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(shared) })
	require.NoError(t, os.Chmod(shared, 0o777)) //nolint:gosec // nobody writes here
	rootOnly := t.TempDir()                     // under root's 0700 test directory
	require.NoError(t, os.WriteFile(filepath.Join(rootOnly, "f"), nil, 0o600))

	regain, err := dropPrivileges(func(k string) string { return env[k] })
	require.NoError(t, err)
	t.Cleanup(func() { _ = regain() })
	assert.Equal(t, uid, os.Geteuid())
	assert.Equal(t, gid, os.Getegid())
	groups, err := syscall.Getgroups()
	require.NoError(t, err)
	assert.NotContains(t, groups, 0, "root's supplementary groups are dropped")
	require.NoError(t, os.WriteFile(filepath.Join(shared, "profile.json"), []byte("{}"), 0o600))
	assert.Error(t, os.Remove(filepath.Join(rootOnly, "f")), "no root after the drop")

	require.NoError(t, regain())
	assert.Equal(t, 0, os.Geteuid())
	fi, err := os.Stat(filepath.Join(shared, "profile.json"))
	require.NoError(t, err)
	st := fi.Sys().(*syscall.Stat_t)
	assert.Equal(t, uint32(uid), st.Uid) //nolint:gosec // test ids are small
	assert.Equal(t, uint32(gid), st.Gid) //nolint:gosec // test ids are small
	require.NoError(t, os.Remove(filepath.Join(rootOnly, "f")), "root again after regain")
}

// The groups of the current user fit NGROUPS_MAX of the OS (16 on macOS),
// with the primary group first: setgroups refuses a longer list.
func TestGroupsOfFitsNGroupsMax(t *testing.T) {
	u, err := user.Current()
	require.NoError(t, err)
	gid, err := strconv.Atoi(u.Gid)
	require.NoError(t, err)
	got := groupsOf(u.Uid, gid)
	assert.LessOrEqual(t, len(got), ngroupsMax)
	assert.Equal(t, gid, got[0])
}
