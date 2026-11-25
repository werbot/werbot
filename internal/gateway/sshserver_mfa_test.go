package gateway

import (
	"bytes"
	"io"
	"regexp"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	gossh "golang.org/x/crypto/ssh"
)

func TestNoneAuthRejected(t *testing.T) {
	e, fm := mfaSSH(t)
	_, err := gossh.Dial("tcp", e.addr, &gossh.ClientConfig{User: "alice+db", HostKeyCallback: gossh.FixedHostKey(e.host.PublicKey()),
		Timeout: 5 * time.Second})
	require.ErrorContains(t, err, "unable to authenticate")
	e.f.with(func(f *fakeControl) { assert.Empty(t, f.authCalls) })
	requireNoChallenge(t, fm)
}

func TestKeyboardInteractiveOnlyAfterPublicKey(t *testing.T) {
	e, fm := mfaSSH(t)
	ki := &kiClient{answers: []string{"123456"}}
	_, err := gossh.Dial("tcp", e.addr, &gossh.ClientConfig{User: "alice+db", Auth: []gossh.AuthMethod{gossh.KeyboardInteractive(ki.challenge)},
		HostKeyCallback: gossh.FixedHostKey(e.host.PublicKey()), Timeout: 5 * time.Second})
	require.ErrorContains(t, err, "unable to authenticate")
	assert.Empty(t, ki.rounds, "no prompt round without a verified key")
	requireNoChallenge(t, fm)
}

func TestMFAPartialSuccessTOTP(t *testing.T) {
	e, fm := mfaSSH(t)
	fm.verify = func(code string) (bool, error) { return code == "123456", nil }
	ki := &kiClient{answers: []string{"123456"}}
	c, err := e.dialMFA("alice+db", ki, &syncBuffer{})
	require.NoError(t, err)
	defer c.Close() //nolint:errcheck // test client
	out, code := run(t, c, false, "uptime")
	require.Equal(t, 0, code)
	require.Contains(t, out, "fake:db user=alice")

	assert.Equal(t, [][]string{{"", "", testPromptAny}}, ki.rounds, "one round: empty name and instruction")
	assert.Equal(t, [][]bool{{false}}, ki.echos)
	creates, codes, _ := fm.calls()
	require.Len(t, creates, 1)
	assert.Equal(t, challengeRequest{UserID: e.m["u1"], ResourceID: e.m["rdb"], SSHSessionID: c.SessionID(),
		ClientIP: creates[0].ClientIP, Binding: "key:" + fp(e.keys["alice"]), Strength: "any"}, creates[0])
	assert.Equal(t, "127.0.0.1", creates[0].ClientIP.String())
	assert.Equal(t, []string{"123456"}, codes)
}

func TestMFAWrongTOTPReprompts(t *testing.T) {
	e, fm := mfaSSH(t)
	fm.verify = func(code string) (bool, error) {
		if code == "111111" {
			return false, connect.NewError(connect.CodeResourceExhausted, nil)
		}
		return false, nil
	}
	ki := &kiClient{answers: []string{"abc", "000000", "111111"}}
	banners := &syncBuffer{}
	_, err := e.dialMFA("alice+db", ki, banners)
	require.ErrorContains(t, err, "unable to authenticate")
	assert.Equal(t, "Too many MFA attempts, try again later\r\n", banners.String())
	require.Len(t, ki.rounds, 3, "a malformed and a wrong code ask again")
	for _, r := range ki.rounds {
		assert.Equal(t, []string{"", "", testPromptAny}, r)
	}
	_, codes, _ := fm.calls()
	assert.Equal(t, []string{"000000", "111111"}, codes, "a malformed answer is not checked")
	assert.Zero(t, e.startCalls())
}

// An expired challenge refuses the login with the timeout text as an auth banner.
func TestMFAExpiredBanner(t *testing.T) {
	e, fm := mfaSSH(t)
	fm.status = func(int) string { return "expired" }
	banners := &syncBuffer{}
	_, err := e.dialMFA("alice+db", &kiClient{}, banners)
	require.ErrorContains(t, err, "unable to authenticate")
	assert.Equal(t, "Permission denied (MFA timeout)\r\n", banners.String())
}

func TestVerifiedConnectionChannels(t *testing.T) {
	e, fm := mfaSSH(t)
	e.setMFARule(t, "rblocker", "blocker")
	fm.verify = func(string) (bool, error) { return true, nil }
	c, err := e.dialMFA("alice+blocker", &kiClient{answers: []string{"123456"}}, &syncBuffer{})
	require.NoError(t, err)
	defer c.Close() //nolint:errcheck // test client

	for range 2 {
		s, err := c.NewSession()
		require.NoError(t, err)
		require.NoError(t, s.Start("sleep"))
		defer s.Close() //nolint:errcheck // test session
	}
	shell, err := c.NewSession()
	require.NoError(t, err)
	require.NoError(t, shell.RequestPty("xterm", 40, 120, gossh.TerminalModes{}))
	require.NoError(t, shell.Shell())
	defer shell.Close() //nolint:errcheck // test session
	fwd, err := c.Dial("tcp", "blocker:80")
	require.NoError(t, err, "a port forward needs no new challenge")
	defer fwd.Close() //nolint:errcheck // test channel
	// tracked after StartSession
	require.Eventually(t, func() bool { return len(e.p.active.ids()) == 4 }, 2*time.Second, 10*time.Millisecond)

	creates, _, _ := fm.calls()
	assert.Len(t, creates, 1, "one challenge for the connection")
	e.p.active.mu.Lock()
	defer e.p.active.mu.Unlock()
	actions := map[string]int{}
	for _, tr := range e.p.active.sessions {
		assert.Equal(t, int64(0), tr.req.MFAAgeSeconds)
		assert.Equal(t, "totp", tr.req.MFAMethod)
		actions[tr.req.Action]++
	}
	assert.Equal(t, map[string]int{"": 3, "blocker.shell": 1}, actions, "the shell step carries the factor too")
}

func TestMenuMFAPrompt(t *testing.T) {
	e, fm := mfaSSH(t)
	fm.status = confirmedFrom(1)
	c := e.client(t, "alice")
	s, err := c.NewSession()
	require.NoError(t, err)
	defer s.Close() //nolint:errcheck // test session
	require.NoError(t, s.RequestPty("xterm", 40, 120, gossh.TerminalModes{}))
	in, err := s.StdinPipe()
	require.NoError(t, err)
	out := &syncBuffer{}
	s.Stdout = out
	require.NoError(t, s.Shell())
	waitOut(t, out, menuPrompt)
	n := regexp.MustCompile(`(\d+)  db`).FindStringSubmatch(out.String())
	require.NotNil(t, n, out.String())
	_, err = io.WriteString(in, n[1]+"\r")
	require.NoError(t, err)
	waitOut(t, out, testPromptAny)
	requireNoStart(t, e)
	_, err = io.WriteString(in, "\r")
	require.NoError(t, err)
	waitOut(t, out, "fake:db user=alice")
}

func TestUnverifiedConnectionChannels(t *testing.T) {
	e, fm := mfaSSH(t)
	e.setMFARule(t, "", "")
	c := e.client(t, "alice+db") // no rule at the handshake: no keyboard-interactive
	e.setMFARule(t, "rdb", "fake")
	fm.status = confirmedFrom(1)

	s, err := c.NewSession()
	require.NoError(t, err)
	var stdout, stderr bytes.Buffer
	s.Stdout, s.Stderr = &stdout, &stderr
	require.NoError(t, s.Run("uptime"), "the exec waits for the browser confirmation")
	assert.Equal(t, testPromptWebAuthn+"\r\n", stderr.String(), "an exec without a PTY cannot read a TOTP code: none offered")
	assert.Contains(t, stdout.String(), "fake:db user=alice")

	_, err = c.Dial("tcp", "db:80")
	require.ErrorContains(t, err, "MFA required, reconnect to confirm")
	creates, _, _ := fm.calls()
	assert.Len(t, creates, 1, "a port forward creates no challenge")
}

func TestServerUnavailableNoMFAStep(t *testing.T) {
	e, fm := mfaSSH(t)
	e.f.with(func(f *fakeControl) { f.authErr = connect.NewError(connect.CodeUnavailable, nil) })
	ki := &kiClient{}
	c, err := e.dialMFA("alice+db", ki, &syncBuffer{})
	require.NoError(t, err)
	defer c.Close() //nolint:errcheck // test client
	out, code := run(t, c, false, "uptime")
	assert.Equal(t, "Gateway is not ready, try again later\r\n", out)
	assert.Equal(t, 1, code)
	assert.Empty(t, ki.rounds)
	requireNoChallenge(t, fm)
}

// A resource denied without an MFA rule and an unknown one still read the same: no prompt, "Access denied".
func TestMFADeniedAndUnknownLookSame(t *testing.T) {
	e, fm := mfaSSH(t)
	for _, user := range []string{"alice+denied", "alice+missing"} {
		ki := &kiClient{}
		c, err := e.dialMFA(user, ki, &syncBuffer{})
		require.NoError(t, err, user)
		out, code := run(t, c, false, "uptime")
		assert.Equal(t, "Access denied\r\n", out, user)
		assert.Equal(t, 1, code, user)
		assert.Empty(t, ki.rounds, user)
		_ = c.Close()
	}
	requireNoChallenge(t, fm)
}

// waitOut waits until out holds want.
func waitOut(t *testing.T, out *syncBuffer, want string) {
	t.Helper()
	require.Eventually(t, func() bool { return strings.Contains(out.String(), want) }, 5*time.Second, 5*time.Millisecond, out.String())
}

func requireNoStart(t *testing.T, e *sshEnv) {
	t.Helper()
	assert.Zero(t, e.startCalls(), "nothing starts before the factor")
}
