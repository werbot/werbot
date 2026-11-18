package policy

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/rand"
	"net/http"
	"net/netip"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/policy/geoiptest"
)

var geoTestIP = netip.MustParseAddr("10.1.2.3")

func gz(t *testing.T, b []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := gzip.NewWriter(&buf)
	_, err := w.Write(b)
	require.NoError(t, err)
	require.NoError(t, w.Close())
	return buf.Bytes()
}

// geoFeed answers by request path (status+body) and records the paths asked.
type geoFeed struct {
	mu    sync.Mutex
	paths []string
	reply map[string]geoReply
}

type geoReply struct {
	status int
	body   []byte
}

func (f *geoFeed) set(path string, status int, body []byte) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.reply == nil {
		f.reply = map[string]geoReply{}
	}
	f.reply[path] = geoReply{status, body}
}

func (f *geoFeed) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.paths = append(f.paths, r.URL.Path)
	rep, ok := f.reply[r.URL.Path]
	if !ok {
		rep.status = http.StatusNotFound
	}
	w.WriteHeader(rep.status)
	_, _ = w.Write(rep.body)
}

func (f *geoFeed) asked() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.paths)
}

const (
	pathNov = "/dbip-country-lite-2026-11.mmdb.gz"
	pathOct = "/dbip-country-lite-2026-10.mmdb.gz"
)

func runGeoIP(e jobsEnv) error {
	return NewGeoIPRefreshWorker(e.svc, e.cfg).Work(context.Background(), &river.Job[GeoIPRefreshArgs]{})
}

// stored returns the month and data of the geoip_db row.
func stored(t *testing.T, e jobsEnv) (month string, data []byte) {
	t.Helper()
	require.NoError(t, e.pool.QueryRow(context.Background(), "SELECT month::text, data FROM geoip_db WHERE id = 1").Scan(&month, &data))
	return month, data
}

func noNotification(t *testing.T, conn *pgx.Conn) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	_, err := conn.WaitForNotification(ctx)
	assert.ErrorIs(t, err, context.DeadlineExceeded)
}

// withStoredDE seeds the October row and loads it, as an earlier run would have.
func withStoredDE(t *testing.T, e jobsEnv) []byte {
	t.Helper()
	de := geoiptest.MMDB(netip.MustParsePrefix("10.0.0.0/8"), "DE")
	_, err := e.pool.Exec(context.Background(), "INSERT INTO geoip_db (id, month, data) VALUES (1, '2026-10-01', $1)", de)
	require.NoError(t, err)
	require.NoError(t, e.svc.ReloadGeoIP(context.Background()))
	return de
}

func assertKept(t *testing.T, e jobsEnv, conn *pgx.Conn, de []byte, msg string) {
	t.Helper()
	month, data := stored(t, e)
	assert.Equal(t, "2026-10-01", month, msg)
	assert.Equal(t, de, data, msg)
	assert.Equal(t, "DE", e.svc.Country(geoTestIP), msg)
	noNotification(t, conn)
}

func TestGeoIPRefreshWritesRowAndNotifies(t *testing.T) {
	de := geoiptest.MMDB(netip.MustParsePrefix("10.0.0.0/8"), "DE")
	f := &geoFeed{}
	f.set(pathNov, 200, gz(t, de))
	e := newJobsEnv(t).serve(t, f.ServeHTTP)
	conn := listen(t, e.pool)

	require.NoError(t, runGeoIP(e))
	month, data := stored(t, e)
	assert.Equal(t, "2026-11-01", month)
	assert.Equal(t, de, data)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	n, err := conn.WaitForNotification(ctx)
	require.NoError(t, err)
	assert.Equal(t, "werbot_geoip_changed", n.Channel)
	assert.Empty(t, n.Payload)
	assert.Equal(t, "DE", e.svc.Country(geoTestIP))
}

func TestGeoIPRefreshFallsBackToPreviousMonth(t *testing.T) {
	fr := geoiptest.MMDB(netip.MustParsePrefix("10.0.0.0/8"), "FR")
	f := &geoFeed{}
	f.set(pathOct, 200, gz(t, fr))
	e := newJobsEnv(t).serve(t, f.ServeHTTP)

	require.NoError(t, runGeoIP(e))
	month, _ := stored(t, e)
	assert.Equal(t, "2026-10-01", month)
	assert.Equal(t, "FR", e.svc.Country(geoTestIP))
	assert.Equal(t, []string{pathNov, pathOct}, f.asked())

	require.NoError(t, runGeoIP(e))
	assert.Equal(t, []string{pathNov, pathOct, pathNov, pathOct}, f.asked(), "a stored previous month is fetched again")
}

func TestGeoIPRefreshKeepsPreviousOnFailure(t *testing.T) {
	f := &geoFeed{}
	e := newJobsEnv(t).serve(t, f.ServeHTTP)
	de := withStoredDE(t, e)
	conn := listen(t, e.pool)

	random := make([]byte, 4096)
	_, _ = rand.Read(random)
	f.set(pathOct, 200, gz(t, random))
	assert.Error(t, runGeoIP(e), "404 then a non-mmdb database")
	assertKept(t, e, conn, de, "non-mmdb")

	f.set(pathNov, 500, nil)
	assert.Error(t, runGeoIP(e), "500")
	assertKept(t, e, conn, de, "500")
}

func TestGeoIPRefreshRejectsBadArchive(t *testing.T) {
	fr := geoiptest.MMDB(netip.MustParsePrefix("10.0.0.0/8"), "FR")
	frGz := gz(t, fr)
	f := &geoFeed{}
	e := newJobsEnv(t).serve(t, f.ServeHTTP)
	de := withStoredDE(t, e)
	conn := listen(t, e.pool)

	f.set(pathNov, 200, []byte("not gzip"))
	assert.Error(t, runGeoIP(e), "not gzip")
	assertKept(t, e, conn, de, "not gzip")

	f.set(pathNov, 200, frGz[:len(frGz)/2])
	assert.Error(t, runGeoIP(e), "truncated")
	assertKept(t, e, conn, de, "truncated")

	f.set(pathNov, 200, frGz)
	e.cfg.GeoIPMaxBytes = int64(len(fr)) - 1
	assert.ErrorContains(t, runGeoIP(e), "exceeds")
	assertKept(t, e, conn, de, "too large")
}

func TestGeoIPRefreshSkipsCurrentMonth(t *testing.T) {
	f := &geoFeed{}
	e := newJobsEnv(t).serve(t, f.ServeHTTP)
	de := geoiptest.MMDB(netip.MustParsePrefix("10.0.0.0/8"), "DE")
	_, err := e.pool.Exec(context.Background(), "INSERT INTO geoip_db (id, month, data) VALUES (1, '2026-11-01', $1)", de)
	require.NoError(t, err)
	require.NoError(t, runGeoIP(e))
	assert.Empty(t, f.asked())
	assert.Equal(t, "DE", e.svc.Country(geoTestIP), "the skip path retries a failed reload")
}

func TestGeoIPRefreshFailsWhenNothingPublished(t *testing.T) {
	f := &geoFeed{}
	e := newJobsEnv(t).serve(t, f.ServeHTTP)
	assert.Error(t, runGeoIP(e))
	assert.Equal(t, []string{pathNov, pathOct}, f.asked())
	var n int
	require.NoError(t, e.pool.QueryRow(context.Background(), "SELECT count(*) FROM geoip_db").Scan(&n))
	assert.Zero(t, n)
}

func TestGeoIPRefreshRejectsCorruptTree(t *testing.T) {
	f := &geoFeed{}
	e := newJobsEnv(t).serve(t, f.ServeHTTP)
	de := withStoredDE(t, e)
	conn := listen(t, e.pool)
	bad := geoiptest.MMDB(netip.MustParsePrefix("10.0.0.0/8"), "FR")
	copy(bad, "\xFF\xFF\xFF") // first record points far outside the data section
	f.set(pathNov, 200, gz(t, bad))
	assert.Error(t, runGeoIP(e))
	assertKept(t, e, conn, de, "corrupt tree")
}
