package pgconnector

import (
	"bytes"
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/gateway/mask"
)

// token has the shape of a GitHub token: masked by the patterns.
var token = "ghp_" + strings.Repeat("A1b2", 9)

// cells are the values of column v of table s, ordered by v.
func cells(t *testing.T, conn *pgconn.PgConn) []string {
	t.Helper()
	rs, err := conn.Exec(t.Context(), "select v from s order by v").ReadAll()
	require.NoError(t, err)
	out := make([]string, 0, len(rs[0].Rows))
	for _, r := range rs[0].Rows {
		out = append(out, string(r[0]))
	}
	return out
}

// copied is the COPY TO STDOUT text of table s.
func copied(t *testing.T, conn *pgconn.PgConn) string {
	t.Helper()
	var out bytes.Buffer
	_, err := conn.CopyTo(t.Context(), &out, "copy (select v from s order by v) to stdout")
	require.NoError(t, err)
	return out.String()
}

// The cells of an agent's rows are masked as the pipeline asks (mask_output
// "agents": the session credential and credential-shaped text); a person's
// pass as they are; the querylog counts the original bytes and holds no secret.
func TestMaskDataRowForAgent(t *testing.T) {
	warm(t)
	cfg, ro, rw, owner := agentDB(t)
	_, err := owner.Exec(t.Context(), "create table s (v text); insert into s values ('a "+string(ro.Value)+"'), ('b "+token+"'), ('c plain'), (null);"+
		"grant select on s to "+cfg.GetAgentUsername()+", "+cfg.GetUsername()).ReadAll()
	require.NoError(t, err)
	h, _ := newHooks(agentLimits())
	secrets := [][]byte{ro.Value}
	h.Mask = func(b []byte) []byte { return mask.Bytes(b, secrets, true) } // as the pipeline sets it
	p := serveProxy(t.Context(), t, testResource(t, cfg), ro, withHooks(h))
	agent := clientOf(t, p, cfg.GetDatabase(), nil, nil)
	if !slices.Equal(cells(t, agent), []string{"a [masked]", "b [masked]", "c plain", ""}) {
		t.Fatal("an agent's cells are not masked") // never print the cells: they may hold the secret
	}
	if copied(t, agent) != "a [masked]\nb [masked]\nc plain\n\\N\n" {
		t.Fatal("an agent's COPY TO STDOUT is not masked")
	}
	require.NoError(t, agent.Close(t.Context()))

	hp, _ := newHooks(nil) // a person: Mask nil
	pp := serveProxy(t.Context(), t, testResource(t, cfg), rw, withHooks(hp))
	person := clientOf(t, pp, cfg.GetDatabase(), nil, nil)
	if !slices.Equal(cells(t, person), []string{"a " + string(ro.Value), "b " + token, "c plain", ""}) {
		t.Fatal("a person's cells are masked")
	}
	require.NoError(t, person.Close(t.Context()))

	agentLines, personLines := p.entries(t), pp.entries(t)
	for _, e := range agentLines {
		b, err := json.Marshal(e)
		require.NoError(t, err)
		if bytes.Contains(b, ro.Value) || bytes.Contains(b, []byte(token)) {
			t.Fatal("the querylog holds a masked value")
		}
	}
	require.Len(t, agentLines, 2)
	require.Len(t, personLines, 1)
	assert.Equal(t, personLines[0].Bytes, agentLines[0].Bytes, "the querylog counts the bytes the database sent")
	assert.Equal(t, personLines[0].Rows, agentLines[0].Rows)
}

// The rows of an agent's approved write obey the agent's limits and masking
// as the session's own rows do; the write's own end passes.
func TestApprovedWriteLimitedAndMasked(t *testing.T) {
	warm(t)
	cfg, ro, rw, owner := agentDB(t)
	_, err := owner.Exec(t.Context(), "insert into t select g, 1 from generate_series(2, 50) g").ReadAll()
	require.NoError(t, err)
	h, rec := newHooks(limits(10, 1<<20))
	rec.cred = rw
	h.Mask = maskAll
	p := serveProxy(t.Context(), t, testResource(t, cfg), ro, withHooks(h))
	var notices []string
	conn := clientOf(t, p, cfg.GetDatabase(), &notices, nil)
	rs, err := conn.Exec(t.Context(), "UPDATE t SET a = 2 WHERE id > 0 RETURNING '"+token+"' AS v").ReadAll()
	require.NoError(t, err)
	if len(rs[0].Rows) != 10 {
		t.Fatal("an approved write's rows are not limited") // never print the cells
	}
	for _, r := range rs[0].Rows {
		if string(r[0]) != "[masked]" {
			t.Fatal("a cell of an approved write is not masked")
		}
	}
	assert.Equal(t, "UPDATE 50", rs[0].CommandTag.String(), "the write's own end passes")
	assert.Contains(t, notices, "werbot: result truncated at 10 rows")
	assert.Equal(t, "2", value(t, owner, "select a from t where id = 50"))
}
