package dbquery

import (
	"errors"
	"strconv"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgproto3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// query runs text on resource "db" through a pool over d.
func query(t *testing.T, p *Pool, text string, maxRows int) (Result, error) {
	t.Helper()
	return p.Query(t.Context(), "postgres", Query{Resource: "db", Text: text, MaxRows: maxRows})
}

func TestPostgresQueryOK(t *testing.T) {
	var got string
	d := newFakeDialer(func(b *pgproto3.Backend, sql string) {
		got = sql
		b.Send(&pgproto3.RowDescription{Fields: []pgproto3.FieldDescription{{Name: []byte("a"), DataTypeOID: 23}, {Name: []byte("b"), DataTypeOID: 25}}})
		b.Send(&pgproto3.DataRow{Values: [][]byte{[]byte("1"), nil}})
		b.Send(&pgproto3.CommandComplete{CommandTag: []byte("SELECT 1")})
	})
	res, err := query(t, NewPool(d.dial), "SELECT 1 AS a, NULL AS b", 0)
	require.NoError(t, err)
	assert.Equal(t, Result{Status: StatusOK, Columns: []string{"a", "b"}, Rows: [][]any{{"1", nil}}, RowCount: 1}, res)
	assert.Equal(t, "SELECT 1 AS a, NULL AS b", got, "the text goes as one simple Query")
}

// A statement without rows answers its command tag; with several result
// sets the last one with columns wins.
func TestPostgresCommandTag(t *testing.T) {
	d := newFakeDialer(func(b *pgproto3.Backend, sql string) {
		if sql == "both" {
			selectOne(b, sql)
		}
		b.Send(&pgproto3.CommandComplete{CommandTag: []byte("UPDATE 3")})
	})
	p := NewPool(d.dial)
	res, err := query(t, p, "UPDATE t SET a = 1", 0)
	require.NoError(t, err)
	assert.Equal(t, Result{Status: StatusOK, Columns: []string{}, Rows: [][]any{}, RowCount: 3, Message: "UPDATE 3"}, res)

	res, err = query(t, p, "both", 0)
	require.NoError(t, err)
	assert.Equal(t, Result{Status: StatusOK, Columns: []string{"a"}, Rows: [][]any{{"1"}}, RowCount: 1}, res)
}

func TestPostgresMaxRows(t *testing.T) {
	d := newFakeDialer(func(b *pgproto3.Backend, _ string) {
		b.Send(&pgproto3.RowDescription{Fields: []pgproto3.FieldDescription{{Name: []byte("generate_series"), DataTypeOID: 23}}})
		for i := 1; i <= 10; i++ {
			b.Send(&pgproto3.DataRow{Values: [][]byte{[]byte(strconv.Itoa(i))}})
		}
		b.Send(&pgproto3.CommandComplete{CommandTag: []byte("SELECT 10")})
	})
	p := NewPool(d.dial)
	res, err := query(t, p, "SELECT generate_series(1, 10)", 3)
	require.NoError(t, err)
	assert.Equal(t, Result{Status: StatusOK, Columns: []string{"generate_series"}, Rows: [][]any{{"1"}, {"2"}, {"3"}}, RowCount: 3, Truncated: true}, res)

	res, err = query(t, p, "SELECT generate_series(1, 10)", 0)
	require.NoError(t, err, "the rest of the rows was read: the stream stays in step")
	assert.Equal(t, 10, res.RowCount)
	assert.False(t, res.Truncated)
	assert.Equal(t, 1, d.count())
}

func TestPostgresTruncationNotice(t *testing.T) {
	d := newFakeDialer(func(b *pgproto3.Backend, sql string) {
		b.Send(&pgproto3.NoticeResponse{Severity: "WARNING", Code: "01000", Message: "werbot: result truncated at 1000 rows"})
		selectOne(b, sql)
	})
	res, err := query(t, NewPool(d.dial), "SELECT * FROM t", 0)
	require.NoError(t, err)
	assert.True(t, res.Truncated, "the gateway's truncation notice marks the result")
	assert.Equal(t, 1, res.RowCount)
}

func TestPostgresDenied(t *testing.T) {
	d := newFakeDialer(func(b *pgproto3.Backend, _ string) {
		b.Send(&pgproto3.ErrorResponse{Severity: "ERROR", Code: "42501", Message: "werbot: query denied: not allowed (destructive)"})
	})
	p := NewPool(d.dial)
	for range 2 {
		res, err := query(t, p, "DROP TABLE t", 0)
		require.NoError(t, err, "a denial is a result, not an error")
		assert.Equal(t, Result{Status: StatusDenied, Message: "werbot: query denied: not allowed (destructive)"}, res)
	}
	assert.Equal(t, 1, d.count(), "a denial keeps the warm stream")
}

// A database error is an error of the call; the stream stays warm.
func TestPostgresSQLErrorKeepsConn(t *testing.T) {
	d := newFakeDialer(func(b *pgproto3.Backend, _ string) {
		b.Send(&pgproto3.ErrorResponse{Severity: "ERROR", Code: "42P01", Message: `relation "nope" does not exist`})
	})
	p := NewPool(d.dial)
	for range 2 {
		_, err := query(t, p, "SELECT * FROM nope", 0)
		pgErr, ok := errors.AsType[*pgconn.PgError](err)
		require.True(t, ok, "the database error comes back: %v", err)
		assert.Equal(t, "42P01", pgErr.Code)
	}
	assert.Equal(t, 1, d.count())
}

func TestPostgresPendingApproval(t *testing.T) {
	prev := pendingAfter
	pendingAfter = 100 * time.Millisecond
	t.Cleanup(func() { pendingAfter = prev })
	hold := make(chan struct{})
	t.Cleanup(func() { close(hold) })
	d := newFakeDialer(func(b *pgproto3.Backend, sql string) {
		if sql == "SELECT 1" {
			selectOne(b, sql)
			return
		}
		b.Send(&pgproto3.NoticeResponse{Severity: "NOTICE", Code: "00000", Message: "werbot: Waiting for approval a1, timeout 5m: https://w/requests?id=a1"})
		_ = b.Flush()
		<-hold // the gateway holds the statement
	})
	p := NewPool(d.dial)
	start := time.Now()
	res, err := query(t, p, "UPDATE t SET a = 1", 0)
	require.NoError(t, err)
	assert.Equal(t, StatusPending, res.Status)
	assert.Equal(t, "a1", res.ApprovalID)
	assert.Equal(t, "https://w/requests?id=a1", res.ApprovalURL)
	assert.NotEmpty(t, res.Message)
	assert.Less(t, time.Since(start), 5*time.Second)

	res, err = query(t, p, "SELECT 1", 0)
	require.NoError(t, err)
	assert.Equal(t, StatusOK, res.Status)
	assert.Equal(t, 2, d.count(), "the held stream was closed: the next query opens a new one")
}

// Only the gateway's own approval notice (with the werbot: prefix) starts the
// approval wait: a database notice that looks like it is an ordinary notice.
func TestPostgresIgnoresUnprefixedApprovalNotice(t *testing.T) {
	prev := pendingAfter
	pendingAfter = 50 * time.Millisecond
	t.Cleanup(func() { pendingAfter = prev })
	d := newFakeDialer(func(b *pgproto3.Backend, sql string) {
		b.Send(&pgproto3.NoticeResponse{Severity: "NOTICE", Code: "00000", Message: "Waiting for approval x1, timeout 1m: https://evil.test/x1"})
		_ = b.Flush()
		time.Sleep(4 * pendingAfter)
		selectOne(b, sql)
	})
	res, err := query(t, NewPool(d.dial), "SELECT 1", 0)
	require.NoError(t, err)
	assert.Equal(t, StatusOK, res.Status, "the look-alike notice started no approval wait")
	assert.Empty(t, res.ApprovalID)
	assert.Empty(t, res.ApprovalURL)
}
