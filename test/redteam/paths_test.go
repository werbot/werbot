//go:build redteam

package redteam

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	gossh "golang.org/x/crypto/ssh"

	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
	"github.com/werbot/werbot/test/cluster"
)

// Scenario 11b (stage 11b, 17 §3–§4): folder rules and the command console
// on target (a real /bin/sh). Every test removes its rules in Cleanup.
const (
	outsideRead   = "cat /etc/hosts" // a read outside ~/app/**
	outsideDenied = "Command denied: path outside allowed folders"
	consoleBanner = "Werbot command console on " + target + "."
	shellDenied   = "Interactive shell is not allowed on " + target
	termWait      = 10 * time.Second
	// denialReport bounds the wait for a person's denial audit: the gateway
	// reports it with its next heartbeat (every 15 s, gateway/tunnel.go).
	denialReport = 20 * time.Second
)

// appOnly keeps every path inside ~/app.
var appOnly = []string{"~/app/**"}

// TestRedteamPathRuleOnlyAgent: under an "only ~/app/**" rule for all
// agents, a read outside is refused with the rule's message and leaves
// session.deny with reason path:<id>; TestCommand marks every corpus path
// outside the folder (and every unresolved one) and none inside.
func TestRedteamPathRuleOnlyAgent(t *testing.T) {
	begin(t)
	rule := pathRule(t, allAgents, appOnly, "any", "only")
	a := st.StartAgent(t, home, "rt-paths", "redteam")
	requireAgentHealthy(t, a)
	var out struct{ status, message, stdout string }
	require.True(t, within(settle, func() bool { // the rule reaches the gateways; until then the read runs
		res, err := a.Exec(t.Context(), execOn(outsideRead))
		require.NoError(t, err)
		out.status, out.message, out.stdout = res.Status, res.Message, res.Stdout
		return res.Status == "denied"
	}), "a read outside ~/app is refused")
	assert.Equal(t, outsideDenied, out.message)
	assert.Empty(t, out.stdout, "a refused command prints nothing of the target")

	for _, section := range []string{"read", "write", "unresolved"} {
		for _, l := range pathCorpus(t, section) {
			res, err := st.Edge.Policy.TestCommand(t.Context(), connect.NewRequest(&werbotv1.TestCommandRequest{
				Subject:    &werbotv1.TestCommandRequest_AgentId{AgentId: a.AgentID.String()},
				ResourceId: st.Resources[target].String(), Command: l.command, Cwd: l.cwd,
			}))
			require.NoError(t, err, "paths.txt line %d", l.n)
			outside := slices.ContainsFunc(l.refs, func(r string) bool { return !strings.HasPrefix(r[min(2, len(r)):], "~/app/") })
			assert.Equal(t, outside, slices.Contains(res.Msg.GetPathOutside(), rule.String()), "paths.txt line %d: outside ~/app", l.n)
			if outside {
				assert.False(t, res.Msg.GetAllow(), "paths.txt line %d", l.n)
				assert.Contains(t, res.Msg.GetReasons(), "path:"+rule.String(), "paths.txt line %d", l.n)
			}
		}
	}
	assert.Equal(t, 1, pathDenials(t, "session.deny", a.RunID, rule))
	assert.Equal(t, "denied", transcript(t, a.RunID)[outsideRead], "the transcript keeps the refused call")
}

// TestRedteamPathRuleOnlyHuman: the same rule for all users refuses a
// person's read outside ~/app with exit 1 and the rule's message.
func TestRedteamPathRuleOnlyHuman(t *testing.T) {
	begin(t)
	pathRule(t, allUsers, appOnly, "any", "only")
	before := humanDenials(t)
	var out string
	var code int
	require.True(t, within(settle, func() bool {
		out, code = aliceExec(t, target, outsideRead)
		return code == 1
	}), "a read outside ~/app is refused")
	assert.Equal(t, outsideDenied+"\r\n", out)
	out, code = aliceExec(t, target, "echo ok")
	assert.Equal(t, "ok\n", out)
	assert.Zero(t, code)
	assert.True(t, within(denialReport, func() bool { return humanDenials(t) > before }), "session.deny of the refused read")
}

// TestRedteamConsoleDeniesOutsidePaths: in the command console a line
// outside ~/app is refused, the console stays and the denial is audited on
// the session; the console recording verifies.
func TestRedteamConsoleDeniesOutsidePaths(t *testing.T) {
	begin(t)
	terminalRule(t, allUsers, "console")
	rule := pathRule(t, allUsers, appOnly, "any", "only")
	require.True(t, within(settle, func() bool { _, code := aliceExec(t, target, outsideRead); return code == 1 }),
		"the rules reach the gateways")
	tm := openTerm(t)
	tm.expect(t, consoleBanner)
	id := shellSession(t)
	tm.mark(t, "ok1")
	tm.send(t, outsideRead)
	tm.expect(t, outsideDenied)
	tm.mark(t, "ok2") // the console lives on after a refused line
	tm.send(t, "exit")
	tm.exited(t)
	assert.True(t, within(denialReport, func() bool { return pathDenials(t, "session.command_denied", id, rule) == 1 }),
		"session.command_denied with reason path:<id> on the session")
	st.WaitVerified(t, id, cluster.UploadDrain)
}

// TestRedteamConsoleRuleRevokesRawShell: a new console rule ends an open raw
// shell within revokeMax (access_revoked); the next shell is the console; a
// deny rule refuses the shell with exit 1.
func TestRedteamConsoleRuleRevokesRawShell(t *testing.T) {
	begin(t)
	sh := cluster.OpenShell(t, st.Dial(t, st.Addr(t, "nginx", 2222), "alice+"+target))
	sh.Mark(t, "raw")
	id := shellSession(t)
	from := dbNow(t)
	console := terminalRule(t, allUsers, "console")
	endedWithin(t, id, from)
	openTerm(t).expect(t, consoleBanner)

	deleteTerminalRule(t, console)
	terminalRule(t, allUsers, "deny")
	var tm *term
	require.True(t, within(settle, func() bool { // until the rule arrives the shell opens as the console
		tm = openTerm(t)
		return tm.ended(revokeMax) && tm.code == 1
	}), "a deny rule refuses the shell")
	assert.Contains(t, tm.out.String(), shellDenied)
}

// pathLine is one "<refs>\t<cwd>\t<command>" line of the path corpus; n is its line number.
type pathLine struct {
	n            int
	refs         []string
	cwd, command string
}

// pathCorpus returns the lines of section ("# <section>" opens it) of the
// path corpus of stage 11b.
func pathCorpus(t *testing.T, section string) []pathLine {
	t.Helper()
	f, err := os.Open("../../internal/platform/shellclass/testdata/paths.txt")
	require.NoError(t, err)
	defer f.Close()
	var out []pathLine
	in := false
	sc := bufio.NewScanner(f)
	for n := 1; sc.Scan(); n++ {
		if name, ok := strings.CutPrefix(sc.Text(), "# "); ok {
			in = name == section || in && strings.Contains(name, " ") // a comment line keeps the section
			continue
		}
		if f := strings.SplitN(sc.Text(), "\t", 3); in && len(f) == 3 {
			out = append(out, pathLine{n: n, refs: strings.Fields(f[0]), cwd: f[1], command: f[2]})
		}
	}
	require.NoError(t, sc.Err())
	require.NotEmpty(t, out, "path corpus section %s", section)
	return out
}

// pathDenials counts the audit events action of id (the session, or the
// agent run) whose data.reasons holds path:<rule>.
func pathDenials(t *testing.T, action string, id, rule uuid.UUID) int {
	t.Helper()
	var n int
	require.NoError(t, st.DB.QueryRow(context.Background(), `SELECT count(*) FROM audit_event WHERE action = $1
		AND (target_id = $2 OR data->>'agent_run_id' = $2) AND data->'reasons' ? $3`, action, id.String(), "path:"+rule.String()).Scan(&n))
	return n
}

// humanDenials counts alice's session.deny events of outsideRead.
func humanDenials(t *testing.T) int {
	t.Helper()
	var n int
	require.NoError(t, st.DB.QueryRow(context.Background(), `SELECT count(*) FROM audit_event WHERE action = 'session.deny'
		AND actor_user_id = $1 AND data->>'target' = $2 AND NOT data ? 'agent_run_id'`, st.AliceID, outsideRead).Scan(&n))
	return n
}

// term is an interactive session of alice on target on an xterm 80x24 PTY.
type term struct {
	in   io.Writer
	out  *lockedBuffer
	from int // expect looks after this offset
	done chan struct{}
	code int // exit status once done; -1 without one
}

// openTerm opens a shell of alice on target; it closes with the test.
func openTerm(t *testing.T) *term {
	t.Helper()
	sess, err := st.Dial(t, st.Addr(t, "nginx", 2222), "alice+"+target).NewSession()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sess.Close() })
	require.NoError(t, sess.RequestPty("xterm", 24, 80, gossh.TerminalModes{gossh.ECHO: 1}))
	in, err := sess.StdinPipe()
	require.NoError(t, err)
	tm := &term{in: in, out: &lockedBuffer{}, done: make(chan struct{}), code: -1}
	sess.Stdout, sess.Stderr = tm.out, tm.out
	require.NoError(t, sess.Shell())
	go func() {
		err := sess.Wait()
		if ee, ok := errors.AsType[*gossh.ExitError](err); ok {
			tm.code = ee.ExitStatus()
		} else if err == nil {
			tm.code = 0
		}
		close(tm.done) // code is read only after done
	}()
	return tm
}

// send types line and Enter.
func (tm *term) send(t *testing.T, line string) {
	t.Helper()
	_, err := io.WriteString(tm.in, line+"\r")
	require.NoError(t, err)
}

// expect waits termWait for substr in the output after the last match.
func (tm *term) expect(t *testing.T, substr string) {
	t.Helper()
	found := within(termWait, func() bool {
		i := strings.Index(tm.out.String()[tm.from:], substr)
		if i >= 0 {
			tm.from += i + len(substr)
		}
		return i >= 0
	})
	if !found {
		t.Fatalf("%q not printed within %s; output:\n%s", substr, termWait, tm.out.String()[tm.from:])
	}
}

// mark runs printf of "mark:<word>" and waits for it: only the output carries it.
func (tm *term) mark(t *testing.T, word string) {
	t.Helper()
	tm.send(t, fmt.Sprintf("printf 'mark:%%s\\n' %s", word))
	tm.expect(t, "mark:"+word)
}

// ended reports whether the session ends within d.
func (tm *term) ended(d time.Duration) bool {
	select {
	case <-tm.done:
		return true
	case <-time.After(d):
		return false
	}
}

// exited waits termWait for the session to end and returns its exit status.
func (tm *term) exited(t *testing.T) int {
	t.Helper()
	if !tm.ended(termWait) {
		t.Fatalf("session did not end within %s; output:\n%s", termWait, tm.out.String())
	}
	return tm.code
}

// lockedBuffer is a strings.Builder safe for one writer and concurrent readers.
type lockedBuffer struct {
	mu sync.Mutex
	b  strings.Builder
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.b.Write(p)
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.b.String()
}
