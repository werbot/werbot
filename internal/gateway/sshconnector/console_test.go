//go:build unix

package sshconnector

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/gateway"
	"github.com/werbot/werbot/internal/gateway/sshtest"
)

const consolePrompt = "alice@target:~$ "

func TestServeConsoleEndToEnd(t *testing.T) {
	tg, _ := consoleTarget(t)
	s := consoleSession(t, tg)
	user, out, code, served := startConsole(t.Context(), t, s)
	waitScreen(t, out, consolePrompt)
	typeLine(t, user, out, "cat app/a.txt", "file text\r\n")
	typeLine(t, user, out, "cd app", "alice@target:~/app$ ")
	typeLine(t, user, out, "exit", "")
	require.NoError(t, wait(t, served))
	require.Equal(t, 0, *code)
	_, recorded, _ := s.Recording.(*recSink).get()
	require.Contains(t, recorded, "file text\r\n", "the line output is not recorded")
}

// A line runs only in the checked cwd: when it is gone on the target, the
// line ends with code 1 and runs nothing.
func TestConsoleCwdGoneDoesNotRun(t *testing.T) {
	tg, home := consoleTarget(t)
	user, out, _, _ := startConsole(t.Context(), t, consoleSession(t, tg))
	waitScreen(t, out, consolePrompt)
	typeLine(t, user, out, "cd app", "alice@target:~/app$ ")
	require.NoError(t, os.RemoveAll(filepath.Join(home, "app")))
	typeLine(t, user, out, "touch marker", "[1] alice@target:~/app$ ")
	require.NoFileExists(t, filepath.Join(home, "marker"))
}

// Each line is an exec without a PTY and with stdin closed.
func TestConsoleExecNoPTYStdinClosed(t *testing.T) {
	tg, _ := consoleTarget(t)
	user, out, _, served := startConsole(t.Context(), t, consoleSession(t, tg))
	waitScreen(t, out, consolePrompt)
	_, err := io.WriteString(user, "cat\r")
	require.NoError(t, err)
	require.Eventually(t, func() bool { return strings.Count(out.String(), consolePrompt) == 2 }, waitFor, 10*time.Millisecond,
		"cat did not end with code 0")
	typeLine(t, user, out, "exit", "")
	require.NoError(t, wait(t, served))
	events := drainEvents(tg)
	require.Contains(t, events, "closed")
	for _, e := range events {
		require.NotContains(t, e, "pty:")
	}
}

// Ctrl-C during a line sends INT to it; the prompt shows 128 + 2.
func TestConsoleInterruptSignals(t *testing.T) {
	tg, _ := consoleTarget(t)
	user, out, _, _ := startConsole(t.Context(), t, consoleSession(t, tg))
	waitScreen(t, out, consolePrompt)
	typeLine(t, user, out, "echo started; sleep 30", "started\r\n")
	_, err := user.Write([]byte{0x03})
	require.NoError(t, err)
	waitEvent(t, tg, "signal:INT")
	require.Eventually(t, func() bool { return strings.Contains(out.String(), "[130] "+consolePrompt) }, 2*time.Second, 10*time.Millisecond)
}

// A second Ctrl-C closes the channel of a line that ignores INT.
func TestConsoleSecondCtrlCClosesChannel(t *testing.T) {
	tg, _ := consoleTarget(t)
	user, out, _, _ := startConsole(t.Context(), t, consoleSession(t, tg))
	waitScreen(t, out, consolePrompt)
	typeLine(t, user, out, "trap '' INT; echo started; sleep 3", "started\r\n")
	_, err := user.Write([]byte{0x03})
	require.NoError(t, err)
	waitEvent(t, tg, "signal:INT")
	_, err = user.Write([]byte{0x03})
	require.NoError(t, err)
	require.Eventually(t, func() bool { return strings.Contains(out.String(), "^C\r\n[130] "+consolePrompt) }, 2*time.Second, 10*time.Millisecond)
}

// A cancel of the session closes the target and is not an error.
func TestServeConsoleCancel(t *testing.T) {
	tg, _ := consoleTarget(t)
	ctx, cancel := context.WithCancel(t.Context())
	user, out, _, served := startConsole(ctx, t, consoleSession(t, tg))
	waitScreen(t, out, consolePrompt)
	typeLine(t, user, out, "echo started; sleep 3", "started\r\n")
	cancel()
	select {
	case err := <-served:
		require.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("ServeConsole did not return within 2s of the cancel")
	}
}

func TestServeConsoleHostKeyPinned(t *testing.T) {
	tg, _ := consoleTarget(t)
	s := consoleSession(t, tg)
	var alg string
	var key []byte
	s.PinKey = func(a string, k []byte) error { alg, key = a, k; return nil }
	user, out, _, served := startConsole(t.Context(), t, s)
	waitScreen(t, out, consolePrompt)
	typeLine(t, user, out, "exit", "")
	require.NoError(t, wait(t, served))
	require.Equal(t, "ssh-ed25519", alg)
	require.Equal(t, tg.HostKey.Marshal(), key)

	s = consoleSession(t, tg)
	s.PinKey = func(string, []byte) error { return fmt.Errorf("%w: pinned key differs", gateway.ErrHostKeyMismatch) }
	attachTerminal(t, s, "xterm", "", nil)
	require.ErrorIs(t, Connector{DialTimeout: waitFor}.ServeConsole(t.Context(), s), gateway.ErrHostKeyMismatch)
}

// consoleTarget starts a target running exec with /bin/sh in a new home that
// holds app/a.txt.
func consoleTarget(t *testing.T) (*sshtest.Target, string) {
	t.Helper()
	home := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(home, "app"), 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(home, "app", "a.txt"), []byte("file text\n"), 0o600))
	return sshtest.StartTarget(t, "pw", nil, sshtest.Shell(home)), home
}

// consoleSession is a console session on tg where every line is allowed.
func consoleSession(t *testing.T, tg *sshtest.Target) *gateway.Session {
	t.Helper()
	s := newSession(t, tg, gateway.KindTerminal)
	s.Console = true
	s.CheckCommand = func(context.Context, string, string, string, io.Writer) (gateway.CommandCheck, error) {
		return gateway.CommandCheck{Classes: []string{"read"}}, nil
	}
	return s
}

// startConsole gives s a terminal and serves its console; code is the exit
// status, read it only after ServeConsole returned.
func startConsole(ctx context.Context, t *testing.T, s *gateway.Session) (net.Conn, *lockedBuffer, *int, <-chan error) {
	t.Helper()
	user, out, code := attachTerminal(t, s, "xterm", "", nil)
	served := make(chan error, 1)
	go func() { served <- Connector{DialTimeout: waitFor}.ServeConsole(ctx, s) }()
	return user, out, code, served
}

// typeLine types line and Enter, then waits until the screen shows want.
func typeLine(t *testing.T, user net.Conn, out *lockedBuffer, line, want string) {
	t.Helper()
	require.NoError(t, user.SetWriteDeadline(time.Now().Add(waitFor)))
	_, err := io.WriteString(user, line+"\r")
	require.NoError(t, err)
	waitScreen(t, out, want)
}

func waitScreen(t *testing.T, out *lockedBuffer, want string) {
	t.Helper()
	require.Eventually(t, func() bool { return strings.Contains(out.String(), want) }, waitFor, 10*time.Millisecond,
		"the screen has no %q", want)
}

// drainEvents is the target events seen so far.
func drainEvents(tg *sshtest.Target) []string {
	var events []string
	for {
		select {
		case e := <-tg.Events:
			events = append(events, e)
		default:
			return events
		}
	}
}
