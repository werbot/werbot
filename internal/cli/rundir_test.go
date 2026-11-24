package cli

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"

	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
	"github.com/werbot/werbot/internal/clientauth"
)

// exchanged returns an agent run of the harness profile with a certificate
// that has left validity (0: a full one) and keeps issuing such certificates.
func exchanged(t *testing.T, h *harness, left time.Duration) (*agentFake, *clientauth.AgentProfile) {
	t.Helper()
	f := useAgentServer(t, h)
	if left > 0 {
		f.certLeft = left
	}
	_, p, err := loadProfile(h.env)
	require.NoError(t, err)
	run, err := p.ExchangeAgent(t.Context(), "ops", "test", "")
	require.NoError(t, err)
	_, err = run.Signer(t.Context())
	require.NoError(t, err)
	return f, run
}

func TestRunDirLayout(t *testing.T) {
	h := newHarness(t)
	h.resources[0].Name = "srv"
	h.resources = append(h.resources, &werbotv1.AccessibleResource{Name: "pg", Type: "postgres", Kinds: []string{"stream"}, SshAddress: "gw.example.com:2222"})
	_, run := exchanged(t, h, 0)
	dir := filepath.Join(t.TempDir(), "runs", "r1")

	require.NoError(t, writeRunDir(dir, run, h.resources, "/usr/bin/ssh", "linux"))

	read := func(name string) string {
		raw, err := os.ReadFile(filepath.Join(dir, name)) //nolint:gosec // test temp dir
		require.NoError(t, err)
		return string(raw)
	}
	cfg := read("ssh_config")
	for _, want := range []string{"Host srv\n", "HostName gw.example.com", "Port 2222", "User alice+srv",
		"IdentityFile " + filepath.Join(dir, "id_ed25519"), "CertificateFile " + filepath.Join(dir, "id_ed25519-cert.pub"),
		"IdentitiesOnly yes", "IdentityAgent none", "UserKnownHostsFile " + filepath.Join(dir, "known_hosts")} {
		assert.Contains(t, cfg, want)
	}
	assert.NotContains(t, cfg, "Host pg", "only ssh resources get a block")
	assert.Contains(t, read("known_hosts"), "[gw.example.com]:2222 "+strings.TrimSpace(string(ssh.MarshalAuthorizedKey(h.hostKey))))
	wrapper := read(filepath.Join("bin", "ssh"))
	assert.Contains(t, wrapper, "#!/bin/sh")
	assert.Contains(t, wrapper, "exec '/usr/bin/ssh' -F '"+filepath.Join(dir, "ssh_config")+"' \"$@\"")
	if runtime.GOOS == "windows" {
		return
	}
	for name, mode := range map[string]os.FileMode{
		".": 0o700, "bin": 0o700, "bin/ssh": 0o700, "id_ed25519": 0o600, "id_ed25519-cert.pub": 0o600, "ssh_config": 0o600, "known_hosts": 0o600,
	} {
		st, err := os.Stat(filepath.Join(dir, name))
		require.NoError(t, err, name)
		assert.Equal(t, mode, st.Mode().Perm(), name)
	}
}

func TestRunDirWindowsWrapper(t *testing.T) {
	h := newHarness(t)
	_, run := exchanged(t, h, 0)
	dir := t.TempDir()
	require.NoError(t, writeRunDir(dir, run, h.resources, `C:\Windows\System32\OpenSSH\ssh.exe`, "windows"))
	raw, err := os.ReadFile(filepath.Join(dir, "bin", "ssh.cmd")) //nolint:gosec // test temp dir
	require.NoError(t, err)
	assert.Equal(t, `@"C:\Windows\System32\OpenSSH\ssh.exe" -F "`+filepath.Join(dir, "ssh_config")+`" %*`+"\r\n", string(raw))
	_, err = os.Stat(filepath.Join(dir, "bin", "ssh"))
	assert.ErrorIs(t, err, os.ErrNotExist)

	// cmd expands %NAME% in a script: a literal % in a path is doubled
	pct := filepath.Join(t.TempDir(), "50%off")
	require.NoError(t, writeRunDir(pct, run, h.resources, `C:\Tools%1\ssh.exe`, "windows"))
	raw, err = os.ReadFile(filepath.Join(pct, "bin", "ssh.cmd")) //nolint:gosec // test temp dir
	require.NoError(t, err)
	cfg := strings.ReplaceAll(filepath.Join(pct, "ssh_config"), "%", "%%")
	assert.Equal(t, `@"C:\Tools%%1\ssh.exe" -F "`+cfg+`" %*`+"\r\n", string(raw))
}

func TestShellQuote(t *testing.T) {
	assert.Equal(t, `'a b'`, shellQuote("a b"))
	assert.Equal(t, `'it'\''s'`, shellQuote("it's"))
}

// The run ssh_config of a WebSocket-only pool starts this binary by its
// absolute path with the URL: the child has no profile to look a name up in.
func TestRunSSHConfigWebSocketOnly(t *testing.T) {
	h := newHarness(t)
	const ws = "wss://gw.example.com/edge"
	h.resources[0].SshAddress, h.resources[0].WsUrl = "", ws
	_, run := exchanged(t, h, 0)
	dir := filepath.Join(t.TempDir(), "runs", "r1")
	require.NoError(t, writeRunDir(dir, run, h.resources, "/usr/bin/ssh", runtime.GOOS))

	exe, err := os.Executable()
	require.NoError(t, err)
	require.True(t, filepath.IsAbs(exe))
	raw, err := os.ReadFile(filepath.Join(dir, "ssh_config")) //nolint:gosec // test temp dir
	require.NoError(t, err)
	cfg := string(raw)
	assert.Contains(t, cfg, "Host db\n  HostName db\n  HostKeyAlias [gw.example.com]:443\n  ProxyCommand "+proxyCommand(exe, ws, runtime.GOOS)+"\n  User alice+db\n")
	assert.NotContains(t, cfg, "%h")
	raw, err = os.ReadFile(filepath.Join(dir, "known_hosts")) //nolint:gosec // test temp dir
	require.NoError(t, err)
	assert.Contains(t, string(raw), "[gw.example.com]:443 ssh-ed25519 ")
}
