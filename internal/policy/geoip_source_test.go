package policy

import (
	"archive/tar"
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/policy/geoiptest"
)

// tarball packs name/body pairs in order; a name ending in "/" is a directory.
func tarball(t *testing.T, pairs ...string) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := tar.NewWriter(&buf)
	for i := 0; i+1 < len(pairs); i += 2 {
		h := &tar.Header{Name: pairs[i], Mode: 0o644, Size: int64(len(pairs[i+1])), Typeflag: tar.TypeReg}
		if strings.HasSuffix(h.Name, "/") {
			h.Typeflag, h.Size = tar.TypeDir, 0
		}
		require.NoError(t, w.WriteHeader(h))
		_, err := w.Write([]byte(pairs[i+1]))
		require.NoError(t, err)
	}
	require.NoError(t, w.Close())
	return buf.Bytes()
}

// fetch runs one download of body from a test server.
func fetch(t *testing.T, body []byte, maxBytes int64) ([]byte, error) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write(body) }))
	t.Cleanup(srv.Close)
	cfg := DefaultJobsConfig()
	cfg.GeoIPMaxBytes = maxBytes
	return NewGeoIPRefreshWorker(nil, cfg).download(context.Background(), srv.URL+"/db")
}

func TestGeoIPDownloadFormats(t *testing.T) {
	db := geoiptest.MMDB(netip.MustParsePrefix("10.0.0.0/8"), "DE")
	maxmind := tarball(t, "GeoLite2-Country_20261117/", "",
		"GeoLite2-Country_20261117/COPYRIGHT.txt", "copyright",
		"GeoLite2-Country_20261117/GeoLite2-Country.mmdb", string(db))
	for name, body := range map[string][]byte{
		"raw":    db,
		"gzip":   gz(t, db),
		"tar":    maxmind,
		"tar.gz": gz(t, maxmind),
	} {
		got, err := fetch(t, body, int64(len(db)))
		require.NoError(t, err, name)
		assert.Equal(t, db, got, name)
	}
}

func TestGeoIPDownloadRejects(t *testing.T) {
	db := geoiptest.MMDB(netip.MustParsePrefix("10.0.0.0/8"), "DE")
	_, err := fetch(t, gz(t, tarball(t, "COPYRIGHT.txt", "copyright", "LICENSE.txt", "license")), 1<<20)
	assert.ErrorContains(t, err, "no .mmdb file")
	_, err = fetch(t, gz(t, tarball(t, "x.mmdb", string(db))), int64(len(db))-1)
	assert.ErrorContains(t, err, "exceeds", "tar entry over the limit")
	_, err = fetch(t, db, int64(len(db))-1)
	assert.ErrorContains(t, err, "exceeds", "raw over the limit")
}

func TestGeoIPDownloadErrorsOmitURL(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusForbidden) }))
	t.Cleanup(srv.Close)
	w := NewGeoIPRefreshWorker(nil, DefaultJobsConfig())
	for _, u := range []string{
		srv.URL + "/db?token=SECRET",
		"http://user:SECRET@127.0.0.1:1/db",
		"http://127.0.0.1:1/db?token=SECRET",
		"http://bad host/db?token=SECRET",
	} {
		_, err := w.download(context.Background(), u)
		require.Error(t, err, u)
		assert.NotContains(t, err.Error(), "SECRET", u)
		assert.NotContains(t, err.Error(), "/db", u)
	}
}

func TestGeoIPRefreshFixedURL(t *testing.T) {
	de := geoiptest.MMDB(netip.MustParsePrefix("10.0.0.0/8"), "DE")
	f := &geoFeed{}
	e := newJobsEnv(t).serve(t, f.ServeHTTP)
	e.cfg.GeoIPURL = strings.Replace(e.cfg.GeoIPURL, "dbip-country-lite-%s.mmdb.gz", "country.mmdb", 1)

	assert.Error(t, runGeoIP(e), "404")
	assert.Equal(t, []string{"/country.mmdb"}, f.asked(), "no month fallback")

	f.set("/country.mmdb", 200, de)
	require.NoError(t, runGeoIP(e))
	month, data := stored(t, e)
	assert.Equal(t, "2026-11-01", month)
	assert.Equal(t, de, data)
	assert.Equal(t, "DE", e.svc.Country(geoTestIP))

	require.NoError(t, runGeoIP(e))
	assert.Len(t, f.asked(), 2, "this month is stored: no download")
}
