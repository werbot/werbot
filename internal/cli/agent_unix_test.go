//go:build unix

package cli

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"

	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
	"github.com/werbot/werbot/internal/connectcli"
)

func noRunDirs(t *testing.T, home string) {
	t.Helper()
	left, err := os.ReadDir(filepath.Join(home, "runs"))
	require.NoError(t, err)
	assert.Empty(t, left, "the run directory is removed")
}

func TestAgentRunLifecycle(t *testing.T) {
	h := newHarness(t)
	f := useAgentServer(t, h)

	err := h.run("agent", "run", "--agent", "ops", "--reason", "fix", "--", "sh", "-c", "command -v ssh; exit 3")
	ee, ok := errors.AsType[*exec.ExitError](err)
	require.True(t, ok, "%v", err)
	assert.Equal(t, 3, ee.ExitCode(), "the exit code of the child")

	wrapper := filepath.Join(h.home, "runs", f.runID.String(), "bin", "ssh")
	assert.Equal(t, wrapper+"\n", h.out.String(), "PATH starts with the run bin directory")
	assert.Contains(t, h.errOut.String(), "Agent run "+f.runID.String()+" (agent ops); ssh: "+wrapper+"\n")
	assert.Contains(t, h.errOut.String(), `MCP: {"mcpServers":{"werbot":{"command":"werbot","args":["mcp","--agent","ops"]}}}`)
	assert.Equal(t, []string{f.runID.String()}, f.endedRuns())
	forms := f.exchangeForms()
	require.Len(t, forms, 1)
	assert.Equal(t, "agent:ops", forms[0].Get("scope"))
	assert.Equal(t, "fix", forms[0].Get("werbot_reason"))
	noRunDirs(t, h.home)
}

// The agent name defaults to the command; the shared mode warns once.
func TestAgentRunDefaultsAndWarning(t *testing.T) {
	h := newHarness(t)
	f := useAgentServer(t, h)
	require.NoError(t, h.run("agent", "run", "--", "true"))
	assert.Equal(t, "agent:true", f.exchangeForms()[0].Get("scope"))
	assert.Equal(t, 1, strings.Count(h.errOut.String(), sharedWarningStart))
}

// The child sees no WERBOT_HOME or WERBOT_SERVER and no token of the sponsor.
func TestAgentRunChildEnv(t *testing.T) {
	h := newHarness(t)
	useAgentServer(t, h)
	t.Setenv("WERBOT_HOME", h.home)
	t.Setenv("WERBOT_SERVER", "http://sponsor.example")
	require.NoError(t, h.run("agent", "run", "--", "sh", "-c", `echo "[$WERBOT_HOME][$WERBOT_SERVER]"; env`))
	assert.Contains(t, h.out.String(), "[][]\n")
	assert.NotContains(t, h.out.String(), "sponsor-token")
	assert.NotContains(t, h.out.String(), "sponsor-refresh")
}

func TestAgentRunInterrupt(t *testing.T) {
	h := newHarness(t)
	f := useAgentServer(t, h)
	marker := filepath.Join(t.TempDir(), "started")
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() {
		done <- h.runCtx(ctx, "agent", "run", "--agent", "ops", "--", "sh", "-c", "touch "+marker+"; exec sleep 30")
	}()
	require.Eventually(t, func() bool { _, err := os.Stat(marker); return err == nil }, 10*time.Second, 10*time.Millisecond)
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("the child outlived the interrupt")
	}
	assert.Equal(t, []string{f.runID.String()}, f.endedRuns())
	noRunDirs(t, h.home)
}

// A start failure still ends the run and removes the directory.
func TestAgentRunStartFailure(t *testing.T) {
	h := newHarness(t)
	f := useAgentServer(t, h)
	err := h.run("agent", "run", "--agent", "ops", "--", filepath.Join(t.TempDir(), "missing"))
	require.Error(t, err)
	assert.Equal(t, []string{f.runID.String()}, f.endedRuns())
	noRunDirs(t, h.home)
}

func TestAgentRunLocalPorts(t *testing.T) {
	h := newHarness(t)
	useAgentServer(t, h)
	h.resources = append(h.resources, &werbotv1.AccessibleResource{
		Name: "pg", Type: "postgres", Kinds: []string{"stream"}, Allowed: true,
		SshAddress: "gw.example.com:2222", HostKey: h.resources[0].HostKey,
	})
	var cfg connectcli.Config
	var signer ssh.Signer
	prev := runConnect
	t.Cleanup(func() { runConnect = prev })
	runConnect = func(ctx context.Context, c connectcli.Config, _, _ io.Writer) error {
		cfg = c
		var err error
		signer, err = c.Signer(ctx)
		require.NoError(t, err)
		<-ctx.Done()
		return nil
	}
	require.NoError(t, h.run("agent", "run", "--agent", "ops", "--resource", "pg", "--", "sleep", "0.3"))
	require.Len(t, cfg.Specs, 1)
	assert.Equal(t, "pg", cfg.Specs[0].Resource)
	assert.Equal(t, "alice", cfg.Login)
	assert.Equal(t, "gw.example.com:2222", cfg.Gateway)
	_, isCert := signer.PublicKey().(*ssh.Certificate)
	assert.True(t, isCert, "the run certificate signs the connection")
}

// An agent profile: its name is the agent, another --agent is refused.
func TestAgentRunProfileMode(t *testing.T) {
	h := newHarness(t)
	f := useAgentServer(t, h)
	_, p, err := loadAnyProfile(h.env)
	require.NoError(t, err)
	f.saveProfile(t, p.Server, "ops")

	assert.EqualError(t, h.run("agent", "run", "--agent", "other", "--", "true"), `this profile is bound to agent "ops"`)
	require.NoError(t, h.run("agent", "run", "--", "true"))
	assert.Equal(t, "agent:ops", f.exchangeForms()[0].Get("scope"))
	assert.NotContains(t, h.errOut.String(), "warning")
}

// A bare "ssh" runs the run wrapper (ssh -F <run ssh_config>), not the system
// ssh, and the sponsor's ssh-agent is not visible.
func TestAgentRunUsesWrapperSSH(t *testing.T) {
	h := newHarness(t)
	f := useAgentServer(t, h)
	stub := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(stub, "ssh"), []byte("#!/bin/sh\necho \"system-ssh [$SSH_AUTH_SOCK] $*\"\n"), 0o700)) //nolint:gosec // test stub
	t.Setenv("PATH", stub+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("SSH_AUTH_SOCK", "/sponsor/agent.sock")

	require.NoError(t, h.run("agent", "run", "--agent", "ops", "--", "ssh", "srv", "id"))
	cfg := filepath.Join(h.home, "runs", f.runID.String(), "ssh_config")
	assert.Equal(t, "system-ssh [] -F "+cfg+" srv id\n", h.out.String())
}
