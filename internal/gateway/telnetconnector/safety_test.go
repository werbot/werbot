package telnetconnector_test

import (
	"fmt"
	"io"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/gateway"
	"github.com/werbot/werbot/internal/gateway/telnetconnector"
	"github.com/werbot/werbot/internal/gateway/telnettest"
)

// A panic on a relay goroutine reaches Serve's caller (the pipeline ends the
// session as internal_error) instead of crashing the process.
func TestTelnetRelayPanicReachesServe(t *testing.T) {
	t.Run("input copy", func(t *testing.T) {
		e := start(t, gateway.KindTerminal, telnettest.Login, "<target>")
		e.waitOutput(t, "$ ")
		e.rec.panicInput.Store(true)
		_, err := io.WriteString(e.in, "x")
		require.NoError(t, err)
		select {
		case p := <-e.panicked:
			require.Contains(t, fmt.Sprint(p), "fault injected")
		case <-time.After(waitFor):
			t.Fatal("the relay panic did not reach Serve")
		}
	})
	t.Run("stream client parse", func(t *testing.T) {
		s := &gateway.Session{ID: uuid.New(), Kind: gateway.KindStream, Resource: telnettest.Resource(t, "127.0.0.1:1", ""),
			Recording: &spyRec{}, Stream: panicStream{}}
		p := recovered(func() { _ = telnetconnector.Connector{}.Serve(t.Context(), s) })
		require.Contains(t, fmt.Sprint(p), "fault injected")
	})
}

func recovered(fn func()) (p any) {
	defer func() { p = recover() }()
	fn()
	return nil
}

// panicStream is a client stream whose read panics, as a parser fault would.
type panicStream struct{}

func (panicStream) Read([]byte) (int, error)    { panic("fault injected") }
func (panicStream) Write(p []byte) (int, error) { return len(p), nil }
func (panicStream) Close() error                { return nil }

// loginScript answers the gateway's login lines, then runs after.
func loginScript(after func(c net.Conn)) func(c net.Conn) {
	return func(c net.Conn) {
		_, _ = io.WriteString(c, "login: ")
		if err := skipLine(c); err != nil {
			return
		}
		_, _ = io.WriteString(c, "Password: ")
		if err := skipLine(c); err != nil {
			return
		}
		after(c)
	}
}

// A password echoed in pieces, or after the shell prompt (after Login
// returned), reaches neither the user nor the recording.
func TestTelnetPasswordEchoSplitOrLate(t *testing.T) {
	const pw = "ab#cd" // "ab#" alone looks like a shell prompt
	for name, after := range map[string]func(c net.Conn){
		"split": func(c net.Conn) {
			_, _ = io.WriteString(c, "ab#")
			time.Sleep(50 * time.Millisecond) // two reads at the gateway
			_, _ = io.WriteString(c, "cd\r\nWelcome\r\n$ ")
			_, _ = io.Copy(c, c)
		},
		"late": func(c net.Conn) {
			_, _ = io.WriteString(c, "Welcome\r\n$ ")
			b := make([]byte, 1) // the user typed: Login has returned
			if _, err := c.Read(b); err != nil {
				return
			}
			_, _ = io.WriteString(c, "ab")
			time.Sleep(50 * time.Millisecond)
			_, _ = io.WriteString(c, "#cd"+string(b))
			_, _ = io.Copy(c, c)
		},
	} {
		t.Run(name, func(t *testing.T) {
			addr := scripted(t, loginScript(after))
			e := startWith(t, gateway.KindTerminal, opts{login: telnettest.Login, secret: pw, addr: addr})
			e.waitOutput(t, "Welcome\r\n$ ")
			_, err := io.WriteString(e.in, "x")
			require.NoError(t, err)
			e.waitOutput(t, "$ x")
			for _, part := range []string{pw, "ab#", "#cd"} {
				require.NotContains(t, e.out.String(), part)
				require.NotContains(t, e.rec.output(), part)
			}
		})
	}
}

// The initial client window reaches the target and the recording once, also
// when the client answers TTYPE before NAWS.
func TestTelnetStreamTTYPEFirst(t *testing.T) {
	e := startWith(t, gateway.KindStream, opts{login: telnettest.Login, secret: "<target>", raw: true})
	_, err := e.in.Write([]byte{
		telnetconnector.IAC, telnetconnector.WILL, 24, telnetconnector.IAC, telnetconnector.SB, 24, 0, 'v', 't', '1', '0', '0', telnetconnector.IAC, telnetconnector.SE,
		telnetconnector.IAC, telnetconnector.WILL, 31, telnetconnector.IAC, telnetconnector.SB, 31, 0, 132, 0, 40, telnetconnector.IAC, telnetconnector.SE,
	})
	require.NoError(t, err)
	e.waitEvent(t, "prompt 132x40 vt100")
	e.waitOutput(t, "$ ")
	_, err = io.WriteString(e.in, "x") // a round trip after the relay started
	require.NoError(t, err)
	e.waitOutput(t, "$ x")
	require.Equal(t, [][2]int{{132, 40}}, e.rec.sizes())
	for len(e.srv.Events) > 0 {
		require.NotContains(t, <-e.srv.Events, "win ", "the initial window was sent twice")
	}
}

// A cancel during the automatic login ends Serve at once, without an error.
func TestTelnetCancelDuringLogin(t *testing.T) {
	asked := make(chan struct{})
	addr := scripted(t, func(c net.Conn) {
		_, _ = io.WriteString(c, "login: ")
		if err := skipLine(c); err != nil {
			return
		}
		close(asked) // and never ask for the password
		_, _ = io.Copy(io.Discard, c)
	})
	e := startWith(t, gateway.KindTerminal, opts{login: telnettest.Login, secret: "pw", addr: addr})
	<-asked
	e.cancel()
	require.NoError(t, e.serveErr(t, time.Second))
}

// A cancel while the stream waits for the client's negotiation ends Serve
// without an error and before any dial.
func TestTelnetCancelDuringNegotiation(t *testing.T) {
	e := startWith(t, gateway.KindStream, opts{login: telnettest.Login, secret: "<target>", raw: true})
	require.Eventually(t, func() bool { return strings.Contains(e.out.String(), "\xff\xfd\x18") }, waitFor, time.Millisecond,
		"no handshake") // DO TTYPE: the last handshake command
	e.cancel()
	require.NoError(t, e.serveErr(t, time.Second))
	require.Zero(t, e.dials.Load())
}
