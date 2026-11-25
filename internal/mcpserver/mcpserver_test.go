package mcpserver

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"

	gliderssh "github.com/gliderlabs/ssh"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
)

func TestRunStartsLazily(t *testing.T) {
	t.Run("initialize then tools/call", func(t *testing.T) {
		a := newFakeAPI(t)
		h := startRun(t, Options{Profile: a.profile()})
		h.initialize(t)
		require.NotNil(t, h.raw(t, 2, "tools/list", map[string]any{})["result"])
		assert.Empty(t, a.forms(), "initialize starts no run")

		res := h.raw(t, 3, "tools/call", map[string]any{"name": "list_resources", "arguments": map[string]any{}})
		require.NotNil(t, res["result"], "%v", res)
		h.raw(t, 4, "tools/call", map[string]any{"name": "list_resources", "arguments": map[string]any{}})
		forms := a.forms()
		require.Len(t, forms, 1, "one exchange per run")
		assert.Equal(t, "claude-code", forms[0].Get("werbot_client"))
		assert.Equal(t, "agent:claude-code", forms[0].Get("scope"))

		require.NoError(t, h.stop(t))
		assert.Equal(t, []string{a.runID.String()}, a.endedRuns())
	})
	t.Run("initialize only", func(t *testing.T) {
		a := newFakeAPI(t)
		h := startRun(t, Options{Profile: a.profile()})
		h.initialize(t)
		require.NoError(t, h.stop(t))
		assert.Empty(t, a.forms())
		assert.Empty(t, a.endedRuns(), "no run, nothing to end")
	})
	t.Run("tools/call without client info", func(t *testing.T) {
		a := newFakeAPI(t)
		h := startRun(t, Options{Profile: a.profile()})
		res := h.raw(t, 1, "tools/call", map[string]any{"name": "list_resources", "arguments": map[string]any{},
			"_meta": map[string]any{"io.modelcontextprotocol/protocolVersion": "2026-07-28", "io.modelcontextprotocol/clientCapabilities": map[string]any{}}})
		require.NotNil(t, res["result"], "%v", res)
		require.NoError(t, h.stop(t))
		forms := a.forms()
		require.Len(t, forms, 1)
		assert.Equal(t, "unknown", forms[0].Get("werbot_client"))
		assert.Equal(t, "agent:unknown", forms[0].Get("scope"))
	})
	t.Run("sdk client", func(t *testing.T) {
		a := newFakeAPI(t)
		h := startRun(t, Options{Profile: a.profile(), Reason: "fix the build"})
		cs := h.sdk(t)
		decode[resourcesOutput](t, callTool(t, cs, "list_resources", map[string]any{}))
		require.NoError(t, h.stop(t))
		forms := a.forms()
		require.Len(t, forms, 1)
		assert.Equal(t, "claude-code", forms[0].Get("werbot_client"))
		assert.Equal(t, "fix the build", forms[0].Get("werbot_reason"))
		assert.Equal(t, []string{a.runID.String()}, a.endedRuns())
	})
}

func TestRunUsesBoundAgent(t *testing.T) {
	a := newFakeAPI(t)
	p := a.profile()
	p.Agent = "ops"
	cs := startRun(t, Options{Profile: p}).sdk(t)
	decode[resourcesOutput](t, callTool(t, cs, "list_resources", map[string]any{}))
	forms := a.forms()
	require.Len(t, forms, 1)
	assert.Equal(t, "agent:ops", forms[0].Get("scope"))
	assert.Equal(t, "claude-code", forms[0].Get("werbot_client"))
}

func TestExchangeFailureIsToolError(t *testing.T) {
	a := newFakeAPI(t)
	a.exchangeErr = "agent is frozen or locked"
	h := startRun(t, Options{Profile: a.profile(), Agent: "ops"})
	cs := h.sdk(t)

	msg := errText(t, callTool(t, cs, "list_resources", map[string]any{}))
	assert.Contains(t, msg, "invalid_grant")
	assert.Contains(t, msg, "agent is frozen or locked")
	assert.Empty(t, state(a, func() any { return a.toolCalls }), "no run, no tool call")
	select {
	case err := <-h.done:
		t.Fatalf("Run ended on a failed exchange: %v", err)
	default:
	}

	a.set(func() { a.exchangeErr = "" })
	decode[resourcesOutput](t, callTool(t, cs, "list_resources", map[string]any{}))
	assert.Len(t, a.forms(), 2, "the next call tries again")
}

func TestStdoutOnlyJSONRPC(t *testing.T) {
	a := newFakeAPI(t)
	a.exchangeErr = "agent is frozen or locked"
	var logs bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&lockedWriter{w: &logs}, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })

	h := startRun(t, Options{Profile: a.profile()})
	h.initialize(t)
	h.raw(t, 2, "tools/call", map[string]any{"name": "list_resources", "arguments": map[string]any{}})
	a.set(func() { a.exchangeErr = "" })
	h.raw(t, 3, "tools/call", map[string]any{"name": "ssh_exec", "arguments": map[string]any{"resource": "nope", "command": "id"}})
	h.raw(t, 4, "tools/call", map[string]any{"name": "list_resources", "arguments": map[string]any{}})
	require.NoError(t, h.stdin.Close())
	for line := range h.lines {
		jsonRPC(t, line)
	}
	require.NoError(t, h.stop(t))
	assert.NotContains(t, logs.String(), "sponsor-token")
}

func TestNoSecretsInLogs(t *testing.T) {
	var logs bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&lockedWriter{w: &logs}, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })
	a := newFakeAPI(t)
	g := newFakeGateway(t, a.ca.PublicKey(), func(_ *fakeGateway, s gliderssh.Session) { _ = s.Exit(0) })
	a.resources = []*werbotv1.AccessibleResource{g.resource()}
	h := startRun(t, Options{Profile: a.profile()})
	cs := h.sdk(t)
	decode[ExecOutput](t, callTool(t, cs, "ssh_exec", map[string]any{"resource": "srv", "command": "id"}))

	a.srv.Close() // every later API call fails and is logged or returned
	msg := errText(t, callTool(t, cs, "ssh_exec", map[string]any{"resource": "srv", "command": "id"}))
	require.NoError(t, h.stop(t))
	require.Contains(t, logs.String(), "mcp: end the agent run")
	for _, text := range []string{logs.String(), msg} {
		for _, secret := range []string{"sponsor-token", a.jwt(), "ssh-ed25519-cert", "PRIVATE KEY"} {
			assert.NotContains(t, text, secret)
		}
	}
}

func TestAgentName(t *testing.T) {
	for _, tc := range []struct{ flag, client, want string }{
		{"", "Claude Code", "claude-code"},
		{"", "", "agent"},
		{"ops", "x", "ops"},
		{"", "Cursor/1.2 (beta)", "cursor-1-2--beta-"},
		{"", strings.Repeat("a", 100), strings.Repeat("a", 63)},
	} {
		assert.Equal(t, tc.want, AgentName(tc.flag, tc.client), "%q %q", tc.flag, tc.client)
	}
}
