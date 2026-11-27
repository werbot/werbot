package gateway

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/werbot/werbot/internal/connector"
)

func init() {
	// preconn reads a pre-session before StartSession, by resource name (see
	// PreSession), echoes its stream and probes the resource name as a host
	connector.Register(connector.Type{Name: "preconn", NewConfig: func() proto.Message { return &emptypb.Empty{} },
		Validate: func(proto.Message) error { return nil }, Actions: []string{"preconn.connect"},
		Kinds: []connector.Kind{connector.KindStream},
		Endpoint: func(proto.Message) connector.Endpoint {
			return connector.Endpoint{Host: "db.test", Ports: []connector.PortRange{{Port: 5432}}, AnyTarget: true}
		}})
	RegisterConnector(preConn{funcConn{typ: "preconn", kinds: []Kind{KindStream}, serve: echo}})
}

// preCalls counts PreSession calls of preconn.
var preCalls atomic.Int64

type preConn struct{ funcConn }

// PreSession of "prehandled" handles the stream, "predenied" refuses it,
// "prepanic" panics; any other resource needs 8 bytes and starts database "app".
func (preConn) PreSession(_ context.Context, r Resource, peek func(int) ([]byte, error), _ io.ReadWriteCloser) (PreSessionResult, error) {
	preCalls.Add(1)
	switch r.Name {
	case "prehandled":
		return PreSessionResult{Handled: true}, nil
	case "predenied":
		return PreSessionResult{}, fmt.Errorf("%w: test refusal", ErrDenied)
	case "prepanic":
		panic("pre boom")
	}
	if _, err := peek(8); err != nil {
		return PreSessionResult{}, err
	}
	return PreSessionResult{DBName: "app"}, nil
}

// Probe dials the resource name on port 5432.
func (preConn) Probe(ctx context.Context, r Resource, dial func(ctx context.Context, network, address string) (net.Conn, error)) error {
	c, err := dial(ctx, "tcp", net.JoinHostPort(r.Name, "5432"))
	if err != nil {
		return err
	}
	return c.Close()
}

// newPreEnv is a pipeline env whose snapshot also has the preconn resources
// "pre", "prehandled", "predenied", "prepanic" and "denied" (forbidden).
func newPreEnv(t *testing.T) *pipeEnv {
	t.Helper()
	e := newPipeEnv(t, 10)
	rs := []res{{"rfake", "fake", "fake"}, {"rdenied", "denied", "preconn"}}
	for _, n := range []string{"pre", "prehandled", "predenied", "prepanic"} {
		e.m["r"+n] = uuid.New()
		rs = append(rs, res{"r" + n, n, "preconn"})
	}
	e.f.setPolicy(compileTest(t, e.m, pipePolicies, nil, rs...))
	return e
}

// preReq is a stream request of resource name whose Accept hands out st.
func preReq(e *pipeEnv, name string, st io.ReadWriteCloser) OpenRequest {
	r := e.req(name, KindStream)
	r.Target = "db:5432"
	r.Accept = func() (io.ReadWriteCloser, error) { return st, nil }
	return r
}

// requireClosed checks that the client side of a stream sees its end.
func requireClosed(t *testing.T, c net.Conn) {
	t.Helper()
	_, err := c.Read(make([]byte, 1))
	require.ErrorIs(t, err, io.EOF, "the accepted stream is closed")
}

// requireNoReject checks that no sessions_rejected_total counter moved.
func requireNoReject(t *testing.T, e *pipeEnv) {
	t.Helper()
	require.NotRegexp(t, `werbot_gateway_sessions_rejected_total\{[^}]*\} [1-9]`, e.metrics(t))
}

// send writes p to c in the background: a net.Pipe write waits for the reader.
func send(c net.Conn, p string) { go func() { _, _ = io.WriteString(c, p) }() }

func TestPipelinePreSessionHandled(t *testing.T) {
	e := newPreEnv(t)
	srv, cli := net.Pipe()
	defer cli.Close() //nolint:errcheck // test pipe
	_, err := e.p.Open(t.Context(), preReq(e, "prehandled", srv))
	require.ErrorIs(t, err, ErrHandled)
	require.NotErrorIs(t, err, ErrDenied)
	require.Zero(t, e.startCalls())
	require.Empty(t, e.p.heartbeat().GetDenials(), "a handled stream is no denial")
	require.Zero(t, e.p.active.used(), "the slot is freed")
	requireNoReject(t, e)
	requireClosed(t, cli)
}

func TestPipelinePreSessionDenied(t *testing.T) {
	e := newPreEnv(t)
	srv, cli := net.Pipe()
	defer cli.Close() //nolint:errcheck // test pipe
	_, err := e.p.Open(t.Context(), preReq(e, "predenied", srv))
	require.ErrorIs(t, err, ErrDenied)
	require.Zero(t, e.startCalls(), "refused before StartSession: no session row")
	d := e.p.heartbeat().GetDenials()
	require.Len(t, d, 1)
	require.Equal(t, "denied", d[0].GetReason())
	require.Equal(t, "db:5432", d[0].GetTarget())
	require.Zero(t, e.p.active.used())
	requireClosed(t, cli)
}

func TestPipelinePreSessionWithoutAccept(t *testing.T) {
	e := newPreEnv(t)
	r := e.req("pre", KindStream)
	r.Target = "db:5432"
	_, err := e.p.Open(t.Context(), r)
	require.ErrorIs(t, err, ErrDenied, "a PreSession connector is never served without its checks")
	require.Zero(t, e.startCalls())
	require.Zero(t, e.p.active.used())
}

func TestPipelinePreSessionDBName(t *testing.T) {
	e := newPreEnv(t)
	srv, cli := net.Pipe()
	defer cli.Close() //nolint:errcheck // test pipe
	send(cli, "12345678")
	s, err := e.p.Open(t.Context(), preReq(e, "pre", srv))
	require.NoError(t, err)
	defer e.p.abort(s)
	require.Equal(t, "app", sessionAgent(s.ID).DBName)
	require.Equal(t, 1, e.startCalls())
}

// cwConn is a stream with CloseWrite, like an SSH channel.
type cwConn struct {
	net.Conn
	closedWrite atomic.Bool
}

func (c *cwConn) CloseWrite() error { c.closedWrite.Store(true); return nil }

func TestPipelinePreSessionKeepsPeekedBytes(t *testing.T) {
	e := newPreEnv(t)
	srv, cli := net.Pipe()
	defer cli.Close() //nolint:errcheck // test pipe
	st := &cwConn{Conn: srv}
	send(cli, "12345678")
	s, err := e.p.Open(t.Context(), preReq(e, "pre", st))
	require.NoError(t, err)
	defer e.p.abort(s)

	send(cli, "more")
	got := make([]byte, 12)
	_, err = io.ReadFull(s.Stream, got)
	require.NoError(t, err)
	require.Equal(t, "12345678more", string(got), "Serve reads the stream from its first byte")
	cw, ok := s.Stream.(interface{ CloseWrite() error })
	require.True(t, ok, "CloseWrite is kept")
	require.NoError(t, cw.CloseWrite())
	require.True(t, st.closedWrite.Load())

	plain, cli2 := net.Pipe()
	defer cli2.Close() //nolint:errcheck // test pipe
	send(cli2, "12345678")
	s2, err := e.p.Open(t.Context(), preReq(e, "pre", plain))
	require.NoError(t, err)
	defer e.p.abort(s2)
	_, ok = s2.Stream.(interface{ CloseWrite() error })
	require.False(t, ok, "a stream without CloseWrite gains none")
}

// A successful PreSession keeps the stream open after its context and the
// request's end (Ruling C20).
func TestPipelinePreSessionStreamOpenAfterSuccess(t *testing.T) {
	e := newPreEnv(t)
	srv, cli := net.Pipe()
	defer cli.Close() //nolint:errcheck // test pipe
	ctx, cancel := context.WithCancel(t.Context())
	send(cli, "12345678")
	s, err := e.p.Open(ctx, preReq(e, "pre", srv))
	require.NoError(t, err)
	defer e.p.abort(s)
	cancel()
	time.Sleep(20 * time.Millisecond) // an AfterFunc left armed would close the stream now
	go func() { _, _ = s.Stream.Write([]byte("ok")) }()
	got := make([]byte, 2)
	require.NoError(t, cli.SetReadDeadline(time.Now().Add(2*time.Second)))
	_, err = io.ReadFull(cli, got)
	require.NoError(t, err, "the stream is still open")
	require.Equal(t, "ok", string(got))
}

// A silent client is closed at PreSessionTimeout, before StartSession.
func TestPipelinePreSessionTimeout(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		e := newPreEnv(t)
		srv, cli := net.Pipe()
		defer cli.Close() //nolint:errcheck // test pipe
		start := time.Now()
		_, err := e.p.Open(t.Context(), preReq(e, "pre", srv))
		require.ErrorIs(t, err, errPreSessionTimeout)
		require.Equal(t, PreSessionTimeout, time.Since(start))
		require.Zero(t, e.startCalls())
		d := e.p.heartbeat().GetDenials()
		require.Len(t, d, 1)
		require.Equal(t, "denied", d[0].GetReason())
		require.Zero(t, e.p.active.used())
		requireClosed(t, cli)
	})
}

// A PreSession panic refuses only this stream.
func TestPipelinePreSessionPanic(t *testing.T) {
	e := newPreEnv(t)
	srv, cli := net.Pipe()
	defer cli.Close() //nolint:errcheck // test pipe
	_, err := e.p.Open(t.Context(), preReq(e, "prepanic", srv))
	require.ErrorIs(t, err, errConnectorPanic)
	require.Zero(t, e.startCalls())
	require.Contains(t, e.metrics(t), "werbot_gateway_panics_total 1")
	requireClosed(t, cli)
}

// A PDP refusal ends Open before the stream is accepted or read.
func TestPipelinePreSessionNotCalledWhenDenied(t *testing.T) {
	e := newPreEnv(t)
	before := preCalls.Load()
	r := e.req("denied", KindStream)
	r.Target = "db:5432"
	r.Accept = func() (io.ReadWriteCloser, error) { return nil, errors.New("accept must not be called") }
	_, err := e.p.Open(t.Context(), r)
	require.ErrorIs(t, err, ErrDenied)
	require.Equal(t, before, preCalls.Load())
	require.Zero(t, e.startCalls())
}
