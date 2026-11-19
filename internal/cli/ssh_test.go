package cli

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"

	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
)

func TestSSHArgs(t *testing.T) {
	dir := filepath.Join("home", "srv")
	r := &werbotv1.AccessibleResource{Name: "db", SshAddress: "gw.example.com:2222"}
	got, err := sshArgs(dir, r, "alice", []string{"uptime", "-a"})
	require.NoError(t, err)
	assert.Equal(t, []string{
		"-i", filepath.Join(dir, "id_ed25519"),
		"-o", "CertificateFile=" + filepath.Join(dir, "id_ed25519-cert.pub"),
		"-o", "UserKnownHostsFile=" + filepath.Join(dir, "known_hosts"),
		"-o", "StrictHostKeyChecking=yes",
		"-o", "IdentitiesOnly=yes",
		"-p", "2222", "alice+db@gw.example.com", "uptime", "-a",
	}, got)

	got, err = sshArgs(dir, &werbotv1.AccessibleResource{Name: "db", SshAddress: "[2001:db8::1]:22"}, "alice", nil)
	require.NoError(t, err)
	assert.Equal(t, []string{"-p", "22", "alice+db@2001:db8::1"}, got[len(got)-3:])

	_, err = sshArgs(dir, &werbotv1.AccessibleResource{Name: "db"}, "alice", nil)
	assert.EqualError(t, err, "gateway address for db is not set; ask an admin to set it on the Gateways page")
}

func TestKnownHostsLines(t *testing.T) {
	key, other := newPub(t), newPub(t)
	line := func(k ssh.PublicKey) string { return string(ssh.MarshalAuthorizedKey(k)) }
	rs := []*werbotv1.AccessibleResource{
		{Name: "a", SshAddress: "gw.example.com:2222", HostKey: line(key)},
		{Name: "b", SshAddress: "gw.example.com:2222", HostKey: line(key)},
		{Name: "c", SshAddress: "plain.example.com:22", HostKey: line(other)},
		{Name: "d", SshAddress: "nokey.example.com:2222"},
	}
	path := filepath.Join(t.TempDir(), "known_hosts")
	require.NoError(t, writeKnownHosts(path, rs))
	require.NoError(t, writeKnownHosts(path, rs))
	raw, err := os.ReadFile(path) //nolint:gosec // test temp file
	require.NoError(t, err)
	want := "[gw.example.com]:2222 " + line(key) + "plain.example.com " + line(other)
	assert.Equal(t, want, string(raw))
}

func TestSSHConfig(t *testing.T) {
	rs := []*werbotv1.AccessibleResource{
		{Name: "db", Kinds: []string{"terminal", "exec", "stream"}, SshAddress: "gw.example.com:2222"},
		{Name: "metrics", Kinds: []string{"datagram"}, SshAddress: "gw.example.com:2222"},
		{Name: "lost", Kinds: []string{"terminal"}},
	}
	got := sshConfig(filepath.Join("home", "srv"), "target.localhost", "alice", rs)
	want, err := os.ReadFile(filepath.Join("testdata", "ssh_config"))
	require.NoError(t, err)
	assert.Equal(t, strings.ReplaceAll(string(want), "/", string(filepath.Separator)), got)
}

func TestSSHConfigCommand(t *testing.T) {
	h := newHarness(t)
	require.NoError(t, h.run("ssh-config"))
	assert.Contains(t, h.out.String(), "Host db.127.0.0.1\n")
	raw, err := os.ReadFile(filepath.Join(h.dir, "known_hosts"))
	require.NoError(t, err)
	assert.Contains(t, string(raw), "[gw.example.com]:2222 ssh-ed25519 ")
}

func TestSSHUnknownResource(t *testing.T) {
	h := newHarness(t)
	assert.EqualError(t, h.run("ssh", "nope"), "resource nope not found or not accessible")
}

// The exit code of ssh reaches the caller as an ExitCode() error.
func TestSSHExitCode(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell stub")
	}
	h := newHarness(t)
	bin := t.TempDir()
	argv := filepath.Join(bin, "argv")
	stub := "#!/bin/sh\necho \"$@\" > " + argv + "\nexit 3\n"
	require.NoError(t, os.WriteFile(filepath.Join(bin, "ssh"), []byte(stub), 0o700)) //nolint:gosec // test stub
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))

	err := h.run("ssh", "db", "--", "uptime", "-a")
	ee, ok := errors.AsType[*exec.ExitError](err)
	require.True(t, ok, "%v", err)
	assert.Equal(t, 3, ee.ExitCode())
	got, rerr := os.ReadFile(argv) //nolint:gosec // test temp file
	require.NoError(t, rerr)
	assert.Contains(t, string(got), "-p 2222 alice+db@gw.example.com uptime -a")
	_, serr := os.Stat(filepath.Join(h.dir, "known_hosts"))
	assert.NoError(t, serr)
}
