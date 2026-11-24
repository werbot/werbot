package mcpserver

import (
	"context"
	"io"
	"net"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	gliderssh "github.com/gliderlabs/ssh"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
	"github.com/werbot/werbot/internal/connectcli"
)

// execSetup is an agent run with one resource "srv" behind a fake gateway.
func execSetup(t *testing.T, handle func(*fakeGateway, gliderssh.Session)) (*fakeAPI, *fakeGateway, *mcp.ClientSession) {
	t.Helper()
	a := newFakeAPI(t)
	g := newFakeGateway(t, a.ca.PublicKey(), handle)
	a.resources = []*werbotv1.AccessibleResource{g.resource()}
	return a, g, startRun(t, Options{Profile: a.profile()}).sdk(t)
}

func finishes(a *fakeAPI) []finished {
	return state(a, func() []finished { return slices.Clone(a.finishes) })
}

func TestSSHExecOK(t *testing.T) {
	a, g, cs := execSetup(t, func(_ *fakeGateway, s gliderssh.Session) {
		_, _ = io.WriteString(s, "Linux\n")
		_ = s.Exit(0)
	})
	out := decode[ExecOutput](t, callTool(t, cs, "ssh_exec", map[string]any{"resource": "srv", "command": "uname"}))
	require.NotNil(t, out.ExitCode)
	assert.Equal(t, 0, *out.ExitCode)
	assert.Equal(t, "ok", out.Status)
	assert.Equal(t, "Linux\n", out.Stdout)
	assert.False(t, out.Truncated)

	users, envs, commands := g.seen()
	assert.Equal(t, []string{"alice+srv"}, users)
	assert.Equal(t, []string{"uname"}, commands)
	fin := finishes(a)
	require.Len(t, fin, 1)
	assert.Equal(t, finished{fin[0].id, "ok"}, fin[0])
	assert.Equal(t, []string{"WERBOT_TOOL_CALL=" + fin[0].id}, envs[0], "the gateway learns the tool call")
}

func TestSSHExecInputBounds(t *testing.T) {
	_, g, cs := execSetup(t, func(_ *fakeGateway, s gliderssh.Session) { _ = s.Exit(0) })
	for _, args := range []map[string]any{
		{"resource": "srv", "command": "id", "timeout": 601},
		{"resource": "srv", "command": "id", "timeout": -1},
	} {
		assert.Contains(t, errText(t, callTool(t, cs, "ssh_exec", args)), "timeout must be between 1 and 600 seconds")
	}
	assert.Contains(t, errText(t, callTool(t, cs, "ssh_exec", map[string]any{"resource": "srv", "command": ""})), "command is required")
	assert.Contains(t, errText(t, callTool(t, cs, "ssh_exec", map[string]any{"resource": "", "command": "id"})), "resource is required")
	users, _, _ := g.seen()
	assert.Empty(t, users, "out-of-bounds input opens no SSH session")

	d, err := execTimeout(0)
	require.NoError(t, err)
	assert.Equal(t, DefaultTimeout, d)
	d, err = execTimeout(600)
	require.NoError(t, err)
	assert.Equal(t, MaxTimeout, d)
}

func TestSSHExecTruncates(t *testing.T) {
	_, _, cs := execSetup(t, func(_ *fakeGateway, s gliderssh.Session) {
		var wg sync.WaitGroup
		wg.Go(func() { _, _ = io.WriteString(s, strings.Repeat("o", 100000)) })
		wg.Go(func() { _, _ = io.WriteString(s.Stderr(), strings.Repeat("e", 100000)) })
		wg.Wait()
		_ = s.Exit(3)
	})
	out := decode[ExecOutput](t, callTool(t, cs, "ssh_exec", map[string]any{"resource": "srv", "command": "yes"}))
	assert.Equal(t, MaxOutput, len(out.Stdout)+len(out.Stderr))
	assert.True(t, out.Truncated)
	require.NotNil(t, out.ExitCode)
	assert.Equal(t, 3, *out.ExitCode)
}

func TestSSHExecDenied(t *testing.T) {
	a, _, cs := execSetup(t, func(_ *fakeGateway, s gliderssh.Session) {
		_, _ = io.WriteString(s.Stderr(), "Command denied: destructive commands are not allowed\r\n")
		_ = s.Exit(1)
	})
	out := decode[ExecOutput](t, callTool(t, cs, "ssh_exec", map[string]any{"resource": "srv", "command": "rm -rf /"}))
	assert.Equal(t, "denied", out.Status)
	assert.Equal(t, "Command denied: destructive commands are not allowed", out.Message)
	assert.Equal(t, "denied", finishes(a)[0].status)
}

func TestSSHExecPendingApproval(t *testing.T) {
	prev := pendingAfter
	pendingAfter = 100 * time.Millisecond
	t.Cleanup(func() { pendingAfter = prev })
	a, g, cs := execSetup(t, func(g *fakeGateway, s gliderssh.Session) {
		_, _ = io.WriteString(s.Stderr(), "Waiting for approval a1, timeout 5m\r\nhttps://werbot.test/requests?id=a1\r\n")
		g.silent(s)
	})
	start := time.Now()
	out := decode[ExecOutput](t, callTool(t, cs, "ssh_exec", map[string]any{"resource": "srv", "command": "systemctl restart app"}))
	assert.Less(t, time.Since(start), 300*time.Millisecond)
	assert.Equal(t, "pending_approval", out.Status)
	assert.Equal(t, "a1", out.ApprovalID)
	assert.Equal(t, "https://werbot.test/requests?id=a1", out.ApprovalURL)
	assert.Nil(t, out.ExitCode)
	assert.Equal(t, "pending_approval", finishes(a)[0].status)
	select {
	case <-g.closed:
	case <-time.After(2 * time.Second):
		t.Fatal("the SSH session stays open after pending_approval")
	}
}

// After the gateway's "Approved" line the call waits for the command, even
// one that prints nothing past pendingAfter: it runs once and its own exit
// code or refusal is the result.
func TestSSHExecApprovedSilentCommand(t *testing.T) {
	prev := pendingAfter
	pendingAfter = 100 * time.Millisecond
	t.Cleanup(func() { pendingAfter = prev })
	const notice = "Waiting for approval a1, timeout 5m\r\nhttps://werbot.test/requests?id=a1\r\n"
	for _, tc := range []struct {
		name, after string
		code        int
		want        ExecOutput
	}{
		{"exit code", "", 4, ExecOutput{Status: "ok"}},
		{"refused after the approval", "Agent limit exceeded\r\n", 1, ExecOutput{Status: "error", Message: "Agent limit exceeded"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a, g, cs := execSetup(t, func(_ *fakeGateway, s gliderssh.Session) {
				_, _ = io.WriteString(s.Stderr(), notice)
				time.Sleep(50 * time.Millisecond)
				_, _ = io.WriteString(s.Stderr(), "Approved\r\n")
				time.Sleep(3 * pendingAfter) // the approved command prints nothing for a while
				_, _ = io.WriteString(s.Stderr(), tc.after)
				_ = s.Exit(tc.code)
			})
			out := decode[ExecOutput](t, callTool(t, cs, "ssh_exec", map[string]any{"resource": "srv", "command": "touch f"}))
			require.NotNil(t, out.ExitCode)
			assert.Equal(t, tc.code, *out.ExitCode)
			assert.Equal(t, tc.want.Status, out.Status)
			assert.Equal(t, tc.want.Message, out.Message)
			assert.Equal(t, notice+"Approved\r\n"+tc.after, out.Stderr)
			_, _, commands := g.seen()
			assert.Len(t, commands, 1, "the approved command runs once")
			assert.Equal(t, tc.want.Status, finishes(a)[0].status)
		})
	}
}

func TestSSHExecTimeout(t *testing.T) {
	_, g, cs := execSetup(t, func(g *fakeGateway, s gliderssh.Session) { g.silent(s) })
	start := time.Now()
	out := decode[ExecOutput](t, callTool(t, cs, "ssh_exec", map[string]any{"resource": "srv", "command": "sleep 100", "timeout": 1}))
	elapsed := time.Since(start)
	assert.GreaterOrEqual(t, elapsed, time.Second)
	assert.Less(t, elapsed, 2*time.Second)
	assert.Equal(t, ExecOutput{Status: "error", Message: "timed out after 1s"}, out)
	select {
	case <-g.closed:
	case <-time.After(2 * time.Second):
		t.Fatal("the SSH session stays open after a timeout")
	}
}

func TestSSHExecHostKeyMismatch(t *testing.T) {
	a, g, cs := execSetup(t, func(_ *fakeGateway, s gliderssh.Session) { _ = s.Exit(0) })
	other := newFakeGateway(t, a.ca.PublicKey(), func(_ *fakeGateway, s gliderssh.Session) { _ = s.Exit(0) })
	r := g.resource()
	r.HostKey = other.hostKey
	a.set(func() { a.resources = []*werbotv1.AccessibleResource{r} })

	assert.Contains(t, errText(t, callTool(t, cs, "ssh_exec", map[string]any{"resource": "srv", "command": "id"})), "host key mismatch")
	users, _, _ := g.seen()
	assert.Empty(t, users, "no session with a gateway whose host key differs from the pool key")
	assert.Equal(t, "error", finishes(a)[0].status)
}

func TestGatewayStatus(t *testing.T) {
	for _, tc := range []struct {
		output       string
		code         int
		status, text string
	}{
		{"Access denied\r\n", 1, "denied", "Access denied"},
		{"Command denied: approval denied\r\n", 1, "denied", "Command denied: approval denied"},
		{"Approval timed out\r\n", 1, "denied", "Approval timed out"},
		{"Agent limit exceeded\r\n", 1, "error", "Agent limit exceeded"},
		{"Agent run is not active\r\n", 1, "error", "Agent run is not active"},
		{"Gateway is not ready, try again later\r\n", 1, "error", "Gateway is not ready, try again later"},
		{"Access denied\n", 1, "ok", ""},           // a command's own output ends lines with LF
		{"Access denied\r\n", 2, "ok", ""},         // the gateway exits 1
		{"Access denied\r\nmore\r\n", 1, "ok", ""}, // only the refusal line
		{"", 0, "ok", ""},
	} {
		status, text := gatewayStatus(tc.output, tc.code)
		assert.Equal(t, tc.status, status, "%q %d", tc.output, tc.code)
		assert.Equal(t, tc.text, text, "%q %d", tc.output, tc.code)
	}
}

// A pool with only a WebSocket URL is dialed with that URL.
func TestMCPSSHExecUsesWSURL(t *testing.T) {
	a := newFakeAPI(t)
	g := newFakeGateway(t, a.ca.PublicKey(), func(_ *fakeGateway, s gliderssh.Session) { _ = s.Exit(0) })
	r := g.resource()
	r.SshAddress, r.WsUrl = "", "wss://gw.example.com/v1/ssh"
	a.resources = []*werbotv1.AccessibleResource{r}
	var mu sync.Mutex
	var dialed []connectcli.Endpoint
	dial := func(ctx context.Context, ep connectcli.Endpoint) (net.Conn, error) {
		mu.Lock()
		dialed = append(dialed, ep)
		mu.Unlock()
		var d net.Dialer
		return d.DialContext(ctx, "tcp", g.addr) // stands in for the WebSocket transport
	}
	cs := startRun(t, Options{Profile: a.profile(), Dial: dial}).sdk(t)

	out := decode[ExecOutput](t, callTool(t, cs, "ssh_exec", map[string]any{"resource": "srv", "command": "id"}))
	assert.Equal(t, "ok", out.Status)
	mu.Lock()
	defer mu.Unlock()
	assert.Equal(t, []connectcli.Endpoint{{WSURL: "wss://gw.example.com/v1/ssh"}}, dialed)
}
