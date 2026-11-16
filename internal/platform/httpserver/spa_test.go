package httpserver_test

import (
	"crypto/sha256"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"

	"github.com/werbot/werbot/internal/platform/httpserver"
)

func getAccept(h http.Handler, path, accept string) *httptest.ResponseRecorder {
	req := httptest.NewRequest("GET", path, nil)
	if accept != "" {
		req.Header.Set("Accept", accept)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestSPA(t *testing.T) {
	fsys := fstest.MapFS{"index.html": {Data: []byte("<html>app</html>")}, "_app/a.js": {Data: []byte("js")}}
	h := httpserver.SPA(fsys)
	cases := []struct {
		path, accept string
		code         int
		body         string
	}{
		{"/", "text/html", 200, "<html>app</html>"},
		{"/_app/a.js", "", 200, "js"},
		{"/resources/abc", "text/html,application/xhtml+xml", 200, "<html>app</html>"},
		{"/resources/abc", "", 404, ""},
		{"/werbot.v1.Nope/Get", "application/json", 404, ""},
	}
	for _, c := range cases {
		rec := getAccept(h, c.path, c.accept)
		assert.Equal(t, c.code, rec.Code, c.path)
		if c.code == 200 {
			assert.Equal(t, c.body, rec.Body.String(), c.path)
		}
	}
}

func TestSPANotBuilt(t *testing.T) {
	rec := getAccept(httpserver.SPA(fstest.MapFS{}), "/anything", "text/html")
	assert.Equal(t, 503, rec.Code)
	assert.Contains(t, rec.Body.String(), "make web")
}

func TestSPAHeaders(t *testing.T) {
	idx := `<html><div style="display: contents"><script>start()</script><script type="module" src="/_app/x.js"></script></div></html>`
	fsys := fstest.MapFS{"index.html": {Data: []byte(idx)}, "_app/a.js": {Data: []byte("js")}}
	sum := sha256.Sum256([]byte("start()"))
	want := "default-src 'self'; script-src 'self' 'wasm-unsafe-eval' 'sha256-" + base64.StdEncoding.EncodeToString(sum[:]) +
		"'; style-src 'self' 'unsafe-inline'; frame-ancestors 'none'"
	h := httpserver.SPA(fsys)
	for _, c := range []struct {
		path, accept string
		code         int
	}{
		{"/", "text/html", 200},
		{"/_app/a.js", "", 200},
		{"/nope", "", 404},
	} {
		rec := getAccept(h, c.path, c.accept)
		assert.Equal(t, c.code, rec.Code, c.path)
		assert.Equal(t, want, rec.Header().Get("Content-Security-Policy"), c.path)
		assert.Equal(t, "nosniff", rec.Header().Get("X-Content-Type-Options"), c.path)
		assert.Equal(t, "same-origin", rec.Header().Get("Referrer-Policy"), c.path)
	}

	rec := getAccept(httpserver.SPA(fstest.MapFS{}), "/", "text/html")
	assert.Equal(t, "default-src 'self'; script-src 'self' 'wasm-unsafe-eval'; style-src 'self' 'unsafe-inline'; frame-ancestors 'none'",
		rec.Header().Get("Content-Security-Policy"))
	assert.Equal(t, "nosniff", rec.Header().Get("X-Content-Type-Options"))
	assert.Equal(t, "same-origin", rec.Header().Get("Referrer-Policy"))
}
