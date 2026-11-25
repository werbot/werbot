//go:build unix

package app

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	gossh "golang.org/x/crypto/ssh"

	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
	"github.com/werbot/werbot/internal/gateway/sshtest"
)

const outsideDenied = "Command denied: path outside allowed folders\r\n"

// shellStack is a stack whose target runs commands with /bin/sh in home,
// with resource srv bound to alice. home (symlinks resolved, as pwd -P
// prints it) holds app/a.txt with "a\n".
func shellStack(t *testing.T) (s *stack, srv uuid.UUID, home string) {
	t.Helper()
	home, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)
	require.NoError(t, os.Mkdir(filepath.Join(home, "app"), 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(home, "app", "a.txt"), []byte("a\n"), 0o600))
	s = startStack(t, sshtest.Shell(home))
	srv = s.addResource(t, "srv", "ssh")
	require.Eventually(t, func() bool { return s.allowed(t, s.key, "alice+srv") }, settle, 100*time.Millisecond)
	return s, srv, home
}

// onlyApp creates the path rule "only ~/app/** for any access" for every
// user on srv, waits until the gateway refuses a path outside it and returns
// its id. A snapshot holds every rule, so the rules created before it have
// reached the gateway too.
func (s *stack) onlyApp(t *testing.T, srv uuid.UUID) string {
	t.Helper()
	res, err := s.api.pol.CreatePathRule(context.Background(), connect.NewRequest(&werbotv1.CreatePathRuleRequest{
		Name: "only-app", Paths: []string{"~/app/**"}, Access: "any", Effect: "only",
		Subjects: &werbotv1.Subjects{AllUsers: true}, ResourceSelector: &werbotv1.ResourceSelector{ResourceIds: []string{srv.String()}}}))
	require.NoError(t, err)
	require.Eventually(t, func() bool {
		_, stderr, code := s.execSplit(t, "cat /etc/hosts")
		return stderr == outsideDenied && code == 1
	}, 5*time.Second, 100*time.Millisecond)
	return res.Msg.GetRule().GetId()
}

// terminalRule creates a terminal rule of mode for every user on srv.
func (s *stack) terminalRule(t *testing.T, srv uuid.UUID, mode string) {
	t.Helper()
	_, err := s.api.pol.CreateTerminalRule(context.Background(), connect.NewRequest(&werbotv1.CreateTerminalRuleRequest{
		Name: "terminal-" + mode, Mode: mode,
		Subjects: &werbotv1.Subjects{AllUsers: true}, ResourceSelector: &werbotv1.ResourceSelector{ResourceIds: []string{srv.String()}}}))
	require.NoError(t, err)
}

// pty opens a session with a PTY as alice on srv, a shell when cmd is empty
// and an exec otherwise, with stdin at EOF, and returns its output and exit
// code. A connection or channel failure returns its text and -1.
func (s *stack) pty(t *testing.T, cmd string) (out string, code int) {
	t.Helper()
	c, _, err := s.dial(s.gwSSH, "alice+srv", s.key)
	if err != nil {
		return err.Error(), -1
	}
	defer c.Close()                                                  //nolint:errcheck // test client
	hung := time.AfterFunc(15*time.Second, func() { _ = c.Close() }) // a hung session returns -1
	defer hung.Stop()
	sess, err := c.NewSession()
	if err != nil {
		return err.Error(), -1
	}
	defer sess.Close() //nolint:errcheck // closed by the server after exit
	var stdout, stderr bytes.Buffer
	sess.Stdout, sess.Stderr = &stdout, &stderr
	start := func() error { return sess.Start(cmd) }
	if cmd == "" {
		start = sess.Shell
	}
	if err = sess.RequestPty("xterm", 40, 120, gossh.TerminalModes{}); err == nil {
		if err = start(); err == nil {
			err = sess.Wait()
		}
	}
	return stdout.String() + stderr.String(), exitCode(err)
}

// exitCode is the exit status of a finished session: 0, the remote code, or
// -1 for any other error.
func exitCode(err error) int {
	var ee *gossh.ExitError
	switch {
	case err == nil:
		return 0
	case errors.As(err, &ee):
		return ee.ExitStatus()
	}
	return -1
}

// consoleClient is a PTY shell whose output the test reads line by line.
type consoleClient struct {
	t     *testing.T
	sess  *gossh.Session
	stdin io.Writer
	out   *syncBuffer
	read  int // output bytes already matched by expect
}

// console opens a PTY shell as alice on addr as user.
func (s *stack) console(t *testing.T, addr, user string) *consoleClient {
	t.Helper()
	c, _, err := s.dial(addr, user, s.key)
	require.NoError(t, err)
	t.Cleanup(func() { _ = c.Close() })
	sess, err := c.NewSession()
	require.NoError(t, err)
	require.NoError(t, sess.RequestPty("xterm", 40, 120, gossh.TerminalModes{}))
	stdin, err := sess.StdinPipe()
	require.NoError(t, err)
	cc := &consoleClient{t: t, sess: sess, stdin: stdin, out: &syncBuffer{}}
	sess.Stdout, sess.Stderr = cc.out, cc.out
	require.NoError(t, sess.Shell())
	return cc
}

// expect waits up to 10s for want in the output after the last match.
func (c *consoleClient) expect(want string) {
	c.t.Helper()
	var got string
	ok := assert.Eventually(c.t, func() bool {
		got = c.out.String()[c.read:]
		return strings.Contains(got, want)
	}, 10*time.Second, 10*time.Millisecond)
	if !ok {
		c.t.Fatalf("console output lacks %q after 10s: %q", want, got)
	}
	c.read += strings.Index(got, want) + len(want)
}

// send types line and Enter.
func (c *consoleClient) send(line string) {
	c.t.Helper()
	_, err := io.WriteString(c.stdin, line+"\r")
	require.NoError(c.t, err)
}

// close waits up to 10s for the session to end and returns its exit code.
func (c *consoleClient) close() int {
	c.t.Helper()
	done := make(chan error, 1)
	go func() { done <- c.sess.Wait() }()
	select {
	case err := <-done:
		return exitCode(err)
	case <-time.After(10 * time.Second):
		c.t.Fatal("console still open after 10s")
	}
	return -1
}

// recording waits up to 20s for the verified recording of session sid.
func (s *stack) recording(t *testing.T, sid string) string {
	t.Helper()
	hc := s.aliceHTTP(t)
	var body []byte
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, s.url+"/api/recordings/"+sid+".cast", nil)
		require.NoError(c, err)
		res, err := hc.Do(req)
		require.NoError(c, err)
		defer res.Body.Close() //nolint:errcheck // test client
		body, err = io.ReadAll(res.Body)
		require.NoError(c, err)
		require.Equal(c, http.StatusOK, res.StatusCode)
		assert.Equal(c, "true", res.Header.Get("X-Werbot-Recording-Verified"))
	}, 20*time.Second, 200*time.Millisecond)
	return string(body)
}
