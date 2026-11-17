package telnetconnector_test

import (
	"context"
	"io"
	"log/slog"
	"net"
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

func TestContract(t *testing.T) {
	registered, ok := gateway.LookupConnector("telnet")
	require.True(t, ok)
	require.Equal(t, telnetconnector.Connector{}, registered)
	connectortest.Run(t, telnetconnector.Connector{}, telnettest.Target(t))
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
		err := e.serveErr(t, waitFor)
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
