package pgconnector

import (
	"encoding/binary"
	"io"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// sendCancel sends a CancelRequest of key to addr and waits until the
// gateway closes the connection.
func sendCancel(t *testing.T, addr string, key uint64) {
	t.Helper()
	conn, err := net.Dial("tcp", addr)
	require.NoError(t, err)
	defer conn.Close()
	_, err = conn.Write(cancelPacket(t, key))
	require.NoError(t, err)
	require.NoError(t, conn.SetReadDeadline(time.Now().Add(5*time.Second)))
	got, err := io.ReadAll(conn)
	require.NoError(t, err)
	assert.Empty(t, got, "a cancel request gets no answer")
}

// keyOf is the cancel key a client got from the gateway.
func keyOf(conn *pgconn.PgConn) uint64 {
	return uint64(conn.PID())<<32 | uint64(binary.BigEndian.Uint32(conn.SecretKey()))
}

// sleep runs pg_sleep(seconds) on conn in the background.
func sleep(t *testing.T, conn *pgconn.PgConn, seconds string) <-chan error {
	errc := make(chan error, 1)
	go func() {
		_, err := conn.Exec(t.Context(), "select pg_sleep("+seconds+")").ReadAll()
		errc <- err
	}()
	time.Sleep(200 * time.Millisecond) // the query is running
	return errc
}

func TestCancelLongQuery(t *testing.T) {
	cfg, secret := realPG(t)
	conn, p := pgSession(t, cfg, secret)
	errc := sleep(t, conn, "30")
	t0 := time.Now()
	require.NoError(t, conn.CancelRequest(t.Context()))
	select {
	case err := <-errc:
		assert.Equal(t, "57014", pgErr(t, err).Code)
	case <-time.After(5 * time.Second):
		t.Fatal("the query was not canceled")
	}
	assert.Less(t, time.Since(t0), 2*time.Second)
	require.NoError(t, conn.Close(t.Context()))

	es := p.entries(t)
	require.Len(t, es, 2)
	assert.Equal(t, "cancel", es[0].Kind, "the cancel is logged in the session it canceled")
	assert.Equal(t, "query", es[1].Kind)
	assert.True(t, strings.HasPrefix(es[1].Error, "57014: "), es[1].Error)
	assert.Equal(t, int32(1), p.serves.Load(), "the cancel connection opened no session")
}

func TestCancelUnknownKey(t *testing.T) {
	cfg, secret := realPG(t)
	conn, p := pgSession(t, cfg, secret)
	other := serveProxy(t.Context(), t, testResource(t, cfg), secret) // another resource on the same database
	errc := sleep(t, conn, "0.5")
	sendCancel(t, p.addr, 0x5eed_0000_0000_0001) // registered by no session
	sendCancel(t, other.addr, keyOf(conn))
	require.NoError(t, <-errc, "neither cancel reached the query")
	require.NoError(t, conn.Close(t.Context()))

	es := p.entries(t)
	require.Len(t, es, 1)
	assert.Equal(t, "query", es[0].Kind)
	assert.Empty(t, es[0].Error)
	assert.Equal(t, int32(1), p.serves.Load())
	assert.Equal(t, int32(0), other.serves.Load())
}
