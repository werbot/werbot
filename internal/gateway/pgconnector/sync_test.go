package pgconnector

import (
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgproto3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/gateway/dbproxy"
)

// realRaw is a protocol-level client of a proxy to a fresh test database,
// with table c (n int) created.
func realRaw(t *testing.T) (*proxy, *pgproto3.Frontend) {
	t.Helper()
	cfg, secret := realPG(t)
	p := serveProxy(t.Context(), t, testResource(t, cfg), secret)
	conn, f := rawLogin(t, p, cfg.GetDatabase())
	require.NoError(t, conn.SetReadDeadline(time.Now().Add(10*time.Second)))
	send(t, f, &pgproto3.Query{String: "create table c (n int)"})
	untilReady(t, f)
	return p, f
}

// send writes msgs to the database in one flush.
func send(t *testing.T, f *pgproto3.Frontend, msgs ...pgproto3.FrontendMessage) {
	t.Helper()
	for _, m := range msgs {
		f.Send(m)
	}
	require.NoError(t, f.Flush())
}

// untilCopyIn reads up to CopyInResponse; no ReadyForQuery comes before it.
func untilCopyIn(t *testing.T, f *pgproto3.Frontend) {
	t.Helper()
	for {
		msg, err := f.Receive()
		require.NoError(t, err)
		_, ready := msg.(*pgproto3.ReadyForQuery)
		require.False(t, ready, "no ReadyForQuery before CopyInResponse")
		if _, ok := msg.(*pgproto3.CopyInResponse); ok {
			return
		}
	}
}

// logLine is the kind, text, rows and SQLSTATE of a querylog line.
type logLine struct {
	kind, text string
	rows       int64
	code       string
}

func lines(es []dbproxy.Entry) []logLine {
	out := make([]logLine, 0, len(es))
	for _, e := range es {
		code, _, _ := strings.Cut(e.Error, ":")
		out = append(out, logLine{e.Kind, e.Text, e.Rows, code})
	}
	return out
}

// The database ignores a Sync it reads in COPY IN mode: it gets no
// ReadyForQuery and no line, and later lines keep their own replies.
func TestSyncInCopyIn(t *testing.T) {
	p, f := realRaw(t)
	copyIn := []pgproto3.FrontendMessage{&pgproto3.Parse{Query: "copy c from stdin"}, &pgproto3.Bind{}, &pgproto3.Execute{}, &pgproto3.Sync{}}

	send(t, f, copyIn...) // the Sync right after Execute is read in COPY IN mode
	untilCopyIn(t, f)
	send(t, f, &pgproto3.CopyData{Data: []byte("1\n2\n")}, &pgproto3.CopyDone{}, &pgproto3.Sync{})
	untilReady(t, f)

	send(t, f, &pgproto3.Query{String: "copy c from stdin"})
	untilCopyIn(t, f)
	send(t, f, &pgproto3.Sync{}, &pgproto3.CopyData{Data: []byte("3\n")}, &pgproto3.Sync{}, &pgproto3.CopyDone{})
	untilReady(t, f)

	send(t, f, copyIn...)
	untilCopyIn(t, f)
	send(t, f, &pgproto3.CopyData{Data: []byte("x\n")}, &pgproto3.CopyDone{}, &pgproto3.Sync{}) // a bad row
	untilReady(t, f)

	send(t, f, copyIn...)
	untilCopyIn(t, f)
	send(t, f, &pgproto3.CopyFail{Message: "stop"}, &pgproto3.Query{String: "select 6"}, &pgproto3.Sync{})
	untilReady(t, f) // the Query before the Sync is skipped

	send(t, f, &pgproto3.Query{String: "select n from c"})
	assert.Equal(t, 3, untilReady(t, f)["D"])
	terminate(t, f)
	assert.Equal(t, []logLine{
		{"query", "create table c (n int)", 0, ""},
		{"execute", "copy c from stdin", 2, ""},
		{"query", "copy c from stdin", 1, ""},
		{"execute", "copy c from stdin", 0, "22P02"},
		{"execute", "copy c from stdin", 0, "57014"},
		{"query", "select n from c", 3, ""},
	}, lines(p.entries(t)))
}

// After an extended-protocol error the database skips every message up to
// the next Sync: a Query in between never runs and gets no line.
func TestQueryAfterExtendedError(t *testing.T) {
	p, f := realRaw(t)
	send(t, f, &pgproto3.Parse{Query: "selec 1"}, &pgproto3.Query{String: "select 2"}, &pgproto3.Sync{})
	seen := untilReady(t, f)
	assert.Zero(t, seen["D"], "the skipped Query returned no row")
	send(t, f, &pgproto3.Query{String: "select 3"})
	assert.Equal(t, 1, untilReady(t, f)["D"])
	terminate(t, f)
	assert.Equal(t, []logLine{
		{"query", "create table c (n int)", 0, ""},
		{"parse", "selec 1", 0, "42601"},
		{"query", "select 3", 1, ""},
	}, lines(p.entries(t)))
}

// An error in the middle of a pipeline ends its statements: the ones before
// keep their rows, the failed one its error, the skipped ones get no line.
func TestPipelineErrorBeforeSync(t *testing.T) {
	p, f := realRaw(t)
	stmt := func(sql string, arg string) []pgproto3.FrontendMessage {
		return []pgproto3.FrontendMessage{&pgproto3.Parse{Query: sql}, &pgproto3.Bind{Parameters: [][]byte{[]byte(arg)}}, &pgproto3.Execute{}}
	}
	var msgs []pgproto3.FrontendMessage
	for _, arg := range []string{"1", "2", "3"} {
		msgs = append(msgs, stmt("select generate_series(1, $1::int)", arg)...)
	}
	msgs = append(msgs, stmt("select 1/$1::int", "0")...)
	msgs = append(msgs, stmt("select $1::int", "5")...)
	msgs = append(msgs, &pgproto3.Query{String: "select 6"})
	msgs = append(msgs, stmt("select $1::int", "7")...)
	send(t, f, append(msgs, &pgproto3.Sync{})...)
	assert.Equal(t, 6, untilReady(t, f)["D"], "rows of the three statements before the error")
	send(t, f, &pgproto3.Query{String: "select 8"})
	untilReady(t, f)
	terminate(t, f)
	assert.Equal(t, []logLine{
		{"query", "create table c (n int)", 0, ""},
		{"execute", "select generate_series(1, $1::int)", 1, ""},
		{"execute", "select generate_series(1, $1::int)", 2, ""},
		{"execute", "select generate_series(1, $1::int)", 3, ""},
		{"execute", "select 1/$1::int", 0, "22012"},
		{"query", "select 8", 1, ""},
	}, lines(p.entries(t)))
}

// A Sync that fails (a deferred check at the implicit commit) is an execute
// line with the error; the next statement keeps its own line.
func TestSyncErrorLogged(t *testing.T) {
	p, f := realRaw(t)
	send(t, f, &pgproto3.Query{String: "create table d (x int unique deferrable initially deferred)"})
	untilReady(t, f)
	send(t, f, &pgproto3.Parse{Query: "insert into d values (1), (1)"}, &pgproto3.Bind{}, &pgproto3.Execute{}, &pgproto3.Sync{})
	untilReady(t, f)
	send(t, f, &pgproto3.Query{String: "select 1"})
	untilReady(t, f)
	terminate(t, f)
	assert.Equal(t, []logLine{
		{"query", "create table c (n int)", 0, ""},
		{"query", "create table d (x int unique deferrable initially deferred)", 0, ""},
		{"execute", "insert into d values (1), (1)", 2, ""},
		{"execute", "", 0, "23505"},
		{"query", "select 1", 1, ""},
	}, lines(p.entries(t)))
}
