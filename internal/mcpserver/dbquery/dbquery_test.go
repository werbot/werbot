package dbquery

import (
	"context"
	"crypto/rand"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgproto3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeDialer is a Dialer whose every stream is a fake database answering
// with answer; it counts the dials and keeps the server ends.
type fakeDialer struct {
	answer func(b *pgproto3.Backend, sql string)

	mu     sync.Mutex
	dials  []string
	ends   []net.Conn
	closed chan struct{} // one value per served stream that ended
}

func newFakeDialer(answer func(b *pgproto3.Backend, sql string)) *fakeDialer {
	return &fakeDialer{answer: answer, closed: make(chan struct{}, 16)}
}

func (d *fakeDialer) dial(_ context.Context, resource string) (net.Conn, error) {
	client, server := net.Pipe()
	d.mu.Lock()
	d.dials, d.ends = append(d.dials, resource), append(d.ends, server)
	d.mu.Unlock()
	go func() {
		serveDB(server, d.answer)
		_ = server.Close()
		d.closed <- struct{}{}
	}()
	return client, nil
}

func (d *fakeDialer) count() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return len(d.dials)
}

// end is the server end of the n-th stream.
func (d *fakeDialer) end(n int) net.Conn {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.ends[n]
}

// serveDB answers the start-up of a client on conn, then each Query with
// answer and a ReadyForQuery; Terminate or a broken stream ends it.
func serveDB(conn net.Conn, answer func(b *pgproto3.Backend, sql string)) {
	b := pgproto3.NewBackend(conn, conn)
	if _, err := b.ReceiveStartupMessage(); err != nil {
		return
	}
	b.Send(&pgproto3.AuthenticationOk{})
	b.Send(&pgproto3.BackendKeyData{ProcessID: 1, SecretKey: []byte{0, 0, 0, 1}})
	b.Send(&pgproto3.ReadyForQuery{TxStatus: 'I'})
	for b.Flush() == nil {
		msg, err := b.Receive()
		if err != nil {
			return
		}
		q, ok := msg.(*pgproto3.Query)
		if !ok {
			return // Terminate
		}
		answer(b, q.String)
		b.Send(&pgproto3.ReadyForQuery{TxStatus: 'I'})
	}
}

// selectOne answers every query with one column "a" and one row "1".
func selectOne(b *pgproto3.Backend, _ string) {
	b.Send(&pgproto3.RowDescription{Fields: []pgproto3.FieldDescription{{Name: []byte("a"), DataTypeOID: 23}}})
	b.Send(&pgproto3.DataRow{Values: [][]byte{[]byte("1")}})
	b.Send(&pgproto3.CommandComplete{CommandTag: []byte("SELECT 1")})
}

func TestPoolWarmReuse(t *testing.T) {
	d := newFakeDialer(selectOne)
	p := NewPool(d.dial)
	for range 2 {
		res, err := p.Query(t.Context(), "postgres", Query{Resource: "db", Text: "SELECT 1"})
		require.NoError(t, err)
		assert.Equal(t, StatusOK, res.Status)
	}
	assert.Equal(t, 1, d.count(), "the second query reuses the warm stream")

	_ = d.end(0).Close() // the gateway ends the idle stream
	<-d.closed
	res, err := p.Query(t.Context(), "postgres", Query{Resource: "db", Text: "SELECT 1"})
	require.NoError(t, err, "a query on a stream that ended while idle runs on a new one")
	assert.Equal(t, StatusOK, res.Status)
	assert.Equal(t, 2, d.count())

	_, err = p.Query(t.Context(), "postgres", Query{Resource: "other", Text: "SELECT 1"})
	require.NoError(t, err)
	require.NoError(t, p.Close())
	for range 2 {
		select {
		case <-d.closed:
		case <-time.After(5 * time.Second):
			t.Fatal("Close left a stream open")
		}
	}
	_, err = p.Query(t.Context(), "postgres", Query{Resource: "db", Text: "SELECT 1"})
	require.Error(t, err, "a closed pool runs nothing")
	assert.Equal(t, 3, d.count())
}

func TestPoolUnknownType(t *testing.T) {
	d := newFakeDialer(selectOne)
	_, err := NewPool(d.dial).Query(t.Context(), "ssh", Query{Resource: "srv", Text: "id"})
	require.EqualError(t, err, "resource does not accept queries")
	assert.Zero(t, d.count(), "nothing is dialed for a type without queries")
}

// stubConn answers every query with res and counts its closes.
type stubConn struct {
	res    Result
	closes int
}

func (c *stubConn) Query(context.Context, Query) (Result, error) { return c.res, nil }
func (c *stubConn) Close() error                                 { c.closes++; return nil }

// A query left waiting for an approval closes its Conn, whatever the
// protocol: the held session never runs the statement later.
func TestPoolPendingClosesConn(t *testing.T) {
	c := &stubConn{res: Result{Status: StatusPending, ApprovalID: "a1"}}
	typ := "test-" + rand.Text() // unique: registrations last for the process
	Register(typ, func(context.Context, net.Conn) (Conn, error) { return c, nil })
	dials := 0
	p := NewPool(func(context.Context, string) (net.Conn, error) {
		dials++
		client, _ := net.Pipe()
		return client, nil
	})
	for range 2 {
		res, err := p.Query(t.Context(), typ, Query{Resource: "db", Text: "UPDATE t SET a = 1"})
		require.NoError(t, err)
		assert.Equal(t, "a1", res.ApprovalID)
	}
	assert.Equal(t, 2, c.closes)
	assert.Equal(t, 2, dials, "a held Conn is never reused")
}

func TestRegisterTwicePanics(t *testing.T) {
	open := func(context.Context, net.Conn) (Conn, error) { return nil, nil }
	assert.Panics(t, func() { Register("postgres", open) }, "postgres is registered in init")
	typ := "test-" + rand.Text()
	Register(typ, open)
	assert.Panics(t, func() { Register(typ, open) })
}
