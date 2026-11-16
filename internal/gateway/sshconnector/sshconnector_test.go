package sshconnector

import (
	"bytes"
	"context"
	"crypto/rand"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protojson"

	sshv1 "github.com/werbot/werbot/gen/werbot/connector/ssh/v1"
	"github.com/werbot/werbot/internal/connector"
	_ "github.com/werbot/werbot/internal/connector/ssh" // the server type "ssh"
	"github.com/werbot/werbot/internal/gateway"
	"github.com/werbot/werbot/internal/gateway/connectortest"
	"github.com/werbot/werbot/internal/gateway/sshtest"
)

const waitFor = 5 * time.Second

func TestContract(t *testing.T) {
	secret := "werbot-secret-" + rand.Text()
	tg := sshtest.StartTarget(t, secret, nil)
	typ, ok := connector.Lookup("ssh")
	require.True(t, ok)
	require.ElementsMatch(t, typ.Kinds, Connector{}.Kinds())
	registered, ok := gateway.LookupConnector("ssh")
	require.True(t, ok)
	require.Equal(t, Connector{DialTimeout: 10 * time.Second}, registered)

	connectortest.Run(t, Connector{DialTimeout: 5 * time.Second}, connectortest.Target{
		Resource: resource(t, tg),
		Secret:   gateway.Secret{Kind: "password", Value: []byte(secret)},
		Stream:   tg.EchoAddr,
	})
}

func TestStreamTarget(t *testing.T) {
	tg := sshtest.StartTarget(t, "pw", nil)
	s := newSession(t, tg, gateway.KindStream)
	user, conn := net.Pipe()
	s.Stream, s.Target = conn, tg.EchoAddr
	served := serve(t.Context(), s)
	require.NoError(t, user.SetDeadline(time.Now().Add(waitFor)))
	_, err := io.WriteString(user, "ping")
	require.NoError(t, err)
	got := make([]byte, 4)
	_, err = io.ReadFull(user, got)
	require.NoError(t, err)
	require.Equal(t, "ping", string(got))
	require.NoError(t, user.Close())
	require.NoError(t, wait(t, served))

	s = newSession(t, tg, gateway.KindStream)
	user, conn = net.Pipe()
	s.Stream, s.Target = conn, "127.0.0.1:1"
	require.Error(t, wait(t, serve(t.Context(), s)))
	_, err = user.Read(make([]byte, 1))
	require.ErrorIs(t, err, io.EOF, "the stream is not closed")
}

func TestHostKeyTOFU(t *testing.T) {
	tg := sshtest.StartTarget(t, "pw", nil)
	var alg string
	var key []byte
	s := newSession(t, tg, gateway.KindExec)
	s.PinKey = func(a string, k []byte) error { alg, key = a, k; return nil }
	stdout, code := attachExec(s, "echo hi", strings.NewReader(""))
	require.NoError(t, wait(t, serve(t.Context(), s)))
	require.Equal(t, "hi\n", stdout.String())
	require.Equal(t, 0, *code)
	require.Equal(t, "ssh-ed25519", alg)
	require.Equal(t, tg.HostKey.Marshal(), key)

	s = newSession(t, tg, gateway.KindExec)
	s.PinKey = func(string, []byte) error { return fmt.Errorf("%w: pinned key differs", gateway.ErrHostKeyMismatch) }
	attachExec(s, "echo hi", strings.NewReader(""))
	require.ErrorIs(t, wait(t, serve(t.Context(), s)), gateway.ErrHostKeyMismatch)
}

func TestCancelClosesTarget(t *testing.T) {
	tg := sshtest.StartTarget(t, "pw", nil)
	s := newSession(t, tg, gateway.KindTerminal)
	attachTerminal(t, s, "xterm", "", nil)
	ctx, cancel := context.WithCancel(t.Context())
	served := serve(ctx, s)
	waitEvent(t, tg, "pty:xterm 120x40")
	cancel()
	select {
	case err := <-served:
		require.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("Serve did not return within 2s of the cancel")
	}
	waitEvent(t, tg, "closed")
}

// A panic in a relay goroutine ends only its session, with an error from Serve.
func TestRelayPanicEndsSession(t *testing.T) {
	tg := sshtest.StartTarget(t, "pw", nil)
	s := newSession(t, tg, gateway.KindTerminal)
	s.Terminal = &gateway.Terminal{ReadWriter: panicRW{}, Term: "xterm", Window: gateway.Window{Cols: 80, Rows: 24},
		Resize: make(chan gateway.Window), SetExitStatus: func(int) {}}
	err := wait(t, serve(t.Context(), s))
	require.ErrorContains(t, err, "panic")
	waitEvent(t, tg, "closed")
}

type panicRW struct{}

func (panicRW) Read([]byte) (int, error)    { panic("fault injected") }
func (panicRW) Write(p []byte) (int, error) { return len(p), nil }

// resource is the ssh resource of the target.
func resource(t *testing.T, tg *sshtest.Target) gateway.Resource {
	t.Helper()
	host, port, err := net.SplitHostPort(tg.Addr)
	require.NoError(t, err)
	p, err := strconv.ParseUint(port, 10, 16)
	require.NoError(t, err)
	cfg, err := protojson.Marshal(&sshv1.Config{Host: host, Port: uint32(p), Login: "werbot"})
	require.NoError(t, err)
	return gateway.Resource{ID: uuid.New(), Name: "target", Type: "ssh", Config: cfg}
}

func newSession(t *testing.T, tg *sshtest.Target, kind gateway.Kind) *gateway.Session {
	t.Helper()
	return &gateway.Session{
		ID: uuid.New(), Kind: kind, Front: "test", User: gateway.User{ID: uuid.New(), Login: "alice"},
		Resource: resource(t, tg), Secret: gateway.Secret{Kind: "password", Value: []byte(tg.Password)},
		PinKey: acceptKey, Recording: &recSink{},
	}
}

func serve(ctx context.Context, s *gateway.Session) <-chan error {
	served := make(chan error, 1)
	go func() { served <- Connector{DialTimeout: waitFor}.Serve(ctx, s) }()
	return served
}

func wait(t *testing.T, served <-chan error) error {
	t.Helper()
	select {
	case err := <-served:
		return err
	case <-time.After(waitFor):
		t.Fatal("Serve did not return")
		return nil
	}
}

// waitEvent reads target events up to want and returns the ones before it.
func waitEvent(t *testing.T, tg *sshtest.Target, want string) []string {
	t.Helper()
	var seen []string
	timeout := time.After(waitFor)
	for {
		select {
		case e := <-tg.Events:
			if e == want {
				return seen
			}
			seen = append(seen, e)
		case <-timeout:
			t.Fatalf("no event %q, seen %q", want, seen)
		}
	}
}

// recSink is a RecordingSink that keeps everything.
type recSink struct {
	mu      sync.Mutex
	in, out bytes.Buffer
	sizes   []gateway.Window
}

func (r *recSink) Output(p []byte) { r.mu.Lock(); r.out.Write(p); r.mu.Unlock() }
func (r *recSink) Input(p []byte)  { r.mu.Lock(); r.in.Write(p); r.mu.Unlock() }
func (r *recSink) Resize(cols, rows int) {
	r.mu.Lock()
	r.sizes = append(r.sizes, gateway.Window{Cols: cols, Rows: rows})
	r.mu.Unlock()
}
func (*recSink) Close(context.Context) error { return nil }

func (r *recSink) get() (in, out string, sizes []gateway.Window) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.in.String(), r.out.String(), append([]gateway.Window(nil), r.sizes...)
}

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
