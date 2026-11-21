//go:build unix

package app

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestE2EExecPathRules(t *testing.T) {
	s, srv, _ := shellStack(t)
	s.onlyApp(t, srv)

	_, stderr, code := s.execSplit(t, "cat /etc/hosts")
	assert.Equal(t, outsideDenied, stderr)
	assert.Equal(t, 1, code)
	stdout, _, code := s.execSplit(t, "cat app/a.txt")
	assert.Equal(t, "a\n", stdout, "a path inside the rule runs")
	assert.Zero(t, code)

	ctx := context.Background()
	var paths []string
	require.NoError(t, s.pool.QueryRow(ctx, "SELECT paths FROM session WHERE command = 'cat app/a.txt'").Scan(&paths))
	assert.Equal(t, []string{"~/app/a.txt"}, paths, "exec keeps the home symbol: it does not know the home directory")
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		var n int
		require.NoError(c, s.pool.QueryRow(ctx, `SELECT count(*) FROM audit_event WHERE action = 'session.deny'
			AND target_id = $1 AND data->>'reason' = 'denied' AND data->>'target' = 'cat /etc/hosts'`, srv.String()).Scan(&n))
		assert.Positive(c, n)
	}, 20*time.Second, 200*time.Millisecond)
}

func TestE2EConsoleSession(t *testing.T) {
	s, srv, home := shellStack(t)
	s.terminalRule(t, srv, "console")
	rule := s.onlyApp(t, srv)

	c := s.console(t, s.gwSSH, "alice+srv")
	c.expect("Werbot command console on srv. Type help for details.\r\n")
	c.expect("alice@srv:~$ ")
	c.send("echo hi")
	c.expect("\r\nhi\r\n")
	c.expect("alice@srv:~$ ")
	c.send("cd app")
	c.expect("alice@srv:~/app$ ")
	c.send("cat a.txt")
	c.expect("\r\na\r\n")
	c.expect("alice@srv:~/app$ ")
	c.send("cat /etc/hosts")
	c.expect(outsideDenied)
	c.expect("[1] alice@srv:~/app$ ")
	c.send("exit")
	assert.Equal(t, 0, c.close())

	ctx := context.Background()
	var sid, command string
	var classes, paths []string
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		require.NoError(c, s.pool.QueryRow(ctx, `SELECT id::text, coalesce(command, ''), classes, paths FROM session
			WHERE resource_id = $1 ORDER BY started_at DESC LIMIT 1`, srv).Scan(&sid, &command, &classes, &paths))
		assert.Equal(c, "cat a.txt", command, "the session keeps the last line that ran")
	}, 10*time.Second, 100*time.Millisecond)
	assert.Equal(t, []string{"read"}, classes)
	assert.Contains(t, paths, home+"/app/a.txt", "the console resolves paths from the home directory")

	var raw []byte
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		require.NoError(c, s.pool.QueryRow(ctx, `SELECT data FROM audit_event WHERE action = 'session.command_denied'
			AND target_type = 'session' AND target_id = $1`, sid).Scan(&raw))
	}, 20*time.Second, 200*time.Millisecond)
	var data struct {
		Command, Reason         string
		Classes, Paths, Reasons []string
	}
	require.NoError(t, json.Unmarshal(raw, &data))
	assert.Equal(t, "cat /etc/hosts", data.Command)
	assert.Equal(t, []string{"read"}, data.Classes)
	assert.Equal(t, []string{"/etc/hosts"}, data.Paths)
	assert.Equal(t, []string{"path:" + rule}, data.Reasons)
	assert.Equal(t, "denied", data.Reason)

	cast := s.recording(t, sid)
	assert.Contains(t, cast, `"m","echo hi [read]"`)
	assert.Contains(t, cast, `"m","cat a.txt [read]"`)
	assert.NotContains(t, cast, `"m","cat /etc/hosts`, "a refused line is not marked")
}

func TestE2EShellDeniedByTerminalRule(t *testing.T) {
	s, srv, _ := shellStack(t)
	s.terminalRule(t, srv, "deny")
	require.Eventually(t, func() bool {
		out, code := s.pty(t, "")
		return out == "Interactive shell is not allowed on srv\r\n" && code == 1
	}, 5*time.Second, 100*time.Millisecond)

	stdout, _, code := s.execSplit(t, "echo hi")
	assert.Equal(t, "hi\n", stdout, "exec without a PTY still runs")
	assert.Zero(t, code)
}

// TestE2EPTYExecUnderConsoleRule: a terminal with a command is a shell too
// (Ruling O1); under a console rule it is refused, exec without a PTY runs.
func TestE2EPTYExecUnderConsoleRule(t *testing.T) {
	s, srv, _ := shellStack(t)
	s.terminalRule(t, srv, "console")
	require.Eventually(t, func() bool {
		out, code := s.pty(t, "echo hi")
		return out == "Interactive shell is not allowed on srv\r\n" && code == 1
	}, 5*time.Second, 100*time.Millisecond)

	stdout, _, code := s.execSplit(t, "echo hi")
	assert.Equal(t, "hi\n", stdout)
	assert.Zero(t, code)
}
