package webui_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"

	"github.com/werbot/werbot/internal/webui"
)

func TestProxyPrefixes(t *testing.T) {
	var (
		calls atomic.Int32
		mu    sync.Mutex
		uri   string
	)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		mu.Lock()
		uri = r.URL.RequestURI()
		mu.Unlock()
		w.Header().Set("Set-Cookie", "werbot_session=x; Path=/; HttpOnly")
		_, _ = io.WriteString(w, "api")
	}))
	defer upstream.Close()
	ui := fstest.MapFS{"index.html": {Data: []byte("<html>app</html>")}}
	web := newWeb(t, webui.Config{APIURL: upstream.URL}, ui, nil)
	lastURI := func() string { mu.Lock(); defer mu.Unlock(); return uri }

	_, hdr, body := do(t, http.MethodPost, web.URL+"/werbot.v1.SystemService/GetVersion", map[string]string{"Content-Type": "application/json"}, "{}")
	assert.Equal(t, "api", body)
	assert.Equal(t, "/werbot.v1.SystemService/GetVersion", lastURI())
	assert.Equal(t, []string{"werbot_session=x; Path=/; HttpOnly"}, hdr.Values("Set-Cookie"))

	_, _, body = do(t, http.MethodGet, web.URL+"/api/recordings/x.cast?y=1", nil, "")
	assert.Equal(t, "api", body)
	assert.Equal(t, "/api/recordings/x.cast?y=1", lastURI())

	// The OAuth authorization server lives on the API server (stage 9).
	_, _, body = do(t, http.MethodGet, web.URL+"/.well-known/openid-configuration", nil, "")
	assert.Equal(t, "api", body)
	assert.Equal(t, "/.well-known/openid-configuration", lastURI())
	_, _, body = do(t, http.MethodPost, web.URL+"/oauth/token", map[string]string{"Content-Type": "application/x-www-form-urlencoded"}, "grant_type=refresh_token")
	assert.Equal(t, "api", body)
	assert.Equal(t, "/oauth/token", lastURI())

	// The consent and device pages are UI routes, outside the proxied prefixes.
	for _, p := range []string{"/", "/login", "/cli/authorize?id=1", "/device"} {
		code, hdr, body := do(t, http.MethodGet, web.URL+p, map[string]string{"Accept": "text/html"}, "")
		assert.Equal(t, http.StatusOK, code, p)
		assert.Equal(t, "<html>app</html>", body, p)
		assert.NotEmpty(t, hdr.Get("Content-Security-Policy"), p)
	}

	n := calls.Load()
	code, _, _ := do(t, http.MethodPost, web.URL+"/werbot.v2.X/Y", map[string]string{"Content-Type": "application/json"}, "{}")
	assert.Equal(t, http.StatusNotFound, code)
	assert.Equal(t, n, calls.Load(), "upstream must not be called")
	assert.EqualValues(t, 4, n)
}
