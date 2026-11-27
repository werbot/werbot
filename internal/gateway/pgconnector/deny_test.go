package pgconnector

import (
	"net"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgproto3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	postgresv1 "github.com/werbot/werbot/gen/werbot/connector/postgres/v1"
	"github.com/werbot/werbot/internal/gateway"
	"github.com/werbot/werbot/internal/platform/shellclass"
	"github.com/werbot/werbot/internal/platform/sqlclass"
)

// received is a fake database that answers like answerExtended and records
// every client message's type and, for a query or parse, its text.
type received struct {
	mu   sync.Mutex
	msgs []string
}

func (r *received) add(s string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.msgs = append(r.msgs, s)
}

func (r *received) all() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.msgs...)
}

// answerExtended answers queries and extended messages with one row "1";
// a Parse of "select fail" fails, and the messages after it up to Sync are
// skipped. Replies to a successful Parse wait for the next flush.
func (r *received) answerExtended(b *pgproto3.Backend) {
	row := func() {
		b.Send(&pgproto3.DataRow{Values: [][]byte{[]byte("1")}})
		b.Send(&pgproto3.CommandComplete{CommandTag: []byte("SELECT 1")})
	}
	failed := false // after an error the database skips to Sync
	fakeLoop(b, func(msg pgproto3.FrontendMessage) {
		if _, sync := msg.(*pgproto3.Sync); failed && !sync {
			return
		}
		switch m := msg.(type) {
		case *pgproto3.Parse:
			r.add("P " + m.Query)
			if failed = m.Query == "select fail"; failed {
				b.Send(&pgproto3.ErrorResponse{Severity: "ERROR", Code: "XX000", Message: "boom"})
			} else {
				b.Send(&pgproto3.ParseComplete{})
			}
		case *pgproto3.Query:
			r.add("Q " + m.String)
			b.Send(&pgproto3.RowDescription{Fields: []pgproto3.FieldDescription{{Name: []byte("x"), DataTypeOID: 25, DataTypeSize: -1, TypeModifier: -1}}})
			row()
			b.Send(&pgproto3.ReadyForQuery{TxStatus: 'I'})
		case *pgproto3.Bind:
			r.add("B")
			b.Send(&pgproto3.BindComplete{})
		case *pgproto3.Execute:
			r.add("E")
			row()
		case *pgproto3.Sync:
			r.add("S")
			failed = false
			b.Send(&pgproto3.ReadyForQuery{TxStatus: 'I'})
		case *pgproto3.Flush:
			r.add("H")
		default:
			return
		}
		if _, ok := msg.(*pgproto3.Parse); !ok || failed { // replies to extended messages wait for a Flush or Sync
			_ = b.Flush()
		}
	})
}

func TestDenialInReplyOrder(t *testing.T) {
	warm(t)
	h, _ := newHooks(nil)
	db := &received{}
	p := fakeProxy(t.Context(), t, db.answerExtended, withHooks(h))
	_, f := rawClient(t, p)
	send(t, f, &pgproto3.Query{String: "select 1"}, &pgproto3.Query{String: "drop table t"}, &pgproto3.Query{String: "select 2"})
	assert.Equal(t, []string{
		"T", "D 1", "C SELECT 1", "Z I",
		"E 42501 werbot: query denied: not allowed (destructive)", "Z I",
		"T", "D 1", "C SELECT 1", "Z I",
	}, trace(t, f, 3), "the denial answers in place of the database, in order; the session lives")
	terminate(t, f)
	assert.Equal(t, []string{"Q select 1", "Q select 2"}, db.all(), "the denied query never reached the database")
	ls := lines(p.entries(t))
	require.Len(t, ls, 3)
	assert.Equal(t, logLine{kind: "query", text: "drop table t", code: "42501"}, ls[1])
}

func TestDeniedExtendedSkipsToSync(t *testing.T) {
	warm(t)
	h, _ := newHooks(nil)
	db := &received{}
	p := fakeProxy(t.Context(), t, db.answerExtended, withHooks(h))
	conn, f := rawClient(t, p)
	require.NoError(t, conn.SetReadDeadline(time.Now().Add(10*time.Second)))
	send(t, f,
		&pgproto3.Parse{Query: "select 1"}, &pgproto3.Bind{}, &pgproto3.Execute{},
		&pgproto3.Parse{Name: "d", Query: "drop table t"}, &pgproto3.Bind{PreparedStatement: "d"}, &pgproto3.Execute{},
		&pgproto3.Query{String: "select 3"}, // inside the batch: dropped like the rest up to Sync
		&pgproto3.Sync{},
		&pgproto3.Query{String: "select 2"})
	assert.Equal(t, []string{
		"1", "2", "D 1", "C SELECT 1",
		"E 42501 werbot: query denied: not allowed (destructive)", "Z I",
		"T", "D 1", "C SELECT 1", "Z I",
	}, trace(t, f, 2), "the error precedes the ReadyForQuery of the batch's Sync")
	send(t, f, &pgproto3.Parse{Query: "select 1"}, &pgproto3.Parse{Name: "d", Query: "drop table t"}, &pgproto3.Bind{}, &pgproto3.Flush{})
	for _, want := range []string{"1", "E 42501 werbot: query denied: not allowed (destructive)"} {
		msg, err := f.Receive()
		require.NoError(t, err)
		assert.Equal(t, want, traceOf(msg), "a Flush after the denial gets the replies up to it, and the denial")
	}
	send(t, f, &pgproto3.Sync{})
	assert.Equal(t, []string{"Z I"}, trace(t, f, 1))
	send(t, f,
		&pgproto3.Parse{Query: "select fail"}, &pgproto3.Bind{}, &pgproto3.Execute{},
		&pgproto3.Parse{Name: "d", Query: "drop table t"}, &pgproto3.Sync{},
		&pgproto3.Query{String: "select 2"})
	assert.Equal(t, []string{"E XX000 boom", "Z I", "T", "D 1", "C SELECT 1", "Z I"}, trace(t, f, 2),
		"after the database's error the batch's denial is not sent")
	send(t, f,
		&pgproto3.Parse{Query: "select fail"}, &pgproto3.Bind{}, &pgproto3.Execute{},
		&pgproto3.Query{String: "drop table t"}, &pgproto3.Sync{},
		&pgproto3.Query{String: "drop table t"}, &pgproto3.Query{String: "select 2"})
	assert.Equal(t, []string{"E XX000 boom", "Z I",
		"E 42501 werbot: query denied: not allowed (destructive)", "Z I",
		"T", "D 1", "C SELECT 1", "Z I"}, trace(t, f, 3),
		"a simple query's denial inside the failed batch is not sent; one after its Sync is")
	terminate(t, f)
	assert.Equal(t, []string{"P select 1", "B", "E", "S", "Q select 2", "P select 1", "H", "S", "P select fail", "S", "Q select 2",
		"P select fail", "S", "Q select 2"}, db.all(),
		"messages after the denial up to Sync never reached the database")
}

func TestUnsafeLexingSessionIsOpaque(t *testing.T) {
	warm(t)
	for name, enc := range map[string]string{"standard_conforming_strings off": "UTF8", "client encoding SJIS": "SJIS"} {
		t.Run(name, func(t *testing.T) {
			starts := make(chan map[string]string, 1)
			db := newFakeDB(t, nil, func(b *pgproto3.Backend, _ net.Conn, sm *pgproto3.StartupMessage) {
				starts <- sm.Parameters
				b.Send(&pgproto3.AuthenticationOk{})
				b.Send(&pgproto3.ParameterStatus{Name: "client_encoding", Value: enc})
				b.Send(&pgproto3.ParameterStatus{Name: "standard_conforming_strings", Value: "on"})
				b.Send(&pgproto3.BackendKeyData{ProcessID: fakePID, SecretKey: fakeSecret})
				b.Send(&pgproto3.ReadyForQuery{TxStatus: 'I'})
				if b.Flush() != nil {
					return
				}
				fakeLoop(b, func(msg pgproto3.FrontendMessage) {
					if q, ok := msg.(*pgproto3.Query); ok && q.String == "set standard_conforming_strings = off" {
						b.Send(&pgproto3.ParameterStatus{Name: "standard_conforming_strings", Value: "off"})
					}
					b.Send(&pgproto3.CommandComplete{CommandTag: []byte("OK")})
					b.Send(&pgproto3.ReadyForQuery{TxStatus: 'I'})
					_ = b.Flush()
				})
			})
			h, rec := newHooks(nil)
			rec.pdp.rule = func(gateway.AccessRequest) gateway.Decision { return allow }
			cfg := &postgresv1.Config{Host: "127.0.0.1", Port: db.port, Database: "main", Username: "app", Sslmode: "disable"}
			p := serveProxy(t.Context(), t, testResource(t, cfg), passwordSecret(marker), withHooks(h))
			f := rawStart(t, p, map[string]string{"user": "nobody", "database": "main", "standard_conforming_strings": "off", "backslash_quote": "on"})
			start := <-starts
			assert.Equal(t, "on", start["standard_conforming_strings"], "the gateway asks for the lexing the classifier uses")
			assert.NotContains(t, start, "backslash_quote")
			for _, q := range []string{"select 1", "set standard_conforming_strings = off", "select 1"} {
				send(t, f, &pgproto3.Query{String: q})
				trace(t, f, 1)
			}
			terminate(t, f)
			want := [][]string{{"read"}, {"privilege"}, {"opaque"}}
			if enc == "SJIS" {
				want = [][]string{{"opaque"}, {"opaque"}, {"opaque"}}
			}
			var got [][]string
			for _, e := range p.entries(t) {
				got = append(got, e.Classes)
			}
			assert.Equal(t, want, got)
		})
	}
}

// rawStart is a protocol-level client of p that starts with params.
func rawStart(t *testing.T, p *proxy, params map[string]string) *pgproto3.Frontend {
	t.Helper()
	conn, err := net.Dial("tcp", p.addr)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	require.NoError(t, conn.SetDeadline(time.Now().Add(10*time.Second)))
	f := pgproto3.NewFrontend(conn, conn)
	send(t, f, &pgproto3.StartupMessage{ProtocolVersion: pgproto3.ProtocolVersion30, Parameters: params})
	untilReady(t, f)
	return f
}

func TestAgentParserNotReadyRefused(t *testing.T) {
	useDialect(t, dialect(sqlclass.Result{Classes: []shellclass.Class{shellclass.Opaque}, Transient: true}))
	h, rec := newHooks(agentLimits())
	db := &received{}
	p := fakeProxy(t.Context(), t, db.answerExtended, withHooks(h))
	_, f := rawClient(t, p)
	send(t, f, &pgproto3.Query{String: "select '" + uuid.NewString() + "'"}) // a text the shared class cache does not hold
	assert.Equal(t, []string{"E 42501 werbot: query denied: not allowed (opaque)", "Z I"}, trace(t, f, 1))
	terminate(t, f)
	assert.Empty(t, db.all(), "nothing reached the database")
	assert.Equal(t, [][]string{{"opaque"}}, rec.pdp.decided())
}

func TestAgentNotPipelined(t *testing.T) {
	for _, agent := range []bool{false, true} {
		useDialect(t, dialect(sqlclass.Result{Classes: []shellclass.Class{shellclass.Read}, Statements: 1}))
		h, _ := newHooks(nil)
		if agent {
			h.Limits = agentLimits()
		}
		got := make(chan string, 8)
		p := fakeProxy(t.Context(), t, func(b *pgproto3.Backend) {
			queries := make(chan string, 8)
			go func() { // reads on while the first query runs
				defer close(queries)
				fakeLoop(b, func(msg pgproto3.FrontendMessage) {
					if q, ok := msg.(*pgproto3.Query); ok {
						got <- q.String
						queries <- q.String
					}
				})
			}()
			for q := range queries {
				if q == "select 1" {
					time.Sleep(200 * time.Millisecond) // a second query sent meanwhile is here by now
					got <- "ready 1"
				}
				b.Send(&pgproto3.CommandComplete{CommandTag: []byte("SELECT 0")})
				b.Send(&pgproto3.ReadyForQuery{TxStatus: 'I'})
				_ = b.Flush()
			}
		}, withHooks(h))
		_, f := rawClient(t, p)
		send(t, f, &pgproto3.Query{String: "select 1"}, &pgproto3.Query{String: "select 2"})
		trace(t, f, 2)
		terminate(t, f)
		order := []string{<-got, <-got, <-got}
		if agent {
			assert.Equal(t, []string{"select 1", "ready 1", "select 2"}, order, "an agent's next query waits for the previous ReadyForQuery")
		} else {
			assert.Equal(t, []string{"select 1", "select 2", "ready 1"}, order, "a person's queries are pipelined")
		}
	}
}
