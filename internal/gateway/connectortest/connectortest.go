// Package connectortest checks a gateway connector against the connector
// contract (00-contracts §3.3). It is for _test.go files only.
package connectortest

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net"
	"net/netip"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/connector"
	"github.com/werbot/werbot/internal/gateway"
)

const (
	probe   = "werbot-probe"
	waitFor = 5 * time.Second // a probe round trip, including the connector dialing its target
)

// Target is what the connector under test connects to.
type Target struct {
	Resource gateway.Resource // Type == c.Type()
	Secret   gateway.Secret   // a unique marker: it must not appear in logs or errors
	Stream   string           // host:port of an echo service, for KindStream
	Datagram string           // host:port of a UDP echo service, for KindDatagram
}

// Run checks c: nothing starts before the first session, Kinds match the server
// type, every declared kind works with target, a cancel ends Serve within 1 s,
// the recorder sees the traffic of the recorded kinds only, a TargetDialer dials
// through Session.Dial, the endpoint allows the targets, the secret never leaks and no goroutine is left.
// It replaces slog.Default, so the calling test must not be parallel.
func Run(t *testing.T, c gateway.Connector, target Target) {
	t.Helper()
	base := runtime.NumGoroutine()
	_ = c.Type()
	kinds := c.Kinds()
	time.Sleep(100 * time.Millisecond)
	require.LessOrEqual(t, runtime.NumGoroutine(), base, "the connector starts goroutines before its first session")
	if typ, ok := connector.Lookup(c.Type()); ok {
		require.ElementsMatch(t, typ.Kinds, kinds, "Kinds differ from the server type %q", c.Type())
	}

	logs := &lockedBuffer{}
	old := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(logs, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(old) })

	for _, k := range kinds {
		t.Run(string(k), func(t *testing.T) { runKind(t, c, target, k) })
	}
	require.NotContains(t, logs.String(), string(target.Secret.Value), "the secret reached slog.Default")
	// polled inline: require.Eventually runs its condition on another goroutine
	for deadline := time.Now().Add(2 * time.Second); runtime.NumGoroutine() > base && time.Now().Before(deadline); {
		time.Sleep(10 * time.Millisecond)
	}
	require.LessOrEqual(t, runtime.NumGoroutine(), base, "goroutines are left after the sessions")
}

// runKind serves one session of kind k, drives the probe, then cancels it.
func runKind(t *testing.T, c gateway.Connector, target Target, k gateway.Kind) {
	rec := &spySink{}
	var dials atomic.Int32
	dialer := &net.Dialer{Timeout: waitFor}
	s := &gateway.Session{
		ID: uuid.New(), Kind: k, Front: "test", User: gateway.User{ID: uuid.New(), Login: "probe"},
		Resource: target.Resource, Secret: target.Secret, ClientIP: netip.MustParseAddr("192.0.2.1"),
		PinKey: func(string, []byte) error { return nil }, Recording: rec,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			dials.Add(1)
			return dialer.DialContext(ctx, network, address)
		},
	}
	var drive func(t *testing.T)
	switch k {
	case gateway.KindTerminal:
		user, conn := net.Pipe()
		defer user.Close()
		defer conn.Close()
		out := &lockedBuffer{}
		go func() { _, _ = io.Copy(out, user) }() // net.Pipe is synchronous: keep reading
		s.Terminal = &gateway.Terminal{ReadWriter: conn, Term: "xterm", Window: gateway.Window{Cols: 80, Rows: 24},
			Resize: make(chan gateway.Window), SetExitStatus: func(int) {}}
		drive = func(t *testing.T) {
			require.NoError(t, user.SetWriteDeadline(time.Now().Add(waitFor)))
			_, err := io.WriteString(user, probe+"\r")
			require.NoError(t, err)
			require.Eventually(t, func() bool { return strings.Contains(out.String(), probe) }, waitFor, 10*time.Millisecond,
				"terminal output lacks the probe: %q", out.String())
		}
	case gateway.KindExec:
		stdout, stderr, code := &lockedBuffer{}, &lockedBuffer{}, atomic.Int32{}
		code.Store(-1)
		s.Exec = &gateway.Exec{Command: "echo " + probe, Stdin: strings.NewReader(""), Stdout: stdout, Stderr: stderr,
			SetExitStatus: func(c int) { code.Store(int32(c)) }} //nolint:gosec // exit codes fit int32
		drive = func(t *testing.T) {
			require.Eventually(t, func() bool { return stdout.String() == probe+"\n" && code.Load() == 0 }, waitFor, 10*time.Millisecond,
				"exec: stdout %q, stderr %q, exit status %d", stdout.String(), stderr.String(), code.Load())
		}
	case gateway.KindStream:
		user, conn := net.Pipe()
		defer user.Close()
		s.Stream, s.Target = conn, target.Stream
		requireEndpointAllows(t, c.Type(), target.Resource, target.Stream)
		drive = func(t *testing.T) {
			require.NoError(t, user.SetDeadline(time.Now().Add(waitFor)))
			_, err := io.WriteString(user, probe)
			require.NoError(t, err)
			got := make([]byte, len(probe))
			_, err = io.ReadFull(user, got)
			require.NoError(t, err)
			require.Equal(t, probe, string(got))
		}
	case gateway.KindDatagram:
		client, peer := Datagrams()
		s.Datagram, s.Target = client, target.Datagram
		requireEndpointAllows(t, c.Type(), target.Resource, target.Datagram)
		drive = func(t *testing.T) { driveDatagrams(t, peer) }
	default:
		t.Fatalf("unknown kind %q", k)
	}

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	served := make(chan error, 1)
	go func() { served <- c.Serve(ctx, s) }()
	drive(t)
	cancel()
	select {
	case err := <-served:
		if err != nil {
			require.NotContains(t, err.Error(), string(target.Secret.Value), "the secret reached the Serve error")
		}
	case <-time.After(time.Second):
		t.Fatal("Serve did not return within 1s of the cancel")
	}
	requireRecorded(t, c.Type(), k, rec)
	if k == gateway.KindStream || k == gateway.KindDatagram {
		requireDialsThroughSession(t, c, int(dials.Load()))
	}
}

// requireRecorded checks that the recorder saw traffic exactly for the kinds the
// registered type records. A type that is not registered records nothing.
func requireRecorded(t require.TestingT, typ string, k gateway.Kind, rec *spySink) {
	if registered, ok := connector.Lookup(typ); ok && registered.Records(k) {
		require.True(t, rec.output.Load(), "Recording.Output was not called")
		if k != gateway.KindExec { // exec sends no input
			require.True(t, rec.input.Load(), "Recording.Input was not called")
		}
		return
	}
	require.False(t, rec.output.Load() || rec.input.Load(), "the connector records kind %q, which its type does not record", k)
}

// requireDialsThroughSession checks that a connector dials the target through
// Session.Dial exactly when it is marked a TargetDialer (ssh keeps its own dial).
func requireDialsThroughSession(t require.TestingT, c gateway.Connector, dials int) {
	if _, marked := c.(gateway.TargetDialer); marked {
		require.Positive(t, dials, "a TargetDialer must dial through Session.Dial")
		return
	}
	require.Zero(t, dials, "the connector dials through Session.Dial but lacks DialsTarget")
}

// requireEndpointAllows checks that the registered type's endpoint, if it has one,
// allows addr under the resource name.
func requireEndpointAllows(t *testing.T, typ string, res gateway.Resource, addr string) {
	t.Helper()
	registered, ok := connector.Lookup(typ)
	if !ok {
		return
	}
	ep, has, err := registered.EndpointOf(res.Config)
	require.NoError(t, err)
	if !has {
		return
	}
	host, portText, err := net.SplitHostPort(addr)
	require.NoError(t, err)
	port, err := strconv.ParseUint(portText, 10, 16)
	require.NoError(t, err)
	require.True(t, ep.Allow(res.Name, host, uint16(port)), "the endpoint of %q does not allow %s", typ, addr)
}

// spySink notes which recorder methods were called.
type spySink struct{ output, input atomic.Bool }

func (s *spySink) Output([]byte)             { s.output.Store(true) }
func (s *spySink) Input([]byte)              { s.input.Store(true) }
func (*spySink) Resize(int, int)             {}
func (*spySink) Close(context.Context) error { return nil }

// lockedBuffer is a buffer safe for the connector goroutines and the test.
type lockedBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
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
