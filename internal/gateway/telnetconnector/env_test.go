package telnetconnector_test

import (
	"context"
	"crypto/rand"
	"io"
	"net"
	"net/netip"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/gateway"
	"github.com/werbot/werbot/internal/gateway/telnetconnector"
	"github.com/werbot/werbot/internal/gateway/telnettest"
)

const waitFor = 5 * time.Second

// env is one session against a telnet target, driven as a user.
type env struct {
	srv      *telnettest.Server // nil for a scripted target
	s        *gateway.Session
	rec      *spyRec
	out      *lockedBuffer // what the user sees
	in       io.Writer     // what the user types
	resz     func(cols, rows int)
	dials    atomic.Int32
	err      chan error // Serve's result; nil after a panic
	panicked chan any   // a panic of Serve
	cancel   context.CancelFunc
}

// opts selects the target and the user side of a session.
type opts struct {
	login, secret string // secret "<target>" is the telnettest password
	addr          string // a scripted target instead of telnettest
	raw           bool   // stream: the user side is a raw TCP conn, not a telnet client
}

// start serves a session of kind against telnettest with the resource login
// and secret; the user window is 132×40 with TERM vt100. A stream user is a
// telnet client.
func start(t *testing.T, kind gateway.Kind, login, secret string) *env {
	t.Helper()
	return startWith(t, kind, opts{login: login, secret: secret})
}

func startWith(t *testing.T, kind gateway.Kind, o opts) *env {
	t.Helper()
	e := &env{rec: &spyRec{}, out: &lockedBuffer{}, err: make(chan error, 1), panicked: make(chan any, 1)}
	addr := o.addr
	if addr == "" {
		e.srv = telnettest.Start(t, "werbot-telnet-secret-"+rand.Text())
		addr = e.srv.Addr
		if o.secret == "<target>" {
			o.secret = e.srv.Password
		}
	}
	dialer := &net.Dialer{Timeout: waitFor}
	e.s = &gateway.Session{
		ID: uuid.New(), Kind: kind, Front: "test", User: gateway.User{ID: uuid.New(), Login: "alice"},
		Resource: telnettest.Resource(t, addr, o.login), ClientIP: netip.MustParseAddr("192.0.2.1"), Recording: e.rec,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			e.dials.Add(1)
			return dialer.DialContext(ctx, network, address)
		},
	}
	if o.secret != "" {
		e.s.Secret = gateway.Secret{Kind: "password", Value: []byte(o.secret)}
	}
	user, conn := net.Pipe()
	if kind == gateway.KindStream {
		user, conn = tcpPair(t) // buffered like an SSH channel: both telnet sides write while the other writes
	}
	t.Cleanup(func() { _ = user.Close() })
	switch {
	case kind == gateway.KindTerminal:
		resize := make(chan gateway.Window, 1)
		e.s.Terminal = &gateway.Terminal{ReadWriter: conn, Term: "vt100", Window: gateway.Window{Cols: 132, Rows: 40},
			Resize: resize, SetExitStatus: func(int) {}}
		e.in = user
		e.resz = func(cols, rows int) { resize <- gateway.Window{Cols: cols, Rows: rows} }
		go func() { _, _ = io.Copy(e.out, user) }()
	case o.raw:
		e.s.Stream, e.s.Target = conn, addr
		e.in = user
		go func() { _, _ = io.Copy(e.out, user) }()
	default:
		e.s.Stream, e.s.Target = conn, addr
		client := telnetconnector.NewClient(user, "vt100")
		require.NoError(t, client.SetWindowSize(132, 40))
		e.in = client
		e.resz = func(cols, rows int) { require.NoError(t, client.SetWindowSize(cols, rows)) }
		go func() { _, _ = io.Copy(e.out, client) }()
	}
	ctx, cancel := context.WithCancel(t.Context())
	e.cancel = cancel
	go func() {
		defer func() {
			if r := recover(); r != nil {
				e.panicked <- r
				e.err <- nil
			}
		}()
		e.err <- telnetconnector.Connector{}.Serve(ctx, e.s)
	}()
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

// serveErr waits up to within for Serve to return.
func (e *env) serveErr(t *testing.T, within time.Duration) error {
	t.Helper()
	select {
	case err := <-e.err:
		e.err <- err // for the cleanup
		return err
	case <-time.After(within):
		t.Fatalf("Serve did not return within %v", within)
		return nil
	}
}

func kinds(t *testing.T, f func(t *testing.T, kind gateway.Kind)) {
	for _, k := range []gateway.Kind{gateway.KindTerminal, gateway.KindStream} {
		t.Run(string(k), func(t *testing.T) { f(t, k) })
	}
}
