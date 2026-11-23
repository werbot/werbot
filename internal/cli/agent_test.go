package cli

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/clientauth"
)

// An agent profile (Profile.Agent) binds the agent name, skips the warning
// and refuses the human commands without a request to the server.
func TestAgentProfileMode(t *testing.T) {
	h := newHarness(t)
	f := useAgentServer(t, h)
	_, p, err := loadAnyProfile(h.env)
	require.NoError(t, err)
	f.saveProfile(t, p.Server, "ops")

	require.NoError(t, h.run("mcp"))
	require.NoError(t, h.run("mcp", "--agent", "ops"))
	assert.NotContains(t, h.errOut.String(), "warning")

	err = h.run("mcp", "--agent", "other")
	assert.EqualError(t, err, `this profile is bound to agent "ops"`)

	for _, args := range [][]string{{"ls"}, {"ssh", "db"}, {"request", "db", "--reason", "x"}, {"requests"}, {"ssh-config"}, {"connect", "db"}} {
		assert.ErrorIs(t, h.run(args...), clientauth.ErrAgentProfile, args[0])
	}
	assert.Zero(t, f.hits.Load(), "no request reached the server")

	h.out.Reset()
	require.NoError(t, h.run("status"))
	assert.Contains(t, h.out.String(), "agent ops")
	assert.Zero(t, f.hits.Load(), "status of an agent profile asks no API")
}

// The run certificate is renewed while the child runs, so the files in the
// run directory stay valid.
func TestKeepFresh(t *testing.T) {
	h := newHarness(t)
	f, run := exchanged(t, h, time.Minute) // always inside the renew window
	dir := t.TempDir()
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan struct{})
	go func() { keepFresh(ctx, run, dir, 10*time.Millisecond, h.errOut); close(done) }()
	require.Eventually(t, func() bool { return f.issuedCerts() >= 3 }, 5*time.Second, 10*time.Millisecond)
	cancel()
	<-done
	assert.FileExists(t, dir+"/id_ed25519-cert.pub")
}

func TestResolveCommand(t *testing.T) {
	bin := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(bin, "ssh"), nil, 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(bin, "scp.cmd"), nil, 0o600))
	assert.Equal(t, filepath.Join(bin, "ssh"), resolveCommand("ssh", bin, "linux"))
	assert.Equal(t, "scp", resolveCommand("scp", bin, "linux"), "only the wrapper is looked up")
	assert.Equal(t, filepath.Join(bin, "scp.cmd"), resolveCommand("scp", bin, "windows"))
	assert.Equal(t, "/usr/bin/ssh", resolveCommand("/usr/bin/ssh", bin, "linux"), "a path is kept")
	assert.Equal(t, "true", resolveCommand("true", bin, "linux"))
}

func TestChildEnvHidesSponsorAgent(t *testing.T) {
	env := childEnv([]string{"SSH_AUTH_SOCK=/sponsor/sock", "PATH=/usr/bin", "A=b"}, "/run/bin")
	assert.Equal(t, []string{"A=b", "SSH_AUTH_SOCK=", "PATH=/run/bin" + string(os.PathListSeparator) + "/usr/bin"}, env)
}
