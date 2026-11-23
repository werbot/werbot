package mcpserver

import (
	"errors"
	"slices"
	"testing"
	"time"

	"connectrpc.com/connect"
	gliderssh "github.com/gliderlabs/ssh"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
)

func TestListResources(t *testing.T) {
	a := newFakeAPI(t)
	a.resources = []*werbotv1.AccessibleResource{{Id: "id-db", Name: "db", Type: "ssh", Kinds: []string{"exec", "tcp"},
		Labels: map[string]string{"env": "prod"}, Allowed: true, AllowedClasses: []string{"read"}, ApprovalClasses: []string{"write"}},
		{Id: "id-web", Name: "web", Type: "ssh", Requestable: true}}
	cs := startRun(t, Options{Profile: a.profile()}).sdk(t)

	out := decode[resourcesOutput](t, callTool(t, cs, "list_resources", map[string]any{}))
	assert.Equal(t, []resourceInfo{
		{Name: "db", Type: "ssh", Kinds: []string{"exec", "tcp"}, Labels: map[string]string{"env": "prod"},
			AllowedClasses: []string{"read"}, ApprovalClasses: []string{"write"}},
		{Name: "web", Type: "ssh", Requestable: true},
	}, out.Resources)

	decode[resourcesOutput](t, callTool(t, cs, "list_resources", map[string]any{}))
	assert.Equal(t, 1, state(a, func() int { return a.lists }), "the second call within 60 s uses the cache")

	calls := state(a, func() []*werbotv1.RecordToolCallRequest { return slices.Clone(a.toolCalls) })
	require.Len(t, calls, 2)
	assert.Equal(t, "list_resources", calls[0].Tool)
	fin := state(a, func() []finished { return slices.Clone(a.finishes) })
	require.Len(t, fin, 2)
	assert.Equal(t, "ok", fin[0].status)
	assert.NotEmpty(t, fin[0].id)
}

func TestRequestAccess(t *testing.T) {
	a := newFakeAPI(t)
	a.resources = []*werbotv1.AccessibleResource{{Id: "0b5a7c1e-6a8f-4f53-9a51-111111111111", Name: "db", Requestable: true},
		{Id: "0b5a7c1e-6a8f-4f53-9a51-222222222222", Name: "web", Allowed: true}}
	cs := startRun(t, Options{Profile: a.profile()}).sdk(t)

	out := decode[requestOutput](t, callTool(t, cs, "request_access", map[string]any{"resource": "db", "duration": "2h", "reason": "deploy"}))
	assert.Equal(t, requestOutput{RequestID: "r1", Status: "pending", URL: "https://werbot.test/requests?id=r1"}, out)
	created := state(a, func() []*werbotv1.CreateRequestRequest { return slices.Clone(a.created) })
	require.Len(t, created, 1)
	assert.Equal(t, "0b5a7c1e-6a8f-4f53-9a51-111111111111", created[0].ResourceId)
	assert.Equal(t, 2*time.Hour, created[0].Duration.AsDuration())
	assert.Equal(t, "deploy", created[0].Reason)

	for _, args := range []map[string]any{
		{"resource": "nope", "duration": "1h", "reason": "x"},
		{"resource": "web", "duration": "1h", "reason": "x"},
	} {
		assert.Contains(t, errText(t, callTool(t, cs, "request_access", args)), "resource not found or not requestable")
	}
	assert.Contains(t, errText(t, callTool(t, cs, "request_access", map[string]any{"resource": "db", "duration": "2x", "reason": "x"})), "duration")
	assert.Len(t, state(a, func() []*werbotv1.CreateRequestRequest { return a.created }), 1)
}

func TestRunStatus(t *testing.T) {
	a := newFakeAPI(t)
	a.run = runOf(a)
	a.requests = []*werbotv1.AccessRequest{
		{Id: "a1", Kind: "command", ResourceName: "db", Command: "rm -rf /tmp/x", Status: "pending", Url: "https://werbot.test/requests?id=a1"},
		{Id: "a2", Kind: "command", ResourceName: "db", Command: "ls", Status: "approved"},
	}
	cs := startRun(t, Options{Profile: a.profile()}).sdk(t)

	out := decode[statusOutput](t, callTool(t, cs, "run_status", map[string]any{}))
	assert.Equal(t, statusOutput{RunID: a.runID.String(), Status: "running", Actions: 7, MaxActions: 100, RemainingActions: 93,
		ExpiresAt:        "2026-11-23T12:00:00Z",
		PendingApprovals: []pendingApproval{{ID: "a1", Resource: "db", Command: "rm -rf /tmp/x", URL: "https://werbot.test/requests?id=a1"}},
	}, out)
}

func TestRunNotActive(t *testing.T) {
	a := newFakeAPI(t)
	a.recordErr = connect.NewError(connect.CodeFailedPrecondition, errors.New("agent run is paused"))
	g := newFakeGateway(t, a.ca.PublicKey(), func(_ *fakeGateway, s gliderssh.Session) { _ = s.Exit(0) })
	a.resources = []*werbotv1.AccessibleResource{g.resource()}
	cs := startRun(t, Options{Profile: a.profile()}).sdk(t)

	msg := errText(t, callTool(t, cs, "ssh_exec", map[string]any{"resource": "srv", "command": "uname"}))
	assert.Contains(t, msg, "agent run is paused")
	users, _, _ := g.seen()
	assert.Empty(t, users, "no SSH session for a refused tool call")
	assert.Empty(t, state(a, func() []finished { return a.finishes }))
}

func TestArgumentsMasked(t *testing.T) {
	got := maskArguments(map[string]any{"password": "p", "command": "ls", "API_Token": "t", "nested": map[string]any{"ssh_key": "k", "n": 1.0},
		"list": []any{map[string]any{"secret": "s"}}})
	assert.Equal(t, map[string]any{"password": "[masked]", "command": "ls", "API_Token": "[masked]",
		"nested": map[string]any{"ssh_key": "[masked]", "n": 1.0}, "list": []any{map[string]any{"secret": "[masked]"}}}, got)

	a := newFakeAPI(t)
	cs := startRun(t, Options{Profile: a.profile()}).sdk(t)
	_, err := cs.CallTool(t.Context(), &mcp.CallToolParams{Name: "ssh_exec", Arguments: map[string]any{"resource": "srv", "command": "ls"},
		Meta: mcp.Meta{"traceparent": "00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01"}})
	require.NoError(t, err)
	calls := state(a, func() []*werbotv1.RecordToolCallRequest { return slices.Clone(a.toolCalls) })
	require.Len(t, calls, 1)
	assert.Equal(t, map[string]any{"resource": "srv", "command": "ls"}, calls[0].Arguments.AsMap())
	assert.Equal(t, "00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01", calls[0].Traceparent)
}
