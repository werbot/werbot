package dbproxy_test

import (
	"strconv"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/gateway"
	"github.com/werbot/werbot/internal/gateway/dbproxy"
	"github.com/werbot/werbot/internal/platform/shellclass"
	"github.com/werbot/werbot/internal/platform/sqlclass"
)

var rules = gateway.QueryProfile{Rules: true}

func TestClassCacheSharedAndBounded(t *testing.T) {
	e := newEnv(rules)
	g1, g2 := e.guard(t), e.guard(t)
	check(t, g1, query("select 1"))
	check(t, g2, query("select 1"))
	assert.Equal(t, int64(1), e.dialect.calls.Load(), "two sessions of one type share the class cache")
	assert.Contains(t, scrape(e.reg), `werbot_gateway_query_cache_hits_total{type="`+e.typ+`"} 1`)

	for i := range 4096 {
		check(t, g1, query("select "+strconv.Itoa(i+2)))
	}
	n := e.dialect.calls.Load()
	check(t, g1, query("select 1"))
	assert.Equal(t, n+1, e.dialect.calls.Load(), "the 4097th text evicts the oldest")
	check(t, g1, query("select 4097"))
	assert.Equal(t, n+1, e.dialect.calls.Load(), "a recent text stays")
}

func TestSecretTablesAfterCache(t *testing.T) {
	e := newEnv(rules)
	e.dialect.res["select * from billing.cards"] = result(cRead, []string{"billing.cards"}, nil)
	plain := e.guard(t)
	e.secrets = []string{"billing.*"}
	secret := e.guard(t)
	assert.Equal(t, []string{"read", "secrets"}, check(t, secret, query("select * from billing.cards")).Classes)
	assert.Equal(t, []string{"read"}, check(t, plain, query("select * from billing.cards")).Classes)
	assert.Equal(t, int64(1), e.dialect.calls.Load())
}

func TestTransientOpaqueNotCached(t *testing.T) {
	e := newEnv(rules)
	busy := sqlclass.Opaque()
	busy.Transient = true
	e.dialect.res["select 1"] = busy
	g := e.guard(t)
	assert.Equal(t, []string{"opaque"}, check(t, g, query("select 1")).Classes)
	assert.Equal(t, []string{"opaque"}, check(t, g, query("select 1")).Classes)
	assert.Len(t, e.pdp.decisions(), 2, "a transient opaque decision is not cached")
	delete(e.dialect.res, "select 1")
	assert.Equal(t, []string{"read"}, check(t, g, query("select 1")).Classes, "a transient opaque is classified again")
	assert.Equal(t, int64(3), e.dialect.calls.Load())
	check(t, g, query("select 1"))
	assert.Equal(t, int64(3), e.dialect.calls.Load(), "a deterministic result is cached")
}

func TestPreparedStatementClassifiedOnce(t *testing.T) {
	e := newEnv(rules)
	g := e.guard(t)
	check(t, g, dbproxy.Op{Kind: "parse", Text: "select $1", Statement: "s1", DB: "app"})
	for i := range 100 {
		v := check(t, g, dbproxy.Op{Kind: "execute", Text: "select $1", Statement: "s1", DB: "app", Params: []string{strconv.Itoa(i)}})
		require.True(t, v.Allow)
	}
	assert.Equal(t, int64(1), e.dialect.calls.Load())
	assert.Len(t, e.pdp.decisions(), 1, "the decision cache answers the executes")

	v := check(t, g, dbproxy.Op{Kind: "execute", Text: "", Statement: "nope", DB: "app"})
	assert.Equal(t, []string{"opaque"}, v.Classes, "an unknown statement")
	v = check(t, g, dbproxy.Op{Kind: "execute", Text: "drop table t", Statement: "s1", DB: "app"})
	assert.Equal(t, []string{"read"}, v.Classes, "an execute is judged by its parsed statement, not by the text it brings")
	v = check(t, g, dbproxy.Op{Kind: "command", Text: "fastpath function 42", DB: "app"})
	assert.Equal(t, []string{"opaque"}, v.Classes, "a function call")
}

func TestExecuteTextMustMatchStatement(t *testing.T) {
	e := newEnv(gateway.QueryProfile{})
	g := e.guard(t)
	check(t, g, dbproxy.Op{Kind: "parse", Text: "select 1", Statement: "s1"}) // the fast path keeps the statement unclassified
	e.pdp.profile = rules
	e.pdp.set(nil)
	v := check(t, g, dbproxy.Op{Kind: "execute", Text: "drop table t", Statement: "s1"})
	assert.Equal(t, []string{"opaque"}, v.Classes, "an execute text that is not the parsed one")
	assert.Equal(t, []string{"read"}, check(t, g, dbproxy.Op{Kind: "execute", Text: "select 1", Statement: "s1"}).Classes)
}

func TestFastPathHuman(t *testing.T) {
	e := newEnv(gateway.QueryProfile{})
	e.dialect.res["delete from t"] = result([]shellclass.Class{shellclass.Destructive, shellclass.Write}, []string{"public.t"}, []string{"public.t"})
	g := e.guard(t)
	v := check(t, g, query("delete from t"))
	assert.Equal(t, dbproxy.Verdict{Allow: true}, v)
	assert.Zero(t, e.dialect.calls.Load(), "no classification on the fast path")
	assert.Empty(t, e.pdp.decisions())

	g.Log(dbproxy.Entry{Kind: "query", Text: "delete from t", Decision: "allow"})
	g.Log(dbproxy.Entry{Kind: "cancel"})
	g.Close()
	require.Len(t, e.sink.entries, 2)
	assert.Equal(t, []string{"destructive", "write"}, e.sink.entries[0].Classes, "the querylog writer classifies")
	assert.Equal(t, []string{"public.t"}, e.sink.entries[0].Tables)
	assert.Nil(t, e.sink.entries[1].Classes)
}

func TestFastPathDisabledByCustomPolicy(t *testing.T) {
	e := newEnv(gateway.QueryProfile{Rules: true, TextDependent: true})
	g := e.guard(t)
	check(t, g, query("select 1"))
	check(t, g, query("select 2"))
	check(t, g, query("select 1"))
	assert.Len(t, e.pdp.decisions(), 3, "custom Cedar: no decision cache")
	assert.Equal(t, "select 2", e.pdp.decisions()[1].Command)
}

func TestAgentAlwaysSynchronous(t *testing.T) {
	e := newEnv(gateway.QueryProfile{})
	e.limits = &gateway.AgentLimits{MaxQueries: 10}
	g := e.guard(t)
	v := check(t, g, query("select 1"))
	assert.Equal(t, []string{"read"}, v.Classes)
	assert.Equal(t, int64(1), e.dialect.calls.Load())
	require.Len(t, e.pdp.decisions(), 1)
	r := e.pdp.decisions()[0]
	assert.Equal(t, gateway.AccessRequest{Action: e.typ + ".query", Kind: gateway.KindStream, Command: "select 1",
		Classes: []string{"read"}, DBName: "app"}, r)
}

func TestDecisionCacheResetOnSnapshot(t *testing.T) {
	e := newEnv(rules)
	g := e.guard(t)
	assert.True(t, check(t, g, query("select 1")).Allow)
	assert.True(t, check(t, g, query("select 1")).Allow)
	assert.Len(t, e.pdp.decisions(), 1)
	e.pdp.set(denyClasses("", "", "read"))
	assert.False(t, check(t, g, query("select 1")).Allow, "a new snapshot clears the decision cache")
	assert.Equal(t, 2, e.pdp.profiles, "and recomputes the profile")
}

func TestDecisionCacheExpires(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		e := newEnv(rules)
		g := e.guard(t)
		check(t, g, query("select 1"))
		time.Sleep(59 * time.Second)
		check(t, g, query("select 1"))
		assert.Len(t, e.pdp.decisions(), 1)
		time.Sleep(2 * time.Second)
		check(t, g, query("select 1"))
		assert.Len(t, e.pdp.decisions(), 2, "an entry lives 60 s")
	})
}

func TestQueryLimit(t *testing.T) {
	e := newEnv(rules)
	e.limits = &gateway.AgentLimits{MaxQueries: 3}
	g := e.guard(t)
	check(t, g, dbproxy.Op{Kind: "parse", Text: "select 1", Statement: "s1"})
	for range 2 {
		require.True(t, check(t, g, query("select 1")).Allow)
	}
	require.True(t, check(t, g, dbproxy.Op{Kind: "execute", Text: "select 1", Statement: "s1"}).Allow)
	v := check(t, g, query("select 1"))
	assert.Equal(t, dbproxy.Verdict{Message: "query limit 3 reached"}, v, "parse is not counted, execute is")

	e.limits = &gateway.AgentLimits{MaxQueries: 2}
	g = e.guard(t)
	for range 2 {
		require.True(t, check(t, g, dbproxy.Op{Kind: "request", Text: "GET /_search"}).Allow)
	}
	assert.Equal(t, "query limit 2 reached", check(t, g, dbproxy.Op{Kind: "request", Text: "GET /_search"}).Message)

	e.limits = nil
	g = e.guard(t)
	for range 5 {
		require.True(t, check(t, g, query("select 1")).Allow, "a person has no query limit")
	}
}

func TestDenialMessageDefault(t *testing.T) {
	e := newEnv(rules)
	e.dialect.res["drop table t"] = result([]shellclass.Class{shellclass.Destructive, shellclass.Write}, []string{"public.t"}, []string{"public.t"})
	e.pdp.rule = denyClasses("", "", "destructive")
	g := e.guard(t)
	v := check(t, g, query("drop table t"))
	assert.Equal(t, dbproxy.Verdict{Message: "not allowed (destructive, write)", Classes: []string{"destructive", "write"},
		Tables: []string{"public.t"}, WriteTables: []string{"public.t"}, PolicyIDs: []string{"command:r1"}}, v)
	require.Len(t, e.denials, 1)
	assert.Equal(t, gateway.ApprovalInput{Command: "drop table t", Classes: []string{"destructive", "write"}, Reasons: []string{"command:r1"}}, e.denials[0])

	e.pdp.set(denyClasses("No drops", "", "destructive"))
	assert.Equal(t, "No drops", check(t, g, query("drop table t")).Message)
	assert.Len(t, e.denials, 2, "every denied operation is reported, a cached one too")
	check(t, g, query("drop table t"))
	assert.Len(t, e.denials, 3)
}

func TestApprovalObligationVerdict(t *testing.T) {
	e := newEnv(rules)
	e.dialect.res["update t set a = 1"] = result(cWrite, []string{"public.t"}, []string{"public.t"})
	e.pdp.rule = denyClasses("ask ops", "approval", "write")
	g := e.guard(t)
	v := check(t, g, query("update t set a = 1"))
	assert.Equal(t, dbproxy.Verdict{Obligations: []string{"approval"}, Message: "ask ops", Classes: []string{"write"},
		Tables: []string{"public.t"}, WriteTables: []string{"public.t"}, PolicyIDs: []string{"command:r1"}}, v)
	assert.Empty(t, e.denials, "an obligation is not a denial yet")
	e.pdp.set(denyClasses("", "mfa", "write"))
	v = check(t, g, query("update t set a = 1"))
	assert.Nil(t, v.Obligations, "only approval is met inside a session")
	assert.Equal(t, "not allowed (write)", v.Message)
}

func TestParserBrokenGauge(t *testing.T) {
	e := newEnv(rules)
	var broken bool
	g := dbproxy.NewGuard(&gateway.Session{Recording: e.sink}, e.hooks(),
		dbproxy.GuardConfig{Type: e.typ, Dialect: e.dialect, Broken: func() bool { return broken }, Metrics: e.reg})
	defer g.Close()
	assert.Contains(t, scrape(e.reg), `werbot_gateway_sql_parser_broken{type="`+e.typ+`"} 0`)
	broken = true
	check(t, g, query("select 1"))
	assert.Contains(t, scrape(e.reg), `werbot_gateway_sql_parser_broken{type="`+e.typ+`"} 1`)
}

func TestCheckCachedNoAlloc(t *testing.T) {
	e := newEnv(rules)
	g := e.guard(t)
	ctx, op := t.Context(), query("select id, name from users where id = 42") // longer than a stack buffer
	check(t, g, op)
	assert.Zero(t, testing.AllocsPerRun(1000, func() { _, _ = g.Check(ctx, op) }), "a repeated query")
	check(t, g, dbproxy.Op{Kind: "parse", Text: "select $1", Statement: "s1"})
	exec := dbproxy.Op{Kind: "execute", Text: "select $1", Statement: "s1"}
	check(t, g, exec)
	assert.Zero(t, testing.AllocsPerRun(1000, func() { _, _ = g.Check(ctx, exec) }), "a prepared execute")
}

func TestSQLDialect(t *testing.T) {
	r := dbproxy.SQLDialect("nope").Classify("select 1", sqlclass.Options{})
	assert.Equal(t, []shellclass.Class{shellclass.Opaque}, r.Classes)
	assert.True(t, r.Transient, "a dialect without a parser")
}
