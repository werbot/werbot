package app

import (
	"context"
	"encoding/json"
	"io"
	"sync"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
	"github.com/werbot/werbot/gen/werbot/v1/werbotv1connect"
	"github.com/werbot/werbot/internal/clientauth"
	"github.com/werbot/werbot/internal/mcpserver"
)

// mcpClient is werbot mcp (mcpserver.Run over two pipes) with the go-sdk
// client "claude-code" connected to it.
type mcpClient struct {
	cs   *mcp.ClientSession
	done chan error
	once sync.Once
}

// startMCP runs mcpserver.Run with opts and connects the client; the test
// cleanup closes it.
func (s *stack) startMCP(t *testing.T, opts mcpserver.Options) *mcpClient {
	t.Helper()
	inR, inW := io.Pipe()
	outR, outW := io.Pipe()
	m := &mcpClient{done: make(chan error, 1)}
	go func() {
		err := mcpserver.Run(context.Background(), inR, outW, opts)
		_ = outW.Close()
		m.done <- err
	}()
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "claude-code", Version: "1.0"}, nil).
		Connect(context.Background(), &mcp.IOTransport{Reader: outR, Writer: inW}, nil)
	require.NoError(t, err)
	m.cs = cs
	t.Cleanup(func() { m.close(t) })
	return m
}

// close closes the client, which is EOF on the server's stdin, and waits
// until Run has ended the run and returned.
func (m *mcpClient) close(t *testing.T) {
	t.Helper()
	m.once.Do(func() {
		_ = m.cs.Close()
		select {
		case err := <-m.done:
			assert.NoError(t, err)
		case <-time.After(15 * time.Second):
			t.Error("mcpserver.Run did not return after stdin EOF")
		}
	})
}

// call calls tool name with args.
func (m *mcpClient) call(t *testing.T, name string, args any) *mcp.CallToolResult {
	t.Helper()
	res, err := m.cs.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: args})
	require.NoError(t, err)
	return res
}

// exec calls ssh_exec with command on the resource srv; the call must not
// be a tool error.
func (m *mcpClient) exec(t *testing.T, command string) mcpserver.ExecOutput {
	t.Helper()
	res := m.call(t, "ssh_exec", map[string]any{"resource": "srv", "command": command})
	require.False(t, res.IsError, "unexpected tool error: %s", toolText(res))
	raw, err := json.Marshal(res.StructuredContent)
	require.NoError(t, err)
	var out mcpserver.ExecOutput
	require.NoError(t, json.Unmarshal(raw, &out))
	return out
}

// execError calls ssh_exec with "uname" on srv; the call must be a tool
// error, whose text it returns.
func (m *mcpClient) execError(t *testing.T) string {
	t.Helper()
	res := m.call(t, "ssh_exec", map[string]any{"resource": "srv", "command": "uname"})
	require.True(t, res.IsError, "the call must be a tool error: %v", res.StructuredContent)
	return toolText(res)
}

// toolText is the text of the first content of res.
func toolText(res *mcp.CallToolResult) string {
	if len(res.Content) == 0 {
		return ""
	}
	if tc, ok := res.Content[0].(*mcp.TextContent); ok {
		return tc.Text
	}
	return ""
}

// agentStack is a CLI stack whose gateway snapshot already holds srv, and
// alice's loopback profile: the sponsor's shared profile.
func agentStack(t *testing.T) (*stack, uuid.UUID, *clientauth.Profile) {
	t.Helper()
	s, srv := cliStack(t)
	s.keyReady(t)
	_, p := s.cliLogin(t, false)
	return s, srv, p
}

// agentsAPI is alice's AgentService client (browser session).
func (s *stack) agentsAPI(t *testing.T) werbotv1connect.AgentServiceClient {
	t.Helper()
	return werbotv1connect.NewAgentServiceClient(s.aliceHTTP(t), s.url)
}

// createAgent creates alice's agent name with limits before any run.
func (s *stack) createAgent(t *testing.T, name string, limits *werbotv1.AgentLimits) {
	t.Helper()
	_, err := s.agentsAPI(t).CreateAgent(context.Background(), connect.NewRequest(&werbotv1.CreateAgentRequest{Name: name, Limits: limits}))
	require.NoError(t, err)
}

// agentRun is the only run of the agent "e2e".
func (s *stack) agentRun(t *testing.T) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	require.NoError(t, s.pool.QueryRow(context.Background(),
		"SELECT r.id FROM agent_run r JOIN agent a ON a.id = r.agent_id WHERE a.name = 'e2e'").Scan(&id))
	return id
}

// runTimeline is GetRun of run as alice.
func (s *stack) runTimeline(t *testing.T, run uuid.UUID) *werbotv1.GetRunResponse {
	t.Helper()
	res, err := s.agentsAPI(t).GetRun(context.Background(), connect.NewRequest(&werbotv1.GetRunRequest{Id: run.String()}))
	require.NoError(t, err)
	return res.Msg
}

// agentCommandRule creates a command rule for every agent on srv.
func (s *stack) agentCommandRule(t *testing.T, srv uuid.UUID, effect string, classes ...string) {
	t.Helper()
	_, err := s.api.pol.CreateCommandRule(context.Background(), connect.NewRequest(&werbotv1.CreateCommandRuleRequest{
		Name: "agents-" + effect, Effect: effect, Classes: classes,
		Subjects: &werbotv1.Subjects{AllAgents: true}, ResourceSelector: &werbotv1.ResourceSelector{ResourceIds: []string{srv.String()}}}))
	require.NoError(t, err)
}

// classRule makes pattern a command of classes.
func (s *stack) classRule(t *testing.T, pattern string, classes ...string) {
	t.Helper()
	_, err := s.api.pol.CreateClassRule(context.Background(), connect.NewRequest(&werbotv1.CreateClassRuleRequest{
		Pattern: pattern, Classes: classes}))
	require.NoError(t, err)
}
