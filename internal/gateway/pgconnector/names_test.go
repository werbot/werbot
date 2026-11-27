package pgconnector

import (
	"slices"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgproto3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/gateway"
)

// A Parse to a live named statement does not replace it: the database
// refuses it (42P05) and keeps the first one, so an execute of the name
// with other parameters still needs its own approval.
func TestReparseKeepsApprovedStatement(t *testing.T) {
	warm(t)
	cfg, secret, owner := tableDB(t)
	h, rec := newHooks(nil)
	p := serveProxy(t.Context(), t, testResource(t, cfg), secret, withHooks(h))
	conn := clientOf(t, p, cfg.GetDatabase(), nil, nil)
	_, err := conn.Prepare(t.Context(), "s1", "UPDATE t SET a = $1 WHERE id = $2", nil)
	require.NoError(t, err)
	exec := func(a string) error {
		return conn.ExecPrepared(t.Context(), "s1", [][]byte{[]byte(a), []byte("1")}, nil, nil).Read().Err
	}
	require.NoError(t, exec("3"))
	rec.approveErr = gateway.ErrApprovalDenied
	assert.Equal(t, "werbot: query denied: approval denied", pgErr(t, exec("7")).Message)

	_, err = conn.Prepare(t.Context(), "s1", "SELECT $1::int + $2::int", nil)
	assert.Equal(t, "42P05", pgErr(t, err).Code, "the database keeps the first s1")
	assert.Equal(t, "werbot: query denied: approval denied", pgErr(t, exec("9")).Message, "s1 is still the UPDATE")
	assert.Equal(t, "3", value(t, owner, "select a from t"), "the write never ran unapproved")
	approvals, _ := rec.asked()
	assert.Len(t, approvals, 4)
}

// SQL DEALLOCATE and PREPARE that reach the database forget the named
// statements, even when the client pipelines a Bind of the name right after:
// its execute is opaque, and an opaque statement is never approved.
func TestSQLPrepareForgetsNamedStatements(t *testing.T) {
	warm(t)
	cfg, secret, owner := tableDB(t)
	h, _ := newHooks(nil)
	p := serveProxy(t.Context(), t, testResource(t, cfg), secret, withHooks(h))
	conn, f := rawLogin(t, p, cfg.GetDatabase())
	require.NoError(t, conn.SetReadDeadline(time.Now().Add(10*time.Second)))
	send(t, f, &pgproto3.Parse{Name: "s1", Query: "SELECT $1::int + $2::int"}, &pgproto3.Sync{})
	assert.Equal(t, []string{"1", "Z I"}, trace(t, f, 1))
	send(t, f, &pgproto3.Query{String: "DEALLOCATE s1"},
		&pgproto3.Query{String: "PREPARE s1 AS UPDATE t SET a = $1 WHERE id = $2"},
		&pgproto3.Bind{PreparedStatement: "s1", Parameters: [][]byte{[]byte("9"), []byte("1")}}, &pgproto3.Execute{}, &pgproto3.Sync{})
	got := slices.DeleteFunc(trace(t, f, 3), func(s string) bool { return s[0] == 'N' })
	terminate(t, f)
	assert.Equal(t, []string{"C DEALLOCATE", "Z I", "C PREPARE", "Z I", "2", "E 42501 werbot: query denied: not allowed (opaque)", "Z I"}, got)
	assert.Equal(t, "1", value(t, owner, "select a from t"), "the prepared write never ran")
}

// A name whose Parse the database refused is free again: the next Parse
// of it is the statement. An Execute of a portal the gateway never saw
// bound (a SQL cursor) is opaque, not the unnamed statement.
func TestFailedParseFreesNameAndUnknownPortalIsOpaque(t *testing.T) {
	warm(t)
	cfg, secret, _ := tableDB(t)
	h, rec := newHooks(nil)
	rec.pdp.rule = func(gateway.AccessRequest) gateway.Decision { return allow }
	p := serveProxy(t.Context(), t, testResource(t, cfg), secret, withHooks(h))
	conn, f := rawLogin(t, p, cfg.GetDatabase())
	require.NoError(t, conn.SetReadDeadline(time.Now().Add(10*time.Second)))
	send(t, f, &pgproto3.Parse{Name: "s1", Query: "SELECT a FROM nosuch"}, &pgproto3.Sync{})
	assert.Equal(t, "Z I", trace(t, f, 1)[1])
	send(t, f, &pgproto3.Parse{Name: "s1", Query: "SELECT a FROM t"}, &pgproto3.Bind{PreparedStatement: "s1"}, &pgproto3.Execute{},
		&pgproto3.Parse{Query: "SELECT 1"}, &pgproto3.Sync{},
		&pgproto3.Query{String: "DECLARE c CURSOR WITH HOLD FOR SELECT a FROM t"},
		&pgproto3.Execute{Portal: "c"}, &pgproto3.Sync{})
	assert.Equal(t, []string{"1", "2", "D 1", "C SELECT 1", "1", "Z I", "C DECLARE CURSOR", "Z I", "D 1", "C SELECT 1", "Z I"}, trace(t, f, 3))
	terminate(t, f)
	classes := map[string][]string{}
	for _, e := range p.entries(t) {
		classes[e.Kind+" "+e.Statement+" "+e.Text] = e.Classes
	}
	assert.Equal(t, []string{"read"}, classes["execute s1 SELECT a FROM t"])
	assert.Equal(t, []string{"opaque"}, classes["execute  "], "the cursor's portal is unknown")
}

// A Close the database skips (an error earlier in its batch) leaves the
// statement there: a later Parse of the name fails (42P05), so a Bind
// pipelined behind it, before the gateway reads that answer, is opaque.
func TestSkippedCloseKeepsStatement(t *testing.T) {
	warm(t)
	cfg, secret, owner := tableDB(t)
	h, _ := newHooks(nil)
	p := serveProxy(t.Context(), t, testResource(t, cfg), secret, withHooks(h))
	conn, f := rawLogin(t, p, cfg.GetDatabase())
	require.NoError(t, conn.SetReadDeadline(time.Now().Add(10*time.Second)))
	send(t, f, &pgproto3.Parse{Name: "s1", Query: "UPDATE t SET a = $1 WHERE id = $2"},
		&pgproto3.Bind{PreparedStatement: "s1", Parameters: [][]byte{[]byte("3"), []byte("1")}}, &pgproto3.Execute{}, &pgproto3.Sync{})
	assert.Contains(t, trace(t, f, 1), "C UPDATE 1", "approved with its parameters")
	send(t, f,
		&pgproto3.Bind{PreparedStatement: "s1", Parameters: [][]byte{[]byte("x"), []byte("1")}}, &pgproto3.Close{ObjectType: 'S', Name: "s1"}, &pgproto3.Sync{},
		&pgproto3.Parse{Name: "s1", Query: "SELECT $1::int + $2::int"}, &pgproto3.Sync{},
		&pgproto3.Bind{PreparedStatement: "s1", Parameters: [][]byte{[]byte("9"), []byte("1")}}, &pgproto3.Execute{}, &pgproto3.Sync{})
	got := trace(t, f, 3)
	terminate(t, f)
	assert.Equal(t, "E 42P05 prepared statement \"s1\" already exists", got[2])
	assert.Equal(t, []string{"2", "E 42501 werbot: query denied: not allowed (opaque)", "Z I"}, got[4:])
	assert.Equal(t, "3", value(t, owner, "select a from t"), "the UPDATE never ran with other parameters")
}

// A Parse the gateway denies never reaches the database: its name stays free.
func TestDeniedParseFreesName(t *testing.T) {
	warm(t)
	h, _ := newHooks(nil)
	db := &received{}
	p := fakeProxy(t.Context(), t, db.answerExtended, withHooks(h))
	conn, f := rawClient(t, p)
	require.NoError(t, conn.SetReadDeadline(time.Now().Add(10*time.Second)))
	send(t, f, &pgproto3.Parse{Name: "s1", Query: "drop table t"}, &pgproto3.Sync{},
		&pgproto3.Parse{Name: "s1", Query: "select 1"}, &pgproto3.Bind{PreparedStatement: "s1"}, &pgproto3.Execute{}, &pgproto3.Sync{})
	assert.Equal(t, []string{"E 42501 werbot: query denied: not allowed (destructive)", "Z I", "1", "2", "D 1", "C SELECT 1", "Z I"}, trace(t, f, 2))
	terminate(t, f)
}
