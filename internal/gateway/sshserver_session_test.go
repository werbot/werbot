package gateway

import (
	"io"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	gossh "golang.org/x/crypto/ssh"
)

func TestDirectExec(t *testing.T) {
	e := newSSHEnv(t, Config{}) // cfg.Recording nil: the pipeline closes a no-op sink
	out, code := run(t, e.client(t, "alice+db"), false, "uptime")
	require.Equal(t, "fake:db user=alice secret=s3cret-value kind=exec\n", out)
	require.Equal(t, 0, code)
	e.f.with(func(f *fakeControl) {
		require.Equal(t, []string{e.m["u1"].String() + " " + e.m["rdb"].String() + " 127.0.0.1"}, f.starts)
	})
	require.Eventually(t, func() bool { // the spool delivers it
		var n int
		e.f.with(func(f *fakeControl) { n = len(f.ends) })
		return n == 1
	}, 2*time.Second, 5*time.Millisecond)
	e.f.with(func(f *fakeControl) { require.Equal(t, "closed", f.ends[0].reason) })
}

func TestKindMapping(t *testing.T) {
	e := newSSHEnv(t, Config{})
	c := e.client(t, "alice+spy")
	cases := []struct {
		pty  bool
		cmd  string
		want string
		code int
	}{
		{pty: true, want: `spy:spy kind=terminal term="xterm" cmd="" win=120x40`},
		{want: `spy:spy kind=terminal term="" cmd="" win=0x0`},
		{pty: true, cmd: "uptime", want: `spy:spy kind=terminal term="xterm" cmd="uptime" win=120x40`},
		{cmd: "uptime", want: `spy:spy kind=exec cmd="uptime"`, code: 7},
	}
	for _, tc := range cases {
		out, code := run(t, c, tc.pty, tc.cmd)
		require.Contains(t, out, tc.want)
		require.Equal(t, tc.code, code, tc.want)
	}

	conn, err := c.Dial("tcp", "10.0.0.1:80")
	require.NoError(t, err)
	b, err := io.ReadAll(conn)
	require.NoError(t, err)
	require.Equal(t, "spy:spy kind=stream target=10.0.0.1:80\n", string(b))
}

// The initial window comes with pty-req; Resize carries only changes after shell or exec.
func TestResizeSkipsInitialWindow(t *testing.T) {
	e := newSSHEnv(t, Config{})
	s, err := e.client(t, "alice+spy").NewSession()
	require.NoError(t, err)
	require.NoError(t, s.RequestPty("xterm", 40, 120, gossh.TerminalModes{}))
	require.NoError(t, s.WindowChange(30, 80)) // before exec: dropped
	require.NoError(t, s.WindowChange(31, 81))
	out := &syncBuffer{}
	s.Stdout = out
	require.NoError(t, s.Start("resize"))
	require.Eventually(t, func() bool { return strings.Contains(out.String(), "spy:spy") }, 2*time.Second, 5*time.Millisecond)
	require.NoError(t, s.WindowChange(50, 100))
	require.NoError(t, s.Wait())
	require.Contains(t, out.String(), "resize=100x50")
}

func TestDeniedAndUnknownLookSame(t *testing.T) {
	e := newSSHEnv(t, Config{})
	for _, user := range []string{"alice+denied", "alice+missing"} {
		out, code := run(t, e.client(t, user), false, "uptime")
		require.Equal(t, "Access denied\r\n", out, user)
		require.Equal(t, 1, code, user)
	}
	require.Zero(t, e.startCalls())
}

func TestBusy(t *testing.T) {
	e := newSSHEnv(t, Config{MaxSessions: 1})
	s, err := e.client(t, "alice+blocker").NewSession()
	require.NoError(t, err)
	require.NoError(t, s.Shell())
	require.Eventually(t, func() bool { return e.startCalls() == 1 }, 2*time.Second, 10*time.Millisecond)

	out, code := run(t, e.client(t, "alice+db"), false, "uptime")
	require.Equal(t, "Gateway is busy, try again later\r\n", out)
	require.Equal(t, 1, code)
}

func TestUnsupportedTypeAndKind(t *testing.T) {
	e := newSSHEnv(t, Config{})
	out, code := run(t, e.client(t, "alice+nope"), false, "uptime")
	require.Equal(t, "Resource type \"nope\" is not supported by this gateway\r\n", out)
	require.Equal(t, 1, code)
	out, code = run(t, e.client(t, "alice+tcp"), false, "")
	require.Equal(t, "Resource tcp does not support terminal sessions\r\n", out)
	require.Equal(t, 1, code)
}

func TestMenuLoginWithoutPTYPrintsUsage(t *testing.T) {
	e := newSSHEnv(t, Config{})
	out, code := run(t, e.client(t, "alice"), false, "uptime")
	require.Contains(t, out, "Use ssh <login>+<resource>@")
	require.Equal(t, 1, code)
	require.Zero(t, e.startCalls())
}

const menuPrompt = "Select resource or press Enter to exit > "

// menu opens a PTY shell on c, types input and EOF once the prompt shows and returns the output and exit code.
func menu(t *testing.T, c *gossh.Client, input string) (string, int) {
	t.Helper()
	s, err := c.NewSession()
	require.NoError(t, err)
	require.NoError(t, s.RequestPty("xterm", 40, 120, gossh.TerminalModes{}))
	in, err := s.StdinPipe()
	require.NoError(t, err)
	out := &syncBuffer{}
	s.Stdout = out
	require.NoError(t, s.Shell())
	done := make(chan error, 1)
	go func() { done <- s.Wait() }()
	var ended bool
	var waitErr error
	require.Eventually(t, func() bool {
		select {
		case waitErr = <-done: // ended before the prompt
			ended = true
			return true
		default:
			return strings.Contains(out.String(), menuPrompt)
		}
	}, 5*time.Second, 5*time.Millisecond)
	if !ended {
		_, err = io.WriteString(in, input)
		require.NoError(t, err)
		require.NoError(t, in.Close()) // EOF after the input: a menu that waits for more ends
		select {
		case waitErr = <-done:
		case <-time.After(2 * time.Second):
			t.Fatalf("the menu did not end after input %q", input)
		}
	}
	return out.String(), exitCode(t, waitErr)
}

func TestMenuSelectsResource(t *testing.T) {
	e := newSSHEnv(t, Config{})
	e.f.with(func(f *fakeControl) {
		f.policy = e.compile(t, res{"rdb", "db", "fake"}, res{"rweb", "web", "fake"}, res{"rdenied", "denied", "fake"})
	})
	out, code := menu(t, e.client(t, "alice"), "2\r")
	require.Contains(t, out, "Hello alice, you have access to:\r\n  1  db\r\n  2  web\r\n"+menuPrompt)
	require.Contains(t, out, "fake:web user=alice")
	require.Equal(t, 0, code)

	out, code = menu(t, e.client(t, "alice"), "\r")
	require.Contains(t, out, "Bye\r\n")
	require.Equal(t, 0, code)

	for _, in := range []string{"3\r", "x\r", "0\r"} {
		out, code = menu(t, e.client(t, "alice"), in)
		require.Contains(t, out, "Resource not found\r\n", in)
		require.Equal(t, 1, code, in)
	}

	e.f.with(func(f *fakeControl) { f.policy = e.compile(t, res{"rdenied", "denied", "fake"}) })
	out, code = menu(t, e.client(t, "alice"), "")
	require.Equal(t, "You don't have access to any resources\r\n", out)
	require.Equal(t, 1, code)
	require.Equal(t, 1, e.startCalls())
}

// Bytes typed after the selection reach the connector: the menu reads one byte
// at a time, so it never buffers past the selected line.
func TestMenuKeepsTypeahead(t *testing.T) {
	e := newSSHEnv(t, Config{})
	e.f.with(func(f *fakeControl) { f.policy = e.compile(t, res{"rspy", "typeahead", "spy"}) })
	out, code := menu(t, e.client(t, "alice"), "1\rhello\r")
	require.Contains(t, out, `typed="hello\r"`)
	require.Equal(t, 0, code)
}

// An escape sequence that never completes fills term.Terminal's buffer; the
// menu ends instead of spinning on empty reads.
func TestMenuEndsOnOverlongInput(t *testing.T) {
	e := newSSHEnv(t, Config{})
	out, code := menu(t, e.client(t, "alice"), "\x1b"+strings.Repeat("1", 300))
	require.Contains(t, out, "Bye\r\n")
	require.Equal(t, 0, code)
	require.Zero(t, e.startCalls())
}
