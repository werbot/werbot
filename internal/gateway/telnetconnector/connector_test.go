package telnetconnector_test

import (
	"context"
	"crypto/rand"
	"io"
	"log/slog"
	"net"
	"net/netip"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/gateway"
	"github.com/werbot/werbot/internal/gateway/connectortest"
	"github.com/werbot/werbot/internal/gateway/telnetconnector"
	"github.com/werbot/werbot/internal/gateway/telnettest"
)

const waitFor = 5 * time.Second

func TestContract(t *testing.T) {
	registered, ok := gateway.LookupConnector("telnet")
	require.True(t, ok)
	require.Equal(t, telnetconnector.Connector{}, registered)
	connectortest.Run(t, telnetconnector.Connector{}, telnettest.Target(t))
}

// env is one session against a telnet target, driven as a user.
type env struct {
	srv   *telnettest.Server
	s     *gateway.Session
	rec   *spyRec
	out   *lockedBuffer // what the user sees
	in    io.Writer     // what the user types
	resz  func(cols, rows int)
	dials atomic.Int32
	err   chan error
}

// start serves a session of kind with the resource login and secret; the user
// window is 132×40 with TERM vt100. A stream user is a telnet client.
func start(t *testing.T, kind gateway.Kind, login, secret string) *env {
	t.Helper()
	pw := "werbot-telnet-secret-" + rand.Text()
	e := &env{srv: telnettest.Start(t, pw), rec: &spyRec{}, out: &lockedBuffer{}, err: make(chan error, 1)}
	if secret == "<target>" {
		secret = pw
	}
	dialer := &net.Dialer{Timeout: waitFor}
	e.s = &gateway.Session{
		ID: uuid.New(), Kind: kind, Front: "test", User: gateway.User{ID: uuid.New(), Login: "alice"},
		Resource: e.srv.Resource(t, login), ClientIP: netip.MustParseAddr("192.0.2.1"), Recording: e.rec,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			e.dials.Add(1)
			return dialer.DialContext(ctx, network, address)
		},
	}
	if secret != "" {
		e.s.Secret = gateway.Secret{Kind: "password", Value: []byte(secret)}
	}
	user, conn := net.Pipe()
	if kind == gateway.KindStream {
		user, conn = tcpPair(t) // buffered like an SSH channel: both telnet sides write while the other writes
	}
	t.Cleanup(func() { _ = user.Close() })
	switch kind {
	case gateway.KindTerminal:
		resize := make(chan gateway.Window, 1)
		e.s.Terminal = &gateway.Terminal{ReadWriter: conn, Term: "vt100", Window: gateway.Window{Cols: 132, Rows: 40},
			Resize: resize, SetExitStatus: func(int) {}}
		e.in = user
		e.resz = func(cols, rows int) { resize <- gateway.Window{Cols: cols, Rows: rows} }
		go func() { _, _ = io.Copy(e.out, user) }()
	case gateway.KindStream:
		e.s.Stream, e.s.Target = conn, e.srv.Addr
		client := telnetconnector.NewClient(user, "vt100")
		require.NoError(t, client.SetWindowSize(132, 40))
		e.in = client
		e.resz = func(cols, rows int) { require.NoError(t, client.SetWindowSize(cols, rows)) }
		go func() { _, _ = io.Copy(e.out, client) }()
	}
	ctx, cancel := context.WithCancel(t.Context())
	go func() { e.err <- telnetconnector.Connector{}.Serve(ctx, e.s) }()
	t.Cleanup(func() {
		cancel()
		select {
		case <-e.err:
		case <-time.After(waitFor):
			t.Error("Serve did not return after the cancel")
		}
	})
	return e
}

func (e *env) waitOutput(t *testing.T, want string) {
	t.Helper()
	require.Eventually(t, func() bool { return strings.Contains(e.out.String(), want) }, waitFor, 5*time.Millisecond,
		"the user output lacks %q: %q", want, e.out.String())
}

func (e *env) waitEvent(t *testing.T, want string) {
	t.Helper()
	timeout := time.After(waitFor)
	for {
		select {
		case ev := <-e.srv.Events:
			if ev == want {
				return
			}
		case <-timeout:
			t.Fatalf("no target event %q", want)
		}
	}
}

func (e *env) serveErr(t *testing.T) error {
	t.Helper()
	select {
	case err := <-e.err:
		e.err <- err // for the cleanup
		return err
	case <-time.After(waitFor):
		t.Fatal("Serve did not return")
		return nil
	}
}

func kinds(t *testing.T, f func(t *testing.T, kind gateway.Kind)) {
	for _, k := range []gateway.Kind{gateway.KindTerminal, gateway.KindStream} {
		t.Run(string(k), func(t *testing.T) { f(t, k) })
	}
}

// The password reaches the target only: not the user, the recording, the log or an error.
// The target echoes the password line, so the output strip is exercised.
func TestTelnetPasswordNotRecorded(t *testing.T) {
	logs := &lockedBuffer{}
	old := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(logs, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(old) })
	kinds(t, func(t *testing.T, kind gateway.Kind) {
		e := start(t, kind, telnettest.Login, "<target>")
		pw := e.srv.Password
		e.waitOutput(t, "Welcome\r\n$ ")
		_, err := io.WriteString(e.in, "uptime\r")
		require.NoError(t, err)
		require.Eventually(t, func() bool { return strings.Contains(e.srv.Shell(), "uptime") }, waitFor, 5*time.Millisecond)
		e.waitOutput(t, "$ uptime")

		require.NotContains(t, e.out.String(), pw)
		require.NotContains(t, e.rec.output(), pw)
		require.NotContains(t, e.rec.input(), pw)
		require.NotContains(t, e.rec.input(), telnettest.Login, "the autologin input was recorded")
		require.Contains(t, e.rec.input(), "uptime")
		require.Contains(t, e.rec.output(), "Welcome")
		require.NotContains(t, logs.String(), pw)
	})
}

// 0xFF in the data passes both ways as one byte: escaped on the wire, not doubled.
func TestTelnetIACEscaped(t *testing.T) {
	kinds(t, func(t *testing.T, kind gateway.Kind) {
		e := start(t, kind, telnettest.Login, "<target>")
		e.waitOutput(t, "$ ")
		_, err := e.in.Write([]byte("a\xffb"))
		require.NoError(t, err)
		require.Eventually(t, func() bool { return strings.Contains(e.srv.Shell(), "a\xffb") }, waitFor, 5*time.Millisecond,
			"the target read %q", e.srv.Shell())
		e.waitOutput(t, "$ a\xffb")
		require.NotContains(t, e.srv.Shell(), "\xff\xff")
		require.NotContains(t, e.out.String(), "\xff\xff")
		require.Contains(t, e.rec.input(), "a\xffb")
		require.Contains(t, e.rec.output(), "a\xffb")
	})
}

// The user window and TERM reach the target before its login prompt; later
// window changes go to the target and the recording.
func TestTelnetStreamNAWSBeforeLogin(t *testing.T) {
	kinds(t, func(t *testing.T, kind gateway.Kind) {
		e := start(t, kind, telnettest.Login, "<target>")
		e.waitEvent(t, "prompt 132x40 vt100")
		e.waitOutput(t, "$ ")
		if kind == gateway.KindStream {
			require.Equal(t, [][2]int{{132, 40}}, e.rec.sizes(), "the recording window is the client window")
		}
		e.resz(100, 30)
		e.waitEvent(t, "win 100x30")
		require.Eventually(t, func() bool { s := e.rec.sizes(); return len(s) > 0 && s[len(s)-1] == [2]int{100, 30} },
			waitFor, 5*time.Millisecond)
	})
}

// A wrong password ends the session with ErrLoginFailed; a stream user reads the text.
func TestTelnetLoginFailed(t *testing.T) {
	kinds(t, func(t *testing.T, kind gateway.Kind) {
		e := start(t, kind, telnettest.Login, "wrong-password-marker")
		err := e.serveErr(t)
		require.ErrorIs(t, err, gateway.ErrLoginFailed)
		require.ErrorIs(t, err, telnetconnector.ErrLoginFailed)
		require.NotContains(t, err.Error(), "wrong-password-marker")
		if kind == gateway.KindStream {
			e.waitOutput(t, "Login to router failed\r\n")
		}
		require.NotContains(t, e.out.String(), "wrong-password-marker")
		require.NotContains(t, e.rec.output(), "wrong-password-marker")
	})
}

// Without a password (or a login) the user logs in at the target's own prompt.
func TestTelnetNoCredentials(t *testing.T) {
	for name, c := range map[string]struct{ login, secret string }{
		"no password": {telnettest.Login, ""},
		"no login":    {"", "<target>"},
	} {
		t.Run(name, func(t *testing.T) {
			e := start(t, gateway.KindTerminal, c.login, c.secret)
			e.waitOutput(t, "login: ")
			_, err := io.WriteString(e.in, telnettest.Login+"\r")
			require.NoError(t, err)
			e.waitOutput(t, "Password: ")
			_, err = io.WriteString(e.in, e.srv.Password+"\r")
			require.NoError(t, err)
			e.waitOutput(t, "Welcome\r\n$ ")
			require.Contains(t, e.rec.input(), telnettest.Login, "the user's own typing is recorded")
		})
	}
}

// A command cannot run over telnet: Serve fails before it dials.
func TestTelnetCommandRefused(t *testing.T) {
	srv := telnettest.Start(t, "pw")
	var dials atomic.Int32
	user, conn := net.Pipe()
	defer user.Close()
	s := &gateway.Session{ID: uuid.New(), Kind: gateway.KindTerminal, Resource: srv.Resource(t, telnettest.Login),
		Recording: &spyRec{}, Terminal: &gateway.Terminal{ReadWriter: conn, Term: "xterm", Command: "ls"},
		Dial: func(context.Context, string, string) (net.Conn, error) {
			dials.Add(1)
			return nil, io.EOF
		},
	}
	require.Error(t, telnetconnector.Connector{}.Serve(t.Context(), s))
	require.Zero(t, dials.Load())
	require.Zero(t, srv.Accepts())
}
