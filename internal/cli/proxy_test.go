package cli

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/coder/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
	"github.com/werbot/werbot/internal/clientauth"
	"github.com/werbot/werbot/internal/connectcli"
)

func TestResolveProxyTarget(t *testing.T) {
	const ws = "wss://gw.example.com/v1/ssh"
	rs := []*werbotv1.AccessibleResource{
		{Name: "pg", WsUrl: ws},
		{Name: "db", SshAddress: "gw.example.com:2222", WsUrl: ws},
	}
	list := func(context.Context) ([]*werbotv1.AccessibleResource, error) { return rs, nil }
	for arg, want := range map[string]connectcli.Endpoint{
		"pg":                {WSURL: ws},
		"db":                {SSHAddress: "gw.example.com:2222", WSURL: ws},
		"gw.example.com:22": {SSHAddress: "gw.example.com:22"},
		"gw.example.com":    {SSHAddress: "gw.example.com:2222"},
	} {
		got, err := resolveProxyTarget(t.Context(), arg, list)
		require.NoError(t, err, arg)
		assert.Equal(t, want, got, arg)
	}

	noProfile := func(context.Context) ([]*werbotv1.AccessibleResource, error) { return nil, clientauth.ErrNotLoggedIn }
	for _, u := range []string{ws, "ws://127.0.0.1:8088"} {
		got, err := resolveProxyTarget(t.Context(), u, noProfile)
		require.NoError(t, err, "a URL needs no profile")
		assert.Equal(t, connectcli.Endpoint{WSURL: u}, got)
	}
	_, err := resolveProxyTarget(t.Context(), "pg", noProfile)
	assert.ErrorIs(t, err, clientauth.ErrNotLoggedIn, "a resource name needs the profile")
}

func TestProxyCommandRegistered(t *testing.T) {
	h := newHarness(t)
	require.NoError(t, h.run("--help"))
	assert.Contains(t, h.out.String(), "\n  proxy ")
}

// werbot proxy ws://… copies stdin and stdout over the WebSocket without a profile.
func TestProxyCommandURLWithoutProfile(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		conn := websocket.NetConn(r.Context(), c, websocket.MessageBinary)
		defer conn.Close() //nolint:errcheck // test echo
		buf := make([]byte, 4)
		if _, err := io.ReadFull(conn, buf); err == nil {
			_, _ = conn.Write(buf)
		}
	}))
	t.Cleanup(srv.Close)
	h := newHarness(t)
	h.logout(t)
	h.env.Stdin = strings.NewReader("ping")
	require.NoError(t, h.run("proxy", "ws"+strings.TrimPrefix(srv.URL, "http")))
	assert.Equal(t, "ping", h.out.String())
}

// runProxyCommand runs cmd as ssh does (%-tokens expanded, then "exec <cmd>"
// in the shell) with exe replaced by a stub, and returns the argv the stub got.
func runProxyCommand(t *testing.T, exe, cmd string) []string {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(exe), 0o700))
	stub := "#!/bin/sh\nfor a in \"$@\"; do printf '%s\\n' \"$a\"; done\n"
	require.NoError(t, os.WriteFile(exe, []byte(stub), 0o700))                                     //nolint:gosec // test stub
	out, err := exec.Command("/bin/sh", "-c", "exec "+strings.ReplaceAll(cmd, "%%", "%")).Output() //nolint:gosec // test command
	require.NoError(t, err)
	return strings.Split(strings.TrimSuffix(string(out), "\n"), "\n")
}

// The executable is quoted for the shell of the OS and % is doubled for ssh's
// %-tokens; an argument with shell metacharacters stays one argument.
func TestSSHArgsProxyCommandQuoted(t *testing.T) {
	assert.Equal(t, `'/opt/my tools/50%%werbot' proxy 'pg'`, proxyCommand("/opt/my tools/50%werbot", "pg", "linux"))
	assert.Equal(t, `"C:\My Tools\50%%werbot.exe" proxy "pg"`, proxyCommand(`C:\My Tools\50%werbot.exe`, "pg", "windows"))
	if runtime.GOOS == "windows" {
		t.Skip("POSIX shell")
	}
	exe := filepath.Join(t.TempDir(), "my tools", "50%werbot")
	const u = "wss://gw.example.com/a$b;c'd&e`f"
	assert.Equal(t, []string{"proxy", u}, runProxyCommand(t, exe, proxyCommand(exe, u, runtime.GOOS)))
}

// werbot ssh sets SSH_ASKPASS to itself; the ProxyCommand it gives ssh starts
// werbot with two arguments, which is never the askpass helper.
func TestProxyNotAskpass(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX shell")
	}
	exe := filepath.Join(t.TempDir(), "werbot")
	argv, err := sshArgs(t.TempDir(), exe, &werbotv1.AccessibleResource{Name: "pg", WsUrl: "wss://gw.example.com/v1/ssh"}, "alice", nil)
	require.NoError(t, err)
	var cmd string
	for _, a := range argv {
		if v, ok := strings.CutPrefix(a, "ProxyCommand="); ok {
			cmd = v
		}
	}
	require.NotEmpty(t, cmd, "werbot ssh gives ssh a ProxyCommand for a WebSocket-only pool")
	args := runProxyCommand(t, exe, cmd)
	assert.Equal(t, []string{"proxy", "pg"}, args)
	askpassEnv := map[string]string{"SSH_ASKPASS": exe, "SSH_ASKPASS_REQUIRE": "force"}
	assert.False(t, IsAskpass(args, func(k string) string { return askpassEnv[k] }, exe))
}
