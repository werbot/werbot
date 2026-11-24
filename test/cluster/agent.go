//go:build cluster || load || redteam

package cluster

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
	"github.com/werbot/werbot/internal/mcpserver"
	"github.com/werbot/werbot/internal/platform/secretbox"
)

// agentClient is the MCP client name of StartAgent: agent_run.client of its runs.
const agentClient = "redteam"

const (
	// secretKey is WERBOT_SECRET_KEY of docker-compose.yaml: it opens the sealed email data in river_job.
	secretKey      = "MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY=" //nolint:gosec // test-only
	memberPassword = "member-password-42"                           //nolint:gosec // test-only
	loginWait      = 30 * time.Second                               // device flow: one 5 s poll interval after the approval
	agentEnd       = 15 * time.Second                               // werbot mcp ends its run within 10 s of stdin EOF
)

// userCode is the device code line of werbot login --device.
var userCode = regexp.MustCompile(`enter code (\S+)`)

// SetPoolAddress points pool-a at the published pool front (nginx :2222), the
// address werbot mcp and the CLI dial; it is idempotent.
func (s *Stand) SetPoolAddress(t testing.TB) {
	t.Helper()
	_, err := s.Edge.Gateways.UpdateGateway(context.Background(), connect.NewRequest(&werbotv1.UpdateGatewayRequest{
		Id: s.PoolID.String(), SshAddress: s.Addr(t, "nginx", 2222),
	}))
	require.NoError(t, err)
}

// Login runs werbot login --device --server server with WERBOT_HOME=home and
// approves its code as the user of api; it first sets the pool address.
func (s *Stand) Login(t testing.TB, api API, home, server string) {
	t.Helper()
	s.SetPoolAddress(t)
	cmd := exec.Command(werbotBin, "login", "--device", "--server", server) //nolint:gosec // host build of werbot
	cmd.Env = append(os.Environ(), "WERBOT_HOME="+home)
	var out syncBuffer
	cmd.Stdout, cmd.Stderr = &out, &out
	require.NoError(t, cmd.Start())
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	if !assert.Eventually(t, func() bool { return userCode.MatchString(out.String()) }, loginWait, 50*time.Millisecond) {
		_ = cmd.Process.Kill()
		t.Fatalf("werbot login printed no device code:\n%s", out.String())
	}
	_, err := api.OAuthConsent.ApproveDeviceCode(context.Background(), connect.NewRequest(&werbotv1.ApproveDeviceCodeRequest{
		UserCode: userCode.FindStringSubmatch(out.String())[1],
	}))
	require.NoError(t, err)
	select {
	case err := <-done:
		require.NoError(t, err, "werbot login:\n%s", out.String())
	case <-time.After(loginWait):
		_ = cmd.Process.Kill()
		t.Fatalf("werbot login did not finish within %s:\n%s", loginWait, out.String())
	}
}

// AddMember invites email with role as alice, accepts the invitation as a new
// user (login: the local part of email) and returns the API with that user's
// session. The invitation token comes from the sealed invite email in river_job.
func (s *Stand) AddMember(t testing.TB, email string, role werbotv1.Role) API {
	t.Helper()
	ctx := context.Background()
	_, err := s.Edge.Members.InviteMember(ctx, connect.NewRequest(&werbotv1.InviteMemberRequest{Email: email, Role: role}))
	require.NoError(t, err)
	login, _, _ := strings.Cut(email, "@")
	anon := s.NewAPI(s.Edge.BaseURL, "")
	_, err = anon.Members.AcceptInvite(ctx, connect.NewRequest(&werbotv1.AcceptInviteRequest{
		Token: s.inviteToken(t, email), Login: login, Name: login, Password: memberPassword,
	}))
	require.NoError(t, err)
	res, err := anon.Auth.Login(ctx, connect.NewRequest(&werbotv1.LoginRequest{Email: email, Password: memberPassword}))
	require.NoError(t, err)
	cookie := SessionCookie(res.Header())
	require.NotEmpty(t, cookie, "login of %s set no session", email)
	return s.NewAPI(s.Edge.BaseURL, cookie)
}

// inviteToken opens the newest invite email to email and returns the token of its link.
func (s *Stand) inviteToken(t testing.TB, email string) string {
	t.Helper()
	var args struct {
		SealedData []byte `json:"sealed_data"`
		KeyID      string `json:"key_id"`
	}
	require.NoError(t, s.DB.QueryRow(context.Background(), `SELECT args FROM river_job WHERE kind = 'send_email'
		AND args->>'template' = 'invite' AND args->>'to' = $1 ORDER BY id DESC LIMIT 1`, email).Scan(&args))
	key, err := base64.StdEncoding.DecodeString(secretKey)
	require.NoError(t, err)
	box, err := secretbox.New(key, nil)
	require.NoError(t, err)
	plain, err := box.Open(args.SealedData, args.KeyID)
	require.NoError(t, err)
	var data map[string]string
	require.NoError(t, json.Unmarshal(plain, &data))
	return path.Base(data["url"]) // <web url>/invite/<token>
}

// Agent is one run of werbot mcp --agent with the go-sdk client agentClient on its stdio.
type Agent struct {
	Name           string
	AgentID, RunID uuid.UUID

	cs     *mcp.ClientSession
	stderr *syncBuffer
	once   sync.Once
}

// StartAgent starts werbot mcp --agent name --reason reason with
// WERBOT_HOME=home (a profile from Login) and calls list_resources, which
// starts the run. Cleanup closes it.
func (s *Stand) StartAgent(t testing.TB, home, name, reason string) *Agent {
	t.Helper()
	cmd := exec.Command(werbotBin, "mcp", "--agent", name, "--reason", reason) //nolint:gosec // host build of werbot
	cmd.Env = append(os.Environ(), "WERBOT_HOME="+home)
	a := &Agent{Name: name, stderr: &syncBuffer{}}
	cmd.Stderr = a.stderr
	cs, err := mcp.NewClient(&mcp.Implementation{Name: agentClient, Version: "1.0"}, nil).
		Connect(context.Background(), &mcp.CommandTransport{Command: cmd, TerminateDuration: agentEnd}, nil)
	require.NoError(t, err)
	a.cs = cs
	t.Cleanup(func() { a.Close(t) })
	res, err := a.Call(t.Context(), "list_resources", map[string]any{})
	require.NoError(t, err)
	require.False(t, res.IsError, "list_resources: %s\nstderr:\n%s", toolText(res), a.stderr.String())
	require.NoError(t, s.DB.QueryRow(context.Background(), `SELECT r.id, r.agent_id FROM agent_run r JOIN agent a ON a.id = r.agent_id
		WHERE a.name = $1 AND r.client = $2 AND r.status <> 'ended' ORDER BY r.started_at DESC LIMIT 1`, name, agentClient).
		Scan(&a.RunID, &a.AgentID))
	return a
}

// Call calls tool with args; a tool error is res.IsError, not err. Safe for concurrent use.
func (a *Agent) Call(ctx context.Context, tool string, args any) (*mcp.CallToolResult, error) {
	return a.cs.CallTool(ctx, &mcp.CallToolParams{Name: tool, Arguments: args})
}

// Exec calls ssh_exec with in and decodes its structured output; a tool error
// (inactive or locked run) is err with the tool's text. Safe for concurrent use.
func (a *Agent) Exec(ctx context.Context, in mcpserver.ExecInput) (mcpserver.ExecOutput, error) {
	var out mcpserver.ExecOutput
	res, err := a.Call(ctx, "ssh_exec", in)
	switch {
	case err != nil:
		return out, err
	case res.IsError:
		return out, errors.New(toolText(res))
	}
	raw, err := json.Marshal(res.StructuredContent)
	if err != nil {
		return out, err
	}
	return out, json.Unmarshal(raw, &out)
}

// Close closes stdin, which ends the run, and expects werbot mcp to exit cleanly; it runs once.
func (a *Agent) Close(t testing.TB) {
	t.Helper()
	a.once.Do(func() {
		assert.NoError(t, a.cs.Close(), "werbot mcp exit; stderr:\n%s", a.stderr.String())
	})
}

// toolText is the text of the first content of res.
func toolText(res *mcp.CallToolResult) string {
	if len(res.Content) > 0 {
		if tc, ok := res.Content[0].(*mcp.TextContent); ok {
			return tc.Text
		}
	}
	return ""
}
