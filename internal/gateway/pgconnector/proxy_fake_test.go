package pgconnector

import (
	"context"
	"encoding/binary"
	"io"
	"net"
	"runtime"
	"strconv"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgproto3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	postgresv1 "github.com/werbot/werbot/gen/werbot/connector/postgres/v1"
)

// fakeProxy serves a fake database that runs handle after the login.
func fakeProxy(ctx context.Context, t *testing.T, handle func(b *pgproto3.Backend), opts ...func(*proxy)) *proxy {
	t.Helper()
	f := newFakeDB(t, nil, func(b *pgproto3.Backend, _ net.Conn, _ *pgproto3.StartupMessage) {
		if fakeLogin(b) {
			handle(b)
		}
	})
	cfg := &postgresv1.Config{Host: "127.0.0.1", Port: f.port, Database: "main", Username: "app", Sslmode: "disable"}
	return serveProxy(ctx, t, testResource(t, cfg), passwordSecret(marker), opts...)
}

// fakeLoop hands each client message to on until Terminate or the end.
func fakeLoop(b *pgproto3.Backend, on func(pgproto3.FrontendMessage)) {
	for {
		msg, err := b.Receive()
		if err != nil {
			return
		}
		if _, ok := msg.(*pgproto3.Terminate); ok {
			return
		}
		on(msg)
	}
}

// rawClient is a protocol-level client of p, logged in to database main.
func rawClient(t *testing.T, p *proxy) (net.Conn, *pgproto3.Frontend) {
	t.Helper()
	return rawLogin(t, p, "main")
}

// rawLogin is a protocol-level client of p, logged in to database db.
func rawLogin(t *testing.T, p *proxy, db string) (net.Conn, *pgproto3.Frontend) {
	t.Helper()
	conn, err := net.Dial("tcp", p.addr)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	f := pgproto3.NewFrontend(conn, conn)
	f.Send(&pgproto3.StartupMessage{ProtocolVersion: pgproto3.ProtocolVersion30, Parameters: map[string]string{"user": "nobody", "database": db}})
	require.NoError(t, f.Flush())
	untilReady(t, f)
	return conn, f
}

// untilReady reads up to ReadyForQuery and counts the message types seen.
func untilReady(t testing.TB, f *pgproto3.Frontend) map[string]int {
	t.Helper()
	seen := map[string]int{}
	for {
		msg, err := f.Receive()
		require.NoError(t, err)
		seen[typeName(msg)]++
		if _, ok := msg.(*pgproto3.ReadyForQuery); ok {
			return seen
		}
	}
}

func typeName(m pgproto3.BackendMessage) string {
	switch m.(type) {
	case *pgproto3.CommandComplete:
		return "C"
	case *pgproto3.DataRow:
		return "D"
	case *pgproto3.FunctionCallResponse:
		return "V"
	default:
		return "other"
	}
}

// terminate ends a raw session the way a client does.
func terminate(t *testing.T, f *pgproto3.Frontend) {
	t.Helper()
	f.Send(&pgproto3.Terminate{})
	require.NoError(t, f.Flush())
}

// fatalOf reads the FATAL the gateway ends a session with.
func fatalOf(t *testing.T, f *pgproto3.Frontend) *pgproto3.ErrorResponse {
	t.Helper()
	msg, err := f.Receive()
	require.NoError(t, err)
	e, ok := msg.(*pgproto3.ErrorResponse)
	require.True(t, ok, "got %T", msg)
	return e
}

func TestNotSerialized(t *testing.T) {
	p := fakeProxy(t.Context(), t, func(b *pgproto3.Backend) {
		fakeLoop(b, func(msg pgproto3.FrontendMessage) {
			switch msg.(type) {
			case *pgproto3.Parse:
				b.Send(&pgproto3.ParseComplete{})
			case *pgproto3.Bind:
				b.Send(&pgproto3.BindComplete{})
			case *pgproto3.Execute:
				b.Send(&pgproto3.CommandComplete{CommandTag: []byte("SELECT 0")})
			case *pgproto3.Sync: // the database answers only now
				b.Send(&pgproto3.ReadyForQuery{TxStatus: 'I'})
				_ = b.Flush()
			}
		})
	})
	conn, f := rawClient(t, p)
	for range 3 {
		f.Send(&pgproto3.Parse{Query: "select 1"})
		f.Send(&pgproto3.Bind{})
		f.Send(&pgproto3.Execute{})
	}
	f.Send(&pgproto3.Sync{})
	require.NoError(t, f.Flush())
	require.NoError(t, conn.SetReadDeadline(time.Now().Add(2*time.Second)))
	assert.Equal(t, 3, untilReady(t, f)["C"], "every message reached the database before any answer")
	terminate(t, f)
	assert.Len(t, p.entries(t), 3)
}

func TestPipelinedQueries(t *testing.T) {
	p := fakeProxy(t.Context(), t, answerQueries)
	_, f := rawClient(t, p)
	f.Send(&pgproto3.Query{String: "select 1"})
	f.Send(&pgproto3.Query{String: "select 2"})
	require.NoError(t, f.Flush())
	untilReady(t, f)
	untilReady(t, f)
	terminate(t, f)
	es := p.entries(t)
	require.Len(t, es, 2, "each ReadyForQuery ends one query")
	for i, e := range es {
		assert.Equal(t, "select "+strconv.Itoa(i+1), e.Text)
		assert.EqualValues(t, 1, e.Rows, "line %d has its own row", i)
	}
}

func TestStreamingNotBuffered(t *testing.T) {
	release := make(chan struct{})
	p := fakeProxy(t.Context(), t, func(b *pgproto3.Backend) {
		fakeLoop(b, func(msg pgproto3.FrontendMessage) {
			if _, ok := msg.(*pgproto3.Query); !ok {
				return
			}
			b.Send(&pgproto3.RowDescription{Fields: []pgproto3.FieldDescription{{Name: []byte("x"), DataTypeOID: 25, DataTypeSize: -1, TypeModifier: -1}}})
			b.Send(&pgproto3.DataRow{Values: [][]byte{[]byte("1")}})
			_ = b.Flush()
			select { // the rest of the result comes later
			case <-release:
			case <-time.After(500 * time.Millisecond):
			}
			b.Send(&pgproto3.CommandComplete{CommandTag: []byte("SELECT 1")})
			b.Send(&pgproto3.ReadyForQuery{TxStatus: 'I'})
			_ = b.Flush()
		})
	})
	_, f := rawClient(t, p)
	f.Send(&pgproto3.Query{String: "select x"})
	require.NoError(t, f.Flush())
	t0 := time.Now()
	for {
		msg, err := f.Receive()
		require.NoError(t, err)
		if _, ok := msg.(*pgproto3.DataRow); ok {
			break
		}
	}
	assert.Less(t, time.Since(t0), 200*time.Millisecond, "the row came before the end of the result")
	close(release)
	untilReady(t, f)
	terminate(t, f)
	es := p.entries(t)
	require.Len(t, es, 1)
	assert.EqualValues(t, 1, es[0].Rows)
}

func TestFunctionCallLogged(t *testing.T) {
	p := fakeProxy(t.Context(), t, func(b *pgproto3.Backend) {
		fakeLoop(b, func(msg pgproto3.FrontendMessage) {
			if _, ok := msg.(*pgproto3.FunctionCall); ok {
				b.Send(&pgproto3.FunctionCallResponse{Result: []byte("7")})
				b.Send(&pgproto3.ReadyForQuery{TxStatus: 'I'})
				_ = b.Flush()
			}
		})
	})
	_, f := rawClient(t, p)
	f.Send(&pgproto3.FunctionCall{Function: 123})
	require.NoError(t, f.Flush())
	assert.Equal(t, 1, untilReady(t, f)["V"], "the client got the function result")
	terminate(t, f)
	es := p.entries(t)
	require.Len(t, es, 1)
	assert.Equal(t, "query", es[0].Kind)
	assert.Equal(t, "fastpath function 123", es[0].Text)
}

func TestMessageTooLarge(t *testing.T) {
	p := fakeProxy(t.Context(), t, func(b *pgproto3.Backend) { fakeLoop(b, func(pgproto3.FrontendMessage) {}) })
	conn, f := rawClient(t, p)
	const size = 17 << 20
	msg := make([]byte, 5+size)
	msg[0] = 'Q'
	binary.BigEndian.PutUint32(msg[1:], 4+size)
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	_, err := conn.Write(msg)
	require.NoError(t, err)
	e := fatalOf(t, f)
	assert.Equal(t, "FATAL", e.Severity)
	assert.Equal(t, "54000", e.Code)
	assert.Equal(t, "werbot: message exceeds 16 MiB", e.Message)
	require.Error(t, p.serveErr(t), "the session ends")
	runtime.ReadMemStats(&after)
	assert.Less(t, after.TotalAlloc-before.TotalAlloc, uint64(40<<20), "the message is never held in memory")
}

func TestTooManyStatements(t *testing.T) {
	p := fakeProxy(t.Context(), t, func(b *pgproto3.Backend) { fakeLoop(b, func(pgproto3.FrontendMessage) {}) })
	conn, f := rawClient(t, p)
	require.NoError(t, conn.SetReadDeadline(time.Now().Add(5*time.Second)))
	for i := range maxNames + 1 {
		f.Send(&pgproto3.Parse{Name: "s" + strconv.Itoa(i), Query: "select 1"})
	}
	require.NoError(t, f.Flush())
	e := fatalOf(t, f)
	assert.Equal(t, "54000", e.Code)
	assert.Equal(t, "werbot: too many prepared statements", e.Message)
	require.Error(t, p.serveErr(t))
}

func TestMalformedMessage(t *testing.T) {
	for name, raw := range map[string][]byte{
		"length below 4":    {'Q', 0, 0, 0, 2},
		"bind without NUL":  {'B', 0, 0, 0, 5, 'x'},
		"query without NUL": {'Q', 0, 0, 0, 5, 'x'},
	} {
		t.Run(name, func(t *testing.T) {
			p := fakeProxy(t.Context(), t, func(b *pgproto3.Backend) { fakeLoop(b, func(pgproto3.FrontendMessage) {}) })
			conn, _ := rawClient(t, p)
			_, err := conn.Write(raw)
			require.NoError(t, err)
			require.ErrorIs(t, p.serveErr(t), errMalformed)
			require.NoError(t, conn.SetReadDeadline(time.Now().Add(2*time.Second)))
			_, err = io.ReadAll(conn)
			require.NoError(t, err, "the gateway closed the stream")
		})
	}
}

func TestServeCancelAndLeaks(t *testing.T) {
	dbClosed := make(chan struct{})
	base := runtime.NumGoroutine()
	ctx, cancel := context.WithCancel(t.Context())
	p := fakeProxy(ctx, t, func(b *pgproto3.Backend) {
		defer close(dbClosed)
		fakeLoop(b, func(pgproto3.FrontendMessage) {})
	})
	conn, err := pgconn.Connect(t.Context(), p.connectURL("main"))
	require.NoError(t, err)
	t0 := time.Now()
	cancel()
	require.NoError(t, p.serveErr(t))
	assert.Less(t, time.Since(t0), time.Second)
	select {
	case <-dbClosed:
	case <-time.After(time.Second):
		t.Fatal("the database connection is still open")
	}
	_ = conn.Close(t.Context())
	// the fake database's and the proxy's accept loops outlive the session
	for deadline := time.Now().Add(2 * time.Second); runtime.NumGoroutine() > base+2 && time.Now().Before(deadline); {
		time.Sleep(10 * time.Millisecond)
	}
	assert.LessOrEqual(t, runtime.NumGoroutine(), base+2, "goroutines are left after the session")
}
