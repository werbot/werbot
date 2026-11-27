package dbproxy_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/gateway/dbproxy"
)

func noNotice(string, string, time.Duration) error { return nil }

// A Parse to a non-empty name the Guard keeps never replaces it: the
// database refuses it (42P05) and keeps the first statement. The unnamed
// statement is replaced, as the database does.
func TestReparseNeverReplacesNamedStatement(t *testing.T) {
	_, g := writeEnv(t)
	write := dbproxy.Op{Kind: "parse", Text: "update t set a = $1", Statement: "s1", DB: "app"}
	_, err := approve(t, g, write, noNotice)
	require.NoError(t, err)
	assert.True(t, check(t, g, dbproxy.Op{Kind: "parse", Text: "select 1", Statement: "s1", DB: "app"}).Allow)
	exec := dbproxy.Op{Kind: "execute", Text: write.Text, Statement: "s1", DB: "app", Params: []string{"9"}}
	assert.Equal(t, []string{"approval"}, check(t, g, exec).Obligations, "s1 is still the write")

	check(t, g, dbproxy.Op{Kind: "parse", Text: write.Text, DB: "app"})
	check(t, g, dbproxy.Op{Kind: "parse", Text: "select 1", DB: "app"})
	assert.Equal(t, []string{"read"}, check(t, g, dbproxy.Op{Kind: "execute", Text: "select 1", DB: "app"}).Classes, "the unnamed statement is replaced")
}

// A forgotten statement is unknown: its execute is opaque and never approved,
// and the name is free for the next Parse.
func TestForgottenStatementUnknown(t *testing.T) {
	e, g := writeEnv(t)
	e.pdp.set(denyClasses("ask", "approval", "write", "opaque"))
	check(t, g, dbproxy.Op{Kind: "parse", Text: "select 1", Statement: "s1", DB: "app"})
	check(t, g, dbproxy.Op{Kind: "parse", Text: "", Statement: "s2", DB: "app"})
	g.Forget("s1")
	for _, op := range []dbproxy.Op{
		{Kind: "execute", Statement: "s1", DB: "app", Opaque: true},
		{Kind: "execute", Statement: "s2", DB: "app", Opaque: true}, // known, but the connector cannot tell which text runs
	} {
		v := check(t, g, op)
		assert.Equal(t, []string{"opaque"}, v.Classes)
		v, err := g.Approve(t.Context(), op, v, noNotice)
		require.NoError(t, err)
		assert.False(t, v.Allow, "an execute without its text is never approved")
	}
	assert.Empty(t, e.approved, "no approval request")

	check(t, g, dbproxy.Op{Kind: "parse", Text: "update t set a = $1", Statement: "s1", DB: "app"})
	assert.Equal(t, []string{"write"}, check(t, g, dbproxy.Op{Kind: "execute", Text: "update t set a = $1", Statement: "s1", DB: "app"}).Classes,
		"a forgotten name takes the next Parse")
}
