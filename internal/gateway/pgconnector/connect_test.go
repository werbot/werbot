package pgconnector

import (
	"bytes"
	"context"
	"log/slog"
	"net"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgproto3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	postgresv1 "github.com/werbot/werbot/gen/werbot/connector/postgres/v1"
	"github.com/werbot/werbot/internal/gateway"
	"github.com/werbot/werbot/internal/gateway/dbproxy"
)

func TestConnectSCRAM(t *testing.T) {
	cfg, secret := realPG(t)
	p := serveProxy(t.Context(), t, testResource(t, cfg), secret)
	conn, err := pgconn.Connect(t.Context(), p.connectURL(cfg.GetDatabase()))
	require.NoError(t, err)
	defer conn.Close(t.Context())
	assert.True(t, strings.HasPrefix(conn.ParameterStatus("server_version"), "18"))
	rows, err := conn.Exec(t.Context(), "select pg_backend_pid(), current_user").ReadAll()
	require.NoError(t, err)
	require.Len(t, rows, 1)
	pid, err := strconv.ParseUint(string(rows[0].Rows[0][0]), 10, 32)
	require.NoError(t, err)
	assert.NotEqual(t, uint32(pid), conn.PID(), "the client sees the gateway's key, not the database pid")
	assert.Equal(t, cfg.GetUsername(), string(rows[0].Rows[0][1]), "the client user does not matter")
}

func TestStartupProtocol32(t *testing.T) {
	cfg, secret := realPG(t)
	p := serveProxy(t.Context(), t, testResource(t, cfg), secret)
	conn, err := net.Dial("tcp", p.addr)
	require.NoError(t, err)
	defer conn.Close()
	f := pgproto3.NewFrontend(conn, conn)
	f.Send(&pgproto3.StartupMessage{ProtocolVersion: pgproto3.ProtocolVersion32, Parameters: map[string]string{
		"user": "nobody", "database": cfg.GetDatabase(), "_pq_.x": "1",
	}})
	require.NoError(t, f.Flush())
	msg, err := f.Receive()
	require.NoError(t, err)
	assert.Equal(t, &pgproto3.NegotiateProtocolVersion{NewestMinorProtocol: 0, UnrecognizedOptions: []string{"_pq_.x"}}, msg)
	msg, err = f.Receive()
	require.NoError(t, err)
	assert.IsType(t, &pgproto3.AuthenticationOk{}, msg)
	for {
		msg, err = f.Receive()
		require.NoError(t, err)
		if k, ok := msg.(*pgproto3.BackendKeyData); ok {
			assert.Len(t, k.SecretKey, 4)
		}
		if _, ok := msg.(*pgproto3.ReadyForQuery); ok {
			break
		}
	}
	f.Send(&pgproto3.Query{String: "select 42"})
	require.NoError(t, f.Flush())
	var got string
	for {
		msg, err = f.Receive()
		require.NoError(t, err)
		if d, ok := msg.(*pgproto3.DataRow); ok {
			got = string(d.Values[0])
		}
		if _, ok := msg.(*pgproto3.ReadyForQuery); ok {
			break
		}
	}
	assert.Equal(t, "42", got)
}

func TestPsqlConnect(t *testing.T) {
	psql, err := exec.LookPath("psql")
	if err != nil {
		if os.Getenv("CI") == "" {
			t.Skip("psql is not installed")
		}
		t.Fatal("psql is required in CI")
	}
	cfg, secret := realPG(t)
	p := serveProxy(t.Context(), t, testResource(t, cfg), secret)
	url := "postgres://" + p.addr + "/" + cfg.GetDatabase() + "?sslmode=disable"
	out, err := exec.CommandContext(t.Context(), psql, url, "-X", "-A", "-t", "-c", "select 42").Output() //nolint:gosec // the psql on PATH
	require.NoError(t, err)
	assert.Equal(t, "42", strings.TrimSpace(string(out)))
}

// lockedBuffer is a slog sink safe for concurrent handlers.
type lockedBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (l *lockedBuffer) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.Write(p)
}

func (l *lockedBuffer) has(s string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return bytes.Contains(l.b.Bytes(), []byte(s))
}

// captureLog sends slog.Default to a buffer for the test.
func captureLog(t *testing.T) *lockedBuffer {
	logs := &lockedBuffer{}
	old := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(logs, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(old) })
	return logs
}

func TestWrongPasswordNoLeak(t *testing.T) {
	logs := captureLog(t)
	cfg, _ := realPG(t)
	p := serveProxy(t.Context(), t, testResource(t, cfg), passwordSecret(marker))
	_, err := pgconn.Connect(t.Context(), p.connectURL(cfg.GetDatabase()))
	e := pgErr(t, err)
	assert.Equal(t, "28P01", e.Code)
	assert.True(t, strings.HasPrefix(e.Message, "werbot: connection to pg-test failed: "))
	serr := p.serveErr(t)
	require.Error(t, serr)
	if strings.Contains(err.Error(), marker) || strings.Contains(e.Detail+e.Hint, marker) {
		t.Fatal("the credential reached the client")
	}
	if strings.Contains(serr.Error(), marker) {
		t.Fatal("the credential reached the Serve error")
	}
	if logs.has(marker) {
		t.Fatal("the credential reached slog")
	}
}

// panicStream panics on the first read after the connector wrote to it.
type panicStream struct {
	net.Conn
	wrote atomic.Bool
}

func (p *panicStream) Write(b []byte) (int, error) { p.wrote.Store(true); return p.Conn.Write(b) }

func (p *panicStream) Read(b []byte) (int, error) {
	if p.wrote.Load() {
		panic("relay test panic")
	}
	return p.Conn.Read(b)
}

func TestConnectorLifecycle(t *testing.T) {
	logs := captureLog(t)
	f := newFakeDB(t, nil, func(b *pgproto3.Backend, _ net.Conn, _ *pgproto3.StartupMessage) { acceptSession(b) })
	cfg := &postgresv1.Config{Host: "127.0.0.1", Port: f.port, Database: "main", Username: "app", Sslmode: "disable"}
	res := testResource(t, cfg)
	base := runtime.NumGoroutine()

	ctx, cancel := context.WithCancel(t.Context())
	p := serveProxy(ctx, t, res, passwordSecret(marker))
	conn, err := pgconn.Connect(t.Context(), p.connectURL("main"))
	require.NoError(t, err)
	require.NoError(t, conn.CancelRequest(t.Context()), "a cancel request goes to the gateway")
	select {
	case c := <-f.cancels:
		assert.Equal(t, uint32(fakePID), c.ProcessID, "the gateway cancels with the database key")
		assert.Equal(t, fakeSecret, c.SecretKey)
	case <-time.After(5 * time.Second):
		t.Fatal("the database got no cancel request")
	}
	require.Eventually(t, func() bool { return p.rec.len() == 1 }, 2*time.Second, 10*time.Millisecond)
	p.rec.mu.Lock()
	assert.Equal(t, "cancel", p.rec.events[0].(dbproxy.Entry).Kind, "Recording.Event gets the cancel line")
	p.rec.mu.Unlock()

	t0 := time.Now()
	cancel()
	require.NoError(t, p.serveErr(t), "a cancel is not an error")
	assert.Less(t, time.Since(t0), time.Second)
	_ = conn.Close(t.Context())
	// the proxy's accept loop outlives the session
	for deadline := time.Now().Add(2 * time.Second); runtime.NumGoroutine() > base+1 && time.Now().Before(deadline); {
		time.Sleep(10 * time.Millisecond)
	}
	assert.LessOrEqual(t, runtime.NumGoroutine(), base+1, "goroutines are left after the session")
	assert.Equal(t, int32(2), p.dials.Load(), "the session and its cancel dial through Session.Dial")
	if logs.has(marker) {
		t.Fatal("the credential reached slog")
	}

	t.Run("relay panic", func(t *testing.T) {
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		require.NoError(t, err)
		defer ln.Close()
		go func() {
			if c, err := pgconn.Connect(t.Context(), "postgres://nobody@"+ln.Addr().String()+"/main?sslmode=disable"); err == nil {
				_, _ = c.Exec(t.Context(), "select 1").ReadAll()
				_ = c.Close(t.Context())
			}
		}()
		sc, err := ln.Accept()
		require.NoError(t, err)
		defer sc.Close()
		var d net.Dialer
		s := &gateway.Session{ID: uuid.New(), Kind: gateway.KindStream, Resource: res, Secret: passwordSecret(marker),
			Stream: &panicStream{Conn: sc}, Dial: d.DialContext, Recording: &memRecording{}}
		assert.Panics(t, func() { _ = Connector{}.Serve(t.Context(), s) }, "a relay panic re-panics on Serve")
	})
}

// BenchmarkPostgresConnect compares a direct connect to the test database with
// one through the connector and reports the added p99 (budget 30 ms, gated by
// TestHotPathBudget).
func BenchmarkPostgresConnect(b *testing.B) {
	cfg, secret := realPG(b)
	p := serveProxy(b.Context(), b, testResource(b, cfg), secret)
	direct := directURL(cfg, secret)
	var d, v []time.Duration
	connect := func(url string) time.Duration {
		t0 := time.Now()
		conn, err := pgconn.Connect(b.Context(), url)
		if err != nil {
			b.Fatal("connect failed")
		}
		el := time.Since(t0)
		_ = conn.Close(b.Context())
		return el
	}
	for b.Loop() {
		d = append(d, connect(direct))
		v = append(v, connect(p.connectURL(cfg.GetDatabase())))
	}
	added := pct(v, 99) - pct(d, 99)
	b.ReportMetric(float64(added.Microseconds())/1000, "added-p99-ms")
}
