package sshconnector

import (
	"io"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/gateway"
	"github.com/werbot/werbot/internal/gateway/sshtest"
)

func TestTerminalPTYAndResize(t *testing.T) {
	tg := sshtest.StartTarget(t, "pw", nil)
	s := newSession(t, tg, gateway.KindTerminal)
	resize := make(chan gateway.Window)
	user, out, _ := attachTerminal(t, s, "xterm", "", resize)
	served := serve(t.Context(), s)
	waitEvent(t, tg, "pty:xterm 120x40")

	resize <- gateway.Window{Cols: 100, Rows: 50}
	require.NotContains(t, waitEvent(t, tg, "win:100x50"), "win:120x40", "the initial size came back as a change")
	rec := s.Recording.(*recSink)
	require.Eventually(t, func() bool {
		_, _, sizes := rec.get()
		return len(sizes) == 1 && sizes[0] == gateway.Window{Cols: 100, Rows: 50}
	}, waitFor, 10*time.Millisecond)

	_, err := io.WriteString(user, "hi\r")
	require.NoError(t, err)
	require.Eventually(t, func() bool { return strings.Contains(out.String(), "hi") }, waitFor, 10*time.Millisecond)
	in, output, _ := rec.get()
	require.Equal(t, "hi\r", in)
	require.Contains(t, output, "hi")

	require.NoError(t, user.Close())
	require.NoError(t, wait(t, served))
}

func TestTerminalCommand(t *testing.T) {
	tg := sshtest.StartTarget(t, "pw", nil)
	s := newSession(t, tg, gateway.KindTerminal)
	_, out, code := attachTerminal(t, s, "xterm", "hostname", nil)
	require.NoError(t, wait(t, serve(t.Context(), s)))
	require.Equal(t, 0, *code)
	require.Eventually(t, func() bool { return strings.Contains(out.String(), "target") }, waitFor, 10*time.Millisecond)
	waitEvent(t, tg, "pty:xterm 120x40")
}

// The input relay outlives Serve until the front closes the terminal; it must
// not feed the recording the pipeline closes after Serve.
func TestTerminalNoRecordingAfterServe(t *testing.T) {
	tg := sshtest.StartTarget(t, "pw", nil)
	s := newSession(t, tg, gateway.KindTerminal)
	user, _, _ := attachTerminal(t, s, "xterm", "hostname", nil)
	require.NoError(t, wait(t, serve(t.Context(), s)))
	require.NoError(t, user.SetWriteDeadline(time.Now().Add(waitFor)))
	_, err := io.WriteString(user, "late")
	require.NoError(t, err)
	require.Never(t, func() bool { in, _, _ := s.Recording.(*recSink).get(); return in != "" }, 200*time.Millisecond, 10*time.Millisecond)
}

func TestTerminalWithoutPTY(t *testing.T) {
	tg := sshtest.StartTarget(t, "pw", nil)
	s := newSession(t, tg, gateway.KindTerminal)
	user, out, _ := attachTerminal(t, s, "", "", nil)
	served := serve(t.Context(), s)
	_, err := io.WriteString(user, "hi\n")
	require.NoError(t, err)
	require.Eventually(t, func() bool { return out.String() == "hi\n" }, waitFor, 10*time.Millisecond)
	require.NoError(t, user.Close())
	require.NoError(t, wait(t, served))
	for _, e := range waitEvent(t, tg, "closed") {
		require.NotContains(t, e, "pty:")
	}
}

func TestExecExitStatus(t *testing.T) {
	tg := sshtest.StartTarget(t, "pw", nil)
	s := newSession(t, tg, gateway.KindExec)
	stdout, code := attachExec(s, "echo hello", strings.NewReader(""))
	require.NoError(t, wait(t, serve(t.Context(), s)))
	require.Equal(t, "hello\n", stdout.String())
	require.Equal(t, 0, *code)
	_, out, _ := s.Recording.(*recSink).get()
	require.Equal(t, "hello\n", out)

	s = newSession(t, tg, gateway.KindExec)
	_, code = attachExec(s, "exit 3", strings.NewReader(""))
	require.NoError(t, wait(t, serve(t.Context(), s)))
	require.Equal(t, 3, *code)
}

func TestExecStdinEOF(t *testing.T) {
	tg := sshtest.StartTarget(t, "pw", nil)
	s := newSession(t, tg, gateway.KindExec)
	stdout, code := attachExec(s, "cat", strings.NewReader("abc"))
	require.NoError(t, wait(t, serve(t.Context(), s)))
	require.Equal(t, "abc", stdout.String())
	require.Equal(t, 0, *code)
}

// attachTerminal gives s a terminal of 120x40 on a pipe; out collects what the user sees.
// The exit status is -1 until set; read it only after Serve returned.
func attachTerminal(t *testing.T, s *gateway.Session, term, command string, resize chan gateway.Window) (net.Conn, *lockedBuffer, *int) {
	t.Helper()
	user, conn := net.Pipe()
	t.Cleanup(func() { _ = user.Close(); _ = conn.Close() })
	out := &lockedBuffer{}
	go func() { _, _ = io.Copy(out, user) }() // net.Pipe is synchronous: keep reading
	code := -1
	s.Terminal = &gateway.Terminal{ReadWriter: conn, Term: term, Command: command, Window: gateway.Window{Cols: 120, Rows: 40},
		Resize: resize, SetExitStatus: func(c int) { code = c }}
	return user, out, &code
}

// attachExec gives s an exec of command; the exit status is -1 until set.
func attachExec(s *gateway.Session, command string, stdin io.Reader) (*lockedBuffer, *int) {
	stdout, code := &lockedBuffer{}, -1
	s.Exec = &gateway.Exec{Command: command, Stdin: stdin, Stdout: stdout, Stderr: &lockedBuffer{},
		SetExitStatus: func(c int) { code = c }}
	return stdout, &code
}
