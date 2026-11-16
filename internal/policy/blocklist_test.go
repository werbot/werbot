package policy

import (
	"context"
	"fmt"
	"net/http"
	"net/netip"
	"testing"

	"github.com/riverqueue/river"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// feed answers every request with status and body.
func feed(status int, body string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(status)
		_, _ = fmt.Fprint(w, body)
	}
}

func runBlocklist(e jobsEnv) error {
	return NewBlocklistRefreshWorker(e.svc, e.cfg).Work(context.Background(), &river.Job[BlocklistRefreshArgs]{})
}

func prefixes(t *testing.T, e jobsEnv) []netip.Prefix {
	t.Helper()
	got, err := e.svc.Blocklist(context.Background())
	require.NoError(t, err)
	return got
}

func TestBlocklistRefresh(t *testing.T) {
	body := "# c\n1.2.3.0/24\n5.6.7.8\nbad\n1.2.3.0/24\n"
	e := newJobsEnv(t).serve(t, func(w http.ResponseWriter, r *http.Request) { feed(200, body)(w, r) })
	require.NoError(t, runBlocklist(e))
	assert.ElementsMatch(t, []netip.Prefix{netip.MustParsePrefix("1.2.3.0/24"), netip.MustParsePrefix("5.6.7.8/32")}, prefixes(t, e))

	body = "9.9.9.0/24\n"
	require.NoError(t, runBlocklist(e))
	assert.Equal(t, []netip.Prefix{netip.MustParsePrefix("9.9.9.0/24")}, prefixes(t, e))
}

func TestBlocklistSkipsPrivateRanges(t *testing.T) {
	e := newJobsEnv(t).serve(t, feed(200, "10.0.0.0/8\n203.0.113.0/24\n0.0.0.0/0\n"))
	require.NoError(t, runBlocklist(e))
	// 0.0.0.0/0 overlaps every private range, so only 203.0.113.0/24 stays.
	assert.Equal(t, []netip.Prefix{netip.MustParsePrefix("203.0.113.0/24")}, prefixes(t, e))
	var blocked bool
	require.NoError(t, e.pool.QueryRow(context.Background(),
		"SELECT EXISTS (SELECT 1 FROM ip_blocklist WHERE cidr >>= '10.1.2.3')").Scan(&blocked))
	assert.False(t, blocked)
}

func TestBlocklistRefreshKeepsPreviousOnFailure(t *testing.T) {
	e := newJobsEnv(t).serve(t, feed(200, "1.2.3.0/24\n"))
	require.NoError(t, runBlocklist(e))
	for name, h := range map[string]http.HandlerFunc{
		"500":      feed(500, "boom"),
		"empty":    feed(200, ""),
		"comments": feed(200, "# only comments\n"),
	} {
		assert.Error(t, runBlocklist(e.serve(t, h)), name)
		assert.Equal(t, []netip.Prefix{netip.MustParsePrefix("1.2.3.0/24")}, prefixes(t, e), name)
	}
}
