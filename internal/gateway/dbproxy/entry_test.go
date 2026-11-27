package dbproxy_test

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/gateway/dbproxy"
)

func marshal(t *testing.T, e dbproxy.Entry) string {
	t.Helper()
	b, err := json.Marshal(e)
	require.NoError(t, err)
	return string(b)
}

func TestEntryJSON(t *testing.T) {
	e := dbproxy.Entry{
		T: 1500*time.Millisecond + 400*time.Nanosecond, Kind: "execute", Text: "select $1", DB: "app",
		Params: []string{"42"}, Statement: "s1", Decision: "allow", Rows: 1, Bytes: 12, DurationUS: 340,
	}
	assert.Equal(t, `{"t":1.5,"kind":"execute","sql":"select $1","db":"app","params":["42"],"statement":"s1","decision":"allow","rows":1,"bytes":12,"duration_us":340}`, marshal(t, e))

	full := dbproxy.Entry{
		T: 2*time.Second + 1500*time.Nanosecond, Kind: "query", Text: "select 1", Classes: []string{"read"},
		Tables: []string{"t"}, PolicyIDs: []string{"p1"}, Error: "42601: syntax error",
	}
	assert.Equal(t, `{"t":2.000002,"kind":"query","sql":"select 1","classes":["read"],"tables":["t"],"policy_ids":["p1"],"error":"42601: syntax error"}`, marshal(t, full))

	for _, kind := range []string{"command", "request", "tool_call"} {
		assert.Equal(t, `{"t":0,"kind":"`+kind+`","command":"VACUUM"}`, marshal(t, dbproxy.Entry{Kind: kind, Text: "VACUUM"}))
	}
	assert.Equal(t, `{"t":0,"kind":"cancel"}`, marshal(t, dbproxy.Entry{Kind: "cancel"}), "empty fields are omitted")
}

func TestLogTextTruncate(t *testing.T) {
	assert.Equal(t, "select 1", dbproxy.LogText("select 1"))
	exact := strings.Repeat("x", dbproxy.MaxLogText)
	assert.Equal(t, exact, dbproxy.LogText(exact))

	long := "a" + strings.Repeat("я", 35<<10) // 70 KiB + 1; the cut falls inside a two-byte rune
	got := dbproxy.LogText(long)
	require.True(t, strings.HasSuffix(got, "…[truncated]"))
	assert.LessOrEqual(t, len(got), dbproxy.MaxLogText+len("…[truncated]"))
	assert.True(t, utf8.ValidString(got))
	assert.True(t, strings.HasPrefix(long, strings.TrimSuffix(got, "…[truncated]")))
}

func TestLogParamsTruncate(t *testing.T) {
	assert.Nil(t, dbproxy.LogParams(nil))
	small := []string{"1", "two"}
	assert.Equal(t, small, dbproxy.LogParams(small))

	p1, p2 := strings.Repeat("a", 40<<10), strings.Repeat("b", 40<<10)
	assert.Equal(t, []string{p1, "…[truncated]"}, dbproxy.LogParams([]string{p1, p2}))
	assert.Equal(t, []string{"…[truncated]"}, dbproxy.LogParams([]string{p1 + p2}))
}
