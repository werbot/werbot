package gateway

import (
	"bytes"
	"errors"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newAgentSSHEnv is an SSH front whose key of alice belongs to her agent a1 in run1.
func newAgentSSHEnv(t *testing.T) *sshEnv {
	t.Helper()
	e := newSSHEnv(t, Config{})
	e.m["a1"], e.m["run1"] = uuid.New(), uuid.New()
	e.f.with(func(f *fakeControl) {
		f.byKey[fp(e.keys["alice"])] = User{ID: e.m["u1"], Login: "alice", Agent: &Agent{ID: e.m["a1"], RunID: e.m["run1"], AuthAt: time.Now()}}
	})
	return e
}

// execEnv runs command on a new session of c with env set; it returns
// stderr and the exit code.
func execEnv(t *testing.T, e *sshEnv, user string, env map[string]string, command string) (string, int) {
	t.Helper()
	s, err := e.client(t, user).NewSession()
	require.NoError(t, err)
	defer s.Close() //nolint:errcheck // already closed by the server
	for k, v := range env {
		require.NoError(t, s.Setenv(k, v))
	}
	var stderr bytes.Buffer
	s.Stderr = &stderr
	code := exitCode(t, s.Run(command))
	return stderr.String(), code
}

// lastEnd waits for the n-th session end and returns its session id.
func (e *sshEnv) lastEnd(t *testing.T, n int) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	require.Eventually(t, func() bool {
		e.f.with(func(f *fakeControl) {
			if len(f.ends) == n {
				id = f.ends[n-1].id
			}
		})
		return id != uuid.Nil
	}, 2*time.Second, 5*time.Millisecond)
	return id
}

func TestToolCallEnvParsing(t *testing.T) {
	e := newAgentSSHEnv(t)
	tool := uuid.New()
	_, code := execEnv(t, e, "alice+spy", map[string]string{EnvToolCall: tool.String()}, "ls")
	require.Equal(t, 7, code)
	assert.Equal(t, agentStart{RunID: e.m["run1"], ToolCall: tool}, sessionAgent(e.lastEnd(t, 1)))

	_, code = execEnv(t, e, "alice+spy", map[string]string{EnvToolCall: "garbage"}, "ls")
	require.Equal(t, 7, code)
	assert.Equal(t, agentStart{RunID: e.m["run1"]}, sessionAgent(e.lastEnd(t, 2)), "an invalid tool call is none")

	assert.Equal(t, tool, toolCallOf([]string{"A=1", EnvToolCall + "=" + tool.String()}))
	assert.Equal(t, uuid.Nil, toolCallOf(nil))
}

func TestAgentLimitText(t *testing.T) {
	e := newAgentSSHEnv(t)
	for text, want := range map[string]string{"agent limit exceeded": "Agent limit exceeded\r\n", "agent run is not active": "Agent run is not active\r\n"} {
		e.f.with(func(f *fakeControl) { f.startErr = connect.NewError(connect.CodeFailedPrecondition, errors.New(text)) })
		stderr, code := execEnv(t, e, "alice+spy", nil, "ls")
		assert.Equal(t, want, stderr)
		assert.Equal(t, 1, code)
	}
}
