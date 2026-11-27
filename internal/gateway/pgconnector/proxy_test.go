package pgconnector

import (
	"bytes"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	postgresv1 "github.com/werbot/werbot/gen/werbot/connector/postgres/v1"
	"github.com/werbot/werbot/internal/gateway"
	"github.com/werbot/werbot/internal/gateway/dbproxy"
)

// entries waits for the end of p's session and returns its querylog lines.
func (p *proxy) entries(t testing.TB) []dbproxy.Entry {
	t.Helper()
	require.NoError(t, p.serveErr(t))
	p.rec.mu.Lock()
	defer p.rec.mu.Unlock()
	out := make([]dbproxy.Entry, 0, len(p.rec.events))
	for _, e := range p.rec.events {
		out = append(out, e.(dbproxy.Entry))
	}
	return out
}

// pgSession is a pgconn session through a new proxy of cfg.
func pgSession(t *testing.T, cfg *postgresv1.Config, secret gateway.Secret) (*pgconn.PgConn, *proxy) {
	t.Helper()
	p := serveProxy(t.Context(), t, testResource(t, cfg), secret)
	conn, err := pgconn.Connect(t.Context(), p.connectURL(cfg.GetDatabase()))
	require.NoError(t, err)
	return conn, p
}

func TestSimpleQueryLogged(t *testing.T) {
	cfg, secret := realPG(t)
	conn, p := pgSession(t, cfg, secret)
	_, err := conn.Exec(t.Context(), "select 1; select 2").ReadAll()
	require.NoError(t, err)
	require.NoError(t, conn.Close(t.Context()))
	es := p.entries(t)
	require.Len(t, es, 1)
	assert.Equal(t, "query", es[0].Kind)
	assert.Equal(t, "select 1; select 2", es[0].Text)
	assert.Equal(t, cfg.GetDatabase(), es[0].DB)
	assert.EqualValues(t, 2, es[0].Rows)
	assert.Equal(t, "allow", es[0].Decision)
	assert.Positive(t, es[0].DurationUS)
	assert.Positive(t, es[0].T)
	assert.Empty(t, es[0].Error)
}

func TestPgxExtendedLoggedOnce(t *testing.T) {
	for _, tc := range []struct {
		name   string
		record *bool
		params []string
	}{
		{"params logged when unset", nil, []string{`\x0000002a`, "x"}},
		{"record_params false", proto.Bool(false), nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg, secret := realPG(t)
			cfg.RecordParams = tc.record
			p := serveProxy(t.Context(), t, testResource(t, cfg), secret)
			conn, err := pgx.Connect(t.Context(), p.connectURL(cfg.GetDatabase()))
			require.NoError(t, err)
			var n int
			var s string
			require.NoError(t, conn.QueryRow(t.Context(), "select $1::int, $2::text", 42, "x").Scan(&n, &s))
			require.NoError(t, conn.Close(t.Context()))
			es := p.entries(t)
			require.Len(t, es, 1, "one execute line, no parse line")
			assert.Equal(t, "execute", es[0].Kind)
			assert.Equal(t, "select $1::int, $2::text", es[0].Text)
			assert.Equal(t, tc.params, es[0].Params)
			assert.EqualValues(t, 1, es[0].Rows)
		})
	}
}

func TestPreparedStatementReuse(t *testing.T) {
	cfg, secret := realPG(t)
	conn, p := pgSession(t, cfg, secret)
	_, err := conn.Prepare(t.Context(), "s1", "select $1::text", nil)
	require.NoError(t, err)
	for i := range 3 {
		_, err := conn.ExecPrepared(t.Context(), "s1", [][]byte{[]byte(strconv.Itoa(i))}, nil, nil).Close()
		require.NoError(t, err)
	}
	require.NoError(t, conn.Close(t.Context()))
	es := p.entries(t)
	require.Len(t, es, 3)
	for i, e := range es {
		assert.Equal(t, "execute", e.Kind)
		assert.Equal(t, "s1", e.Statement)
		assert.Equal(t, "select $1::text", e.Text)
		assert.Equal(t, []string{strconv.Itoa(i)}, e.Params)
	}
}

func TestParseErrorLogged(t *testing.T) {
	cfg, secret := realPG(t)
	conn, p := pgSession(t, cfg, secret)
	_, err := conn.ExecParams(t.Context(), "selec 1", nil, nil, nil, nil).Close()
	assert.Equal(t, "42601", pgErr(t, err).Code)
	require.NoError(t, conn.Close(t.Context()))
	es := p.entries(t)
	require.Len(t, es, 1, "no execute line for a statement that never ran")
	assert.Equal(t, "parse", es[0].Kind)
	assert.Equal(t, "selec 1", es[0].Text)
	assert.True(t, strings.HasPrefix(es[0].Error, "42601: "), es[0].Error)
}

func TestPipelineModeErrorMidway(t *testing.T) {
	cfg, secret := realPG(t)
	conn, p := pgSession(t, cfg, secret)
	pl := conn.StartPipeline(t.Context())
	for i := range 100 {
		sql, arg := "select $1::int", strconv.Itoa(i)
		if i == 50 {
			sql, arg = "select 1/$1::int", "0"
		}
		pl.SendQueryParams(sql, [][]byte{[]byte(arg)}, nil, nil, nil)
	}
	require.NoError(t, pl.Sync())
	ok, code := 0, ""
	for done := false; !done; {
		res, err := pl.GetResults()
		if err == nil {
			if rr, isRR := res.(*pgconn.ResultReader); isRR {
				_, err = rr.Close()
			}
		}
		_, done = res.(*pgconn.PipelineSync)
		switch {
		case err != nil:
			require.Empty(t, code, "one error")
			code = pgErr(t, err).Code
		case !done:
			ok++
		}
	}
	assert.Equal(t, 50, ok, "the database skipped the rest of the batch")
	assert.Equal(t, "22012", code)
	pl.SendQueryParams("select 1", nil, nil, nil, nil)
	require.NoError(t, pl.Sync())
	res, err := pl.GetResults()
	require.NoError(t, err)
	_, err = res.(*pgconn.ResultReader).Close()
	require.NoError(t, err, "the next batch after Sync works")
	require.NoError(t, pl.Close())
	require.NoError(t, conn.Close(t.Context()))

	es := p.entries(t)
	require.Len(t, es, 52, "50 executes, the failed one and the next batch")
	for i, e := range es {
		assert.Equal(t, "execute", e.Kind, "line %d", i)
		if i == 50 {
			assert.True(t, strings.HasPrefix(e.Error, "22012: "), e.Error)
			continue
		}
		assert.Empty(t, e.Error, "line %d", i)
	}
}

func TestPgxBatch(t *testing.T) {
	cfg, secret := realPG(t)
	p := serveProxy(t.Context(), t, testResource(t, cfg), secret)
	conn, err := pgx.Connect(t.Context(), p.connectURL(cfg.GetDatabase()))
	require.NoError(t, err)
	b := &pgx.Batch{}
	for i := range 10 {
		b.Queue("select $1::int", i)
	}
	br := conn.SendBatch(t.Context(), b)
	for i := range 10 {
		var n int
		require.NoError(t, br.QueryRow().Scan(&n))
		assert.Equal(t, i, n)
	}
	require.NoError(t, br.Close())
	require.NoError(t, conn.Close(t.Context()))
	es := p.entries(t)
	require.Len(t, es, 10)
	for i, e := range es {
		assert.Equal(t, "execute", e.Kind)
		assert.Equal(t, []string{`\x` + strings.Repeat("0", 7) + strconv.Itoa(i)}, e.Params, "line %d in order", i)
	}
}

func TestCopyInOut(t *testing.T) {
	cfg, secret := realPG(t)
	conn, p := pgSession(t, cfg, secret)
	_, err := conn.Exec(t.Context(), "create table t (n int, s text)").ReadAll()
	require.NoError(t, err)
	var src strings.Builder
	for i := range 10000 {
		src.WriteString(strconv.Itoa(i) + "\tcopy-payload-marker\n")
	}
	tag, err := conn.CopyFrom(t.Context(), strings.NewReader(src.String()), "copy t from stdin")
	require.NoError(t, err)
	assert.EqualValues(t, 10000, tag.RowsAffected())
	var out bytes.Buffer
	_, err = conn.CopyTo(t.Context(), &out, "copy t to stdout")
	require.NoError(t, err)
	require.NoError(t, conn.Close(t.Context()))

	es := p.entries(t)
	require.Len(t, es, 3)
	assert.Equal(t, "copy t from stdin", es[1].Text)
	assert.EqualValues(t, 10000, es[1].Rows)
	assert.EqualValues(t, src.Len(), es[1].Bytes, "COPY FROM counts the bytes sent")
	assert.Equal(t, "copy t to stdout", es[2].Text)
	assert.EqualValues(t, 10000, es[2].Rows)
	assert.EqualValues(t, out.Len(), es[2].Bytes, "COPY TO counts the bytes the client got")
	for _, e := range es {
		if strings.Contains(e.Text+strings.Join(e.Params, ""), "copy-payload-marker") {
			t.Fatal("COPY data reached the query log")
		}
	}
}

func TestLargeBindStreamed(t *testing.T) {
	cfg, secret := realPG(t)
	conn, p := pgSession(t, cfg, secret)
	_, err := conn.Exec(t.Context(), "create table b (x bytea)").ReadAll()
	require.NoError(t, err)
	_, err = conn.Prepare(t.Context(), "ins", "insert into b values ($1)", nil)
	require.NoError(t, err)
	val := bytes.Repeat([]byte{0xab}, 20<<20)
	_, err = conn.ExecPrepared(t.Context(), "ins", [][]byte{val}, []int16{1}, nil).Close()
	require.NoError(t, err)
	rows, err := conn.Exec(t.Context(), "select length(x) from b").ReadAll()
	require.NoError(t, err)
	assert.Equal(t, strconv.Itoa(len(val)), string(rows[0].Rows[0][0]))
	require.NoError(t, conn.Close(t.Context()))

	es := p.entries(t)
	require.Len(t, es, 3)
	assert.Equal(t, "execute", es[1].Kind)
	assert.Equal(t, "ins", es[1].Statement)
	assert.Equal(t, "insert into b values ($1)", es[1].Text)
	assert.Equal(t, []string{dbproxy.Truncated}, es[1].Params, "a streamed Bind's parameters are past the bound: logged cut")
	assert.EqualValues(t, 1, es[1].Rows)
}

func TestQuerylogTruncated(t *testing.T) {
	cfg, secret := realPG(t)
	conn, p := pgSession(t, cfg, secret)
	_, err := conn.Exec(t.Context(), "select '"+strings.Repeat("é", 50<<10)+"'").ReadAll()
	require.NoError(t, err)
	_, err = conn.ExecParams(t.Context(), "select $1::text", [][]byte{bytes.Repeat([]byte("p"), 70<<10)}, nil, nil, nil).Close()
	require.NoError(t, err)
	require.NoError(t, conn.Close(t.Context()))

	es := p.entries(t)
	require.Len(t, es, 2)
	const suffix = "…[truncated]"
	assert.True(t, strings.HasSuffix(es[0].Text, suffix))
	assert.LessOrEqual(t, len(es[0].Text), dbproxy.MaxLogText+len(suffix))
	assert.True(t, utf8.ValidString(es[0].Text), "cut on a UTF-8 boundary")
	assert.Equal(t, []string{suffix}, es[1].Params)
}
