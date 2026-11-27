package pgconnector

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgproto3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	postgresv1 "github.com/werbot/werbot/gen/werbot/connector/postgres/v1"
	"github.com/werbot/werbot/internal/gateway"
	"github.com/werbot/werbot/internal/gateway/mask"
	"github.com/werbot/werbot/internal/platform/db/dbtest"
)

// bigSeries runs far longer than any test: a result of it that ends at
// once was canceled at the database. It streams its rows (a set-returning
// function in the select list); in FROM it would build all of them before
// the first one.
const bigSeries = "select generate_series(1, 100000000)"

// limitedAgent serves agent sessions of a fresh agentDB with limits l; ro
// is the agent session's secret.
func limitedAgent(t *testing.T, l *gateway.AgentLimits) (p *proxy, cfg *postgresv1.Config, ro gateway.Secret) {
	t.Helper()
	warm(t)
	cfg, ro, _, _ = agentDB(t)
	h, _ := newHooks(l)
	return serveProxy(t.Context(), t, testResource(t, cfg), ro, withHooks(h)), cfg, ro
}

func limits(rows int, size int64) *gateway.AgentLimits {
	l := agentLimits()
	l.MaxRows, l.MaxResultBytes = rows, size
	return l
}

func TestRowLimit(t *testing.T) {
	p, cfg, _ := limitedAgent(t, agentLimits())
	var notices []string
	conn := clientOf(t, p, cfg.GetDatabase(), &notices, nil)
	t0 := time.Now()
	rs, err := conn.Exec(t.Context(), bigSeries).ReadAll()
	require.NoError(t, err)
	assert.Less(t, time.Since(t0), 5*time.Second, "the database query was canceled")
	assert.Equal(t, 1000, len(rs[0].Rows))
	assert.Equal(t, "SELECT 1000", rs[0].CommandTag.String())
	assert.Equal(t, []string{"werbot: result truncated at 1000 rows"}, notices)
	assert.Equal(t, "1", value(t, conn, "select 1"), "the session lives")
	require.NoError(t, conn.Close(t.Context()))
	es := p.entries(t)
	require.Len(t, es, 3)
	assert.Equal(t, "cancel", es[0].Kind, "the gateway canceled the query")
	assert.Equal(t, logLine{"query", bigSeries, 1000, "57014"}, lines(es)[1])
}

// An agent's next query waits for the end of the cut one, so the cancel
// never reaches it.
func TestRowLimitCancelsOnlyCurrentQuery(t *testing.T) {
	p, cfg, _ := limitedAgent(t, agentLimits())
	_, f := rawLogin(t, p, cfg.GetDatabase())
	send(t, f, &pgproto3.Query{String: bigSeries}, &pgproto3.Query{String: "select 42"})
	got := trace(t, f, 2)
	terminate(t, f)
	require.Len(t, got, 1008)
	assert.Equal(t, []string{"N werbot: result truncated at 1000 rows", "C SELECT 1000", "Z I", "T", "D 42", "C SELECT 1", "Z I"}, got[1001:])
}

func TestByteLimitCatchesAggregatedCell(t *testing.T) {
	p, cfg, _ := limitedAgent(t, limits(1000, 1<<20))
	var notices []string
	conn := clientOf(t, p, cfg.GetDatabase(), &notices, nil)
	rs, err := conn.Exec(t.Context(), "select string_agg(repeat('x', 1024), '') from generate_series(1, 4096)").ReadAll()
	require.NoError(t, err)
	assert.Equal(t, 0, len(rs[0].Rows), "the 4 MiB cell never reaches the agent")
	assert.Equal(t, "SELECT 0", rs[0].CommandTag.String())
	assert.Equal(t, []string{"werbot: result truncated at 1048576 bytes"}, notices)
	assert.Equal(t, "1", value(t, conn, "select 1"), "the session lives")

	rs, err = conn.Exec(t.Context(), "select repeat('x', n) from unnest(array[10, 2000000, 10]) n").ReadAll()
	require.NoError(t, err)
	assert.Equal(t, 1, len(rs[0].Rows), "no row after the cut reaches the agent, small ones neither")
	assert.Equal(t, "SELECT 1", rs[0].CommandTag.String())
}

func TestCopyOutLimitedForAgent(t *testing.T) {
	for name, tc := range map[string]struct {
		limits *gateway.AgentLimits
		sql    string
		notice string
		lines  int
	}{
		"rows":  {limits(10, 1<<20), "copy (" + bigSeries + ") to stdout", "werbot: result truncated at 10 rows", 10},
		"bytes": {limits(1000, 1024), "copy (select repeat('x', 99) from (" + bigSeries + ") s) to stdout", "werbot: result truncated at 1024 bytes", 10},
	} {
		t.Run(name, func(t *testing.T) {
			p, cfg, _ := limitedAgent(t, tc.limits)
			var notices []string
			conn := clientOf(t, p, cfg.GetDatabase(), &notices, nil)
			var out bytes.Buffer
			t0 := time.Now()
			_, err := conn.CopyTo(t.Context(), &out, tc.sql)
			assert.Equal(t, "57014", pgErr(t, err).Code, "the database's cancel error passes")
			assert.Less(t, time.Since(t0), 5*time.Second)
			assert.Equal(t, tc.lines, strings.Count(out.String(), "\n"))
			assert.Equal(t, []string{tc.notice}, notices)
			assert.Equal(t, "1", value(t, conn, "select 1"), "the session lives")
		})
	}
}

func TestLimitsHumanUnaffected(t *testing.T) {
	warm(t)
	cfg, secret := realPG(t)
	var notices []string
	conn := clientOf(t, ruledProxy(t, cfg, secret), cfg.GetDatabase(), &notices, nil)
	rs, err := conn.Exec(t.Context(), "select generate_series(1, 100000)").ReadAll()
	require.NoError(t, err)
	assert.Equal(t, 100000, len(rs[0].Rows))
	assert.Equal(t, "SELECT 100000", rs[0].CommandTag.String())
	assert.Empty(t, notices)
}

// Limits that are not set (zero) let no row through: the agent's reply is
// cut at once.
func TestZeroLimitsFailClosed(t *testing.T) {
	p, cfg, _ := limitedAgent(t, limits(0, 0))
	var notices []string
	conn := clientOf(t, p, cfg.GetDatabase(), &notices, nil)
	rs, err := conn.Exec(t.Context(), "select 1").ReadAll()
	require.NoError(t, err)
	assert.Equal(t, 0, len(rs[0].Rows))
	assert.Equal(t, []string{"werbot: result truncated at 0 rows"}, notices)
}

func TestStatementTimeoutAgent(t *testing.T) {
	l := agentLimits()
	l.StatementTimeout = time.Second
	p, cfg, _ := limitedAgent(t, l)
	conn := clientOf(t, p, cfg.GetDatabase(), nil, nil)
	t0 := time.Now()
	_, err := conn.Exec(t.Context(), "select pg_sleep(5)").ReadAll()
	assert.Equal(t, "57014", pgErr(t, err).Code)
	assert.Less(t, time.Since(t0), 3*time.Second)
}

// A cancel request with an agent session's key acts only through a proxy
// of the session's own resource.
func TestAgentCancelOnlyWithinResource(t *testing.T) {
	p, cfg, ro := limitedAgent(t, agentLimits())
	h, _ := newHooks(agentLimits())
	other := serveProxy(t.Context(), t, testResource(t, cfg), ro, withHooks(h)) // another resource on the same database
	conn := clientOf(t, p, cfg.GetDatabase(), nil, nil)
	errc := sleep(t, conn, "0.5")
	sendCancel(t, other.addr, keyOf(conn))
	require.NoError(t, <-errc, "a cancel through another resource reached nothing")
	errc = sleep(t, conn, "30")
	sendCancel(t, p.addr, keyOf(conn))
	select {
	case err := <-errc:
		assert.Equal(t, "57014", pgErr(t, err).Code)
	case <-time.After(5 * time.Second):
		t.Fatal("the query was not canceled")
	}
	assert.Equal(t, int32(0), other.serves.Load())
}

// BenchmarkPostgresStreamThroughput reads rows through the connector and
// directly, interleaved. A person streams 1 000 000 rows (budget ≥ 0.9 ×
// direct, spec 13 §4.1); an agent reads one window of its limits (no
// budget: masking and counting cost per row).
func BenchmarkPostgresStreamThroughput(b *testing.B) {
	warm(b)
	cfg, secret := realPG(b)
	b.Run("human", func(b *testing.B) {
		stream(b, pair(b, cfg, secret, serveProxy(b.Context(), b, testResource(b, cfg), secret)), 1_000_000, 16)
	})
	b.Run("agent", func(b *testing.B) {
		name, pass := dbtest.Role(b, "")
		agent := proto.Clone(cfg).(*postgresv1.Config)
		agent.Username, agent.AgentUsername = name, name
		secrets := [][]byte{[]byte(pass)}
		h, _ := newHooks(agentLimits())
		h.Mask = func(p []byte) []byte { return mask.Bytes(p, secrets, true) }
		p := serveProxy(b.Context(), b, testResource(b, agent), passwordSecret(pass), withHooks(h))
		stream(b, pair(b, agent, passwordSecret(pass), p), 1000, 512)
	})
}
