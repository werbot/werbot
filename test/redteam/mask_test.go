//go:build redteam

package redteam

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/mcpserver"
	"github.com/werbot/werbot/test/cluster"
)

// Scenario 4 (12 §12 item 4, contract §3.3): the credential Werbot
// substitutes never reaches an agent. Every check of the secret is a boolean
// with a fixed message: the value is never printed (ruling G5).
const (
	secretTarget = "secret-target" // resource on the stand's sshtarget, created by useSecretTarget
	// targetSecret is the sshtarget -password of the stand compose file: a test-only marker.
	targetSecret = "rt-secret-9f31c0a8" //nolint:gosec // test-only marker, see above
	maskMarker   = "[masked]"
	leakWindow   = 8 // no piece of the secret this long may reach an agent
)

// leaks reports whether out carries targetSecret or any leakWindow-byte piece of it.
func leaks(out string) bool {
	for i := 0; i+leakWindow <= len(targetSecret); i++ {
		if strings.Contains(out, targetSecret[i:i+leakWindow]) {
			return true
		}
	}
	return false
}

// agentSecret runs command on secretTarget as a, requires status ok and
// logs only the sizes and the [masked] count of the output.
func agentSecret(t *testing.T, a *cluster.Agent, command string) mcpserver.ExecOutput {
	t.Helper()
	out, err := a.Exec(t.Context(), mcpserver.ExecInput{Resource: secretTarget, Command: command})
	require.NoError(t, err)
	require.Equal(t, "ok", out.Status, out.Message)
	t.Logf("%s: stdout %d bytes, stderr %d bytes, %d %s", command, len(out.Stdout), len(out.Stderr),
		strings.Count(out.Stdout+out.Stderr, maskMarker), maskMarker)
	if leaks(out.Stdout + out.Stderr) {
		t.Fatal("agent output carries the secret or a piece of it")
	}
	return out
}

// TestRedteamSecretNotVisibleToAgent: under mask_output "agents" (the
// default) an agent reads the substituted credential as [masked].
func TestRedteamSecretNotVisibleToAgent(t *testing.T) {
	begin(t)
	useSecretTarget(t)
	a := st.StartAgent(t, home, "rt-mask", "redteam")
	requireAgentHealthy(t, a)
	if out := agentSecret(t, a, "cat /var/tmp/secret"); !strings.Contains(out.Stdout, maskMarker) {
		t.Fatal("agent stdout has no [masked] in place of the secret")
	}
}

// TestRedteamSecretSplitAcrossChunks: a secret written in two parts, cut in
// the middle of the value, reaches the agent as [masked] and in no part.
func TestRedteamSecretSplitAcrossChunks(t *testing.T) {
	begin(t)
	useSecretTarget(t)
	a := st.StartAgent(t, home, "rt-mask", "redteam")
	requireAgentHealthy(t, a)
	if out := agentSecret(t, a, "cat /var/tmp/secret.split"); !strings.Contains(out.Stdout, maskMarker) {
		t.Fatal("agent stdout has no [masked] in place of the split secret")
	}
}

// TestRedteamSecretInStderr: a secret on stderr reaches the agent as [masked].
func TestRedteamSecretInStderr(t *testing.T) {
	begin(t)
	useSecretTarget(t)
	a := st.StartAgent(t, home, "rt-mask", "redteam")
	requireAgentHealthy(t, a)
	if out := agentSecret(t, a, "cat /var/tmp/secret.err"); !strings.Contains(out.Stderr, maskMarker) {
		t.Fatal("agent stderr has no [masked] in place of the secret")
	}
}

// TestRedteamHumanSeesSecretWhenAgentsOnly: under "agents" a person reads the
// original value; under "all" the person reads [masked] too.
func TestRedteamHumanSeesSecretWhenAgentsOnly(t *testing.T) {
	begin(t)
	useSecretTarget(t)
	if out, code := aliceExec(t, secretTarget, "cat /var/tmp/secret"); out != targetSecret+"\n" || code != 0 {
		t.Fatalf("under mask_output agents a person does not read the original value (%d bytes, exit %d)", len(out), code)
	}
	setMaskOutput(t, "all")
	var out string
	masked := within(settle, func() bool {
		out, _ = aliceExec(t, secretTarget, "cat /var/tmp/secret")
		return out == maskMarker+"\n"
	})
	if leaks(out) {
		t.Fatal("under mask_output all a person reads the secret")
	}
	require.True(t, masked, "under mask_output all a person reads [masked] (%d bytes)", len(out))
}

// TestRedteamRecordingKeepsOriginal: the recording of the agent's exec
// verifies and keeps the original output (contract §3.3); this is the only
// place where the value is compared with a recording, and the body is never printed.
func TestRedteamRecordingKeepsOriginal(t *testing.T) {
	begin(t)
	useSecretTarget(t)
	a := st.StartAgent(t, home, "rt-mask", "redteam")
	requireAgentHealthy(t, a)
	agentSecret(t, a, "cat /var/tmp/secret")
	var id uuid.UUID
	require.NoError(t, st.DB.QueryRow(context.Background(), "SELECT id FROM session WHERE agent_run_id = $1 AND command = $2",
		a.RunID, "cat /var/tmp/secret").Scan(&id))
	st.WaitVerified(t, id, cluster.UploadDrain)

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, st.Edge.BaseURL+"/api/recordings/"+id.String()+".cast", nil)
	require.NoError(t, err)
	res, err := st.Edge.HTTP.Do(req)
	require.NoError(t, err)
	defer res.Body.Close()
	require.Equal(t, http.StatusOK, res.StatusCode)
	require.Equal(t, "true", res.Header.Get("X-Werbot-Recording-Verified"))
	body, err := io.ReadAll(res.Body)
	require.NoError(t, err)
	t.Logf("recording %s: %d bytes", id, len(body))
	if !bytes.Contains(body, []byte(targetSecret)) {
		t.Fatal("the recording does not keep the original output")
	}
}
