package pgconnector

import (
	"bufio"
	"context"
	"crypto/tls"
	"errors"
	"io"
	"net"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgproto3"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protojson"

	postgresv1 "github.com/werbot/werbot/gen/werbot/connector/postgres/v1"
	"github.com/werbot/werbot/internal/connector"
	"github.com/werbot/werbot/internal/gateway"
	"github.com/werbot/werbot/internal/platform/db/dbtest"
)

// marker is the resource password of the fake databases; tests check it never leaks.
const marker = "werbot-secret-marker-7f3a"

func passwordSecret(v string) gateway.Secret {
	return gateway.Secret{Kind: string(connector.CredentialPassword), Value: []byte(v)}
}

// testResource is a postgres resource of cfg.
func testResource(t testing.TB, cfg *postgresv1.Config) gateway.Resource {
	t.Helper()
	raw, err := protojson.Marshal(cfg)
	require.NoError(t, err)
	return gateway.Resource{ID: uuid.New(), Name: "pg-test", Type: "postgres", Config: raw}
}

// realPG is the werbot-testdb database of a fresh dbtest.New, with the
// credentials of its URL (role pgtdbuser, SCRAM).
func realPG(t testing.TB) (*postgresv1.Config, gateway.Secret) {
	t.Helper()
	_, url := dbtest.New(t)
	pc, err := pgconn.ParseConfig(url)
	require.NoError(t, err)
	cfg := &postgresv1.Config{Host: pc.Host, Port: uint32(pc.Port), Database: pc.Database, Username: pc.User, Sslmode: "disable"}
	return cfg, passwordSecret(pc.Password)
}

// memRecording keeps the querylog lines of a session.
type memRecording struct {
	mu     sync.Mutex
	events []any
}

func (*memRecording) Output([]byte)               {}
func (*memRecording) Input([]byte)                {}
func (*memRecording) Resize(int, int)             {}
func (*memRecording) Marker(string)               {}
func (*memRecording) Close(context.Context) error { return nil }

func (m *memRecording) Event(v any) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.events = append(m.events, v)
	return nil
}

func (m *memRecording) len() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.events)
}

// proxy serves postgres sessions of one resource the way the pipeline does:
// PreSession on the peeked stream, then Serve on the replayed one.
type proxy struct {
	addr      string
	rec       *memRecording
	recording gateway.RecordingSink // what Serve records to; rec unless a test sets it
	dials     atomic.Int32
	serves    atomic.Int32
	errs      chan error // Serve results
	dialErr   error      // when set, Session.Dial refuses with it
	hooks     gateway.SessionHooks
}

func serveProxy(ctx context.Context, t testing.TB, res gateway.Resource, secret gateway.Secret, opts ...func(*proxy)) *proxy {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = ln.Close() })
	p := &proxy{addr: ln.Addr().String(), rec: &memRecording{}, errs: make(chan error, 64)}
	p.recording = p.rec
	for _, o := range opts {
		o(p)
	}
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go p.serve(ctx, res, secret, conn)
		}
	}()
	return p
}

func (p *proxy) dial(ctx context.Context, network, address string) (net.Conn, error) {
	p.dials.Add(1)
	if p.dialErr != nil {
		return nil, p.dialErr
	}
	var d net.Dialer
	return d.DialContext(ctx, network, address)
}

func (p *proxy) serve(ctx context.Context, res gateway.Resource, secret gateway.Secret, conn net.Conn) {
	defer conn.Close()
	br := bufio.NewReaderSize(conn, gateway.PreSessionBuffer)
	pctx, cancel := context.WithTimeout(ctx, gateway.PreSessionTimeout)
	out, err := Connector{}.PreSession(pctx, res, br.Peek, conn)
	cancel()
	if err != nil || out.Handled {
		return
	}
	p.serves.Add(1)
	s := &gateway.Session{
		ID: uuid.New(), Kind: gateway.KindStream, Resource: res, Secret: secret,
		Stream: struct {
			io.Reader
			io.WriteCloser
		}{br, conn},
		Dial: p.dial, Recording: p.recording,
	}
	p.errs <- Connector{}.Serve(gateway.WithHooks(ctx, p.hooks), s)
}

// serveErr is the result of the next Serve of p.
func (p *proxy) serveErr(t testing.TB) error {
	t.Helper()
	select {
	case err := <-p.errs:
		return err
	case <-time.After(10 * time.Second):
		t.Fatal("Serve did not return")
		return nil
	}
}

// connectURL is a sslmode=disable client URL of p as user nobody.
func (p *proxy) connectURL(db string) string {
	return "postgres://nobody@" + p.addr + "/" + db + "?sslmode=disable"
}

// fakeDB is a database fake on 127.0.0.1: it answers SSLRequest with S and
// TLS when tlsCfg is set (N otherwise), records cancel requests and hands
// every StartupMessage to handle.
type fakeDB struct {
	port    uint32
	cancels chan *pgproto3.CancelRequest
}

func newFakeDB(t testing.TB, tlsCfg *tls.Config, handle func(b *pgproto3.Backend, conn net.Conn, sm *pgproto3.StartupMessage)) *fakeDB {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = ln.Close() })
	_, port, _ := net.SplitHostPort(ln.Addr().String())
	n, _ := strconv.Atoi(port)
	f := &fakeDB{port: uint32(n), cancels: make(chan *pgproto3.CancelRequest, 8)} //nolint:gosec // a TCP port
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go f.serve(conn, tlsCfg, handle)
		}
	}()
	return f
}

func (f *fakeDB) serve(conn net.Conn, tlsCfg *tls.Config, handle func(*pgproto3.Backend, net.Conn, *pgproto3.StartupMessage)) {
	defer conn.Close()
	b := pgproto3.NewBackend(conn, conn)
	for {
		msg, err := b.ReceiveStartupMessage()
		if err != nil {
			return
		}
		switch m := msg.(type) {
		case *pgproto3.SSLRequest:
			if tlsCfg == nil {
				_, _ = conn.Write([]byte{'N'})
				continue
			}
			_, _ = conn.Write([]byte{'S'})
			tc := tls.Server(conn, tlsCfg)
			if tc.Handshake() != nil {
				return
			}
			conn, b = tc, pgproto3.NewBackend(tc, tc)
		case *pgproto3.CancelRequest:
			f.cancels <- &pgproto3.CancelRequest{ProcessID: m.ProcessID, SecretKey: append([]byte(nil), m.SecretKey...)}
			return
		case *pgproto3.StartupMessage:
			handle(b, conn, m)
			return
		}
	}
}

// fakePID and fakeSecret are the BackendKeyData of the fake database.
const fakePID = 4242

var fakeSecret = []byte{1, 2, 3, 4}

// fakeLogin completes a login; false when the connection is gone.
func fakeLogin(b *pgproto3.Backend) bool {
	b.Send(&pgproto3.AuthenticationOk{})
	b.Send(&pgproto3.ParameterStatus{Name: "server_version", Value: "18.0 (fake)"})
	b.Send(&pgproto3.ParameterStatus{Name: "client_encoding", Value: "UTF8"})
	b.Send(&pgproto3.ParameterStatus{Name: "standard_conforming_strings", Value: "on"})
	b.Send(&pgproto3.BackendKeyData{ProcessID: fakePID, SecretKey: fakeSecret})
	b.Send(&pgproto3.ReadyForQuery{TxStatus: 'I'})
	return b.Flush() == nil
}

// acceptSession completes a login and answers every query with one row "1".
func acceptSession(b *pgproto3.Backend) {
	if fakeLogin(b) {
		answerQueries(b)
	}
}

// answerQueries answers every query with one row "1".
func answerQueries(b *pgproto3.Backend) {
	for {
		msg, err := b.Receive()
		if err != nil {
			return
		}
		switch msg.(type) {
		case *pgproto3.Terminate:
			return
		case *pgproto3.Query:
			b.Send(&pgproto3.RowDescription{Fields: []pgproto3.FieldDescription{{Name: []byte("x"), DataTypeOID: 25, DataTypeSize: -1, TypeModifier: -1}}})
			b.Send(&pgproto3.DataRow{Values: [][]byte{[]byte("1")}})
			b.Send(&pgproto3.CommandComplete{CommandTag: []byte("SELECT 1")})
			b.Send(&pgproto3.ReadyForQuery{TxStatus: 'I'})
			if b.Flush() != nil {
				return
			}
		}
	}
}

// pgErr is the *pgconn.PgError in err; it fails the test without one.
func pgErr(t testing.TB, err error) *pgconn.PgError {
	t.Helper()
	pe, ok := errors.AsType[*pgconn.PgError](err)
	require.True(t, ok, "not a server error: %v", err)
	return pe
}
