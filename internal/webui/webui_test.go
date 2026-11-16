package webui_test

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"testing/fstest"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
	"github.com/werbot/werbot/gen/werbot/v1/werbotv1connect"
	"github.com/werbot/werbot/internal/platform/metrics"
	"github.com/werbot/werbot/internal/webui"
)

func env(m map[string]string) func(string) string { return func(k string) string { return m[k] } }

// recorder returns getenv over m that records every requested key.
func recorder(m map[string]string) (getenv func(string) string, keys func() []string) {
	var (
		mu   sync.Mutex
		seen []string
	)
	getenv = func(k string) string {
		mu.Lock()
		defer mu.Unlock()
		seen = append(seen, k)
		return m[k]
	}
	keys = func() []string {
		mu.Lock()
		defer mu.Unlock()
		return slices.Clone(seen)
	}
	return getenv, keys
}

const apiURL = "http://server:8080/"

func TestLoadConfig(t *testing.T) {
	c, err := webui.LoadConfig(env(map[string]string{"WERBOT_WEB_API_URL": apiURL}))
	require.NoError(t, err)
	assert.Equal(t, webui.Config{Addr: ":8090", APIURL: "http://server:8080", LogLevel: "info", LogFormat: "json"}, c)

	for _, v := range []string{"", "ftp://x", "http://"} {
		_, err = webui.LoadConfig(env(map[string]string{"WERBOT_WEB_API_URL": v}))
		assert.ErrorContains(t, err, "WERBOT_WEB_API_URL", v)
	}

	c, err = webui.LoadConfig(env(map[string]string{"WERBOT_WEB_API_URL": apiURL, "WERBOT_TRUSTED_PROXIES": "10.0.0.0/8, 192.168.1.1/32"}))
	require.NoError(t, err)
	assert.Equal(t, []netip.Prefix{netip.MustParsePrefix("10.0.0.0/8"), netip.MustParsePrefix("192.168.1.1/32")}, c.TrustedProxies)
	_, err = webui.LoadConfig(env(map[string]string{"WERBOT_WEB_API_URL": apiURL, "WERBOT_TRUSTED_PROXIES": "10.0.0.1"}))
	assert.ErrorContains(t, err, "WERBOT_TRUSTED_PROXIES")

	_, err = webui.LoadConfig(env(map[string]string{"WERBOT_WEB_API_URL": apiURL, "WERBOT_LOG_LEVEL": "verbose"}))
	assert.EqualError(t, err, `WERBOT_LOG_LEVEL: unsupported value "verbose"`)
	_, err = webui.LoadConfig(env(map[string]string{"WERBOT_WEB_API_URL": apiURL, "WERBOT_LOG_FORMAT": "xml"}))
	assert.EqualError(t, err, `WERBOT_LOG_FORMAT: unsupported value "xml"`)

	getenv, keys := recorder(map[string]string{"WERBOT_WEB_API_URL": apiURL, "WERBOT_DATABASE_URL": "postgres://x", "WERBOT_SECRET_KEY": "k"})
	_, err = webui.LoadConfig(getenv)
	require.NoError(t, err)
	assert.NotContains(t, keys(), "WERBOT_DATABASE_URL")
	assert.NotContains(t, keys(), "WERBOT_SECRET_KEY")

	_, err = webui.Handler(webui.Config{APIURL: "ftp://x"}, fstest.MapFS{}, new(atomic.Bool), metrics.NewRegistry(), slog.New(slog.DiscardHandler))
	assert.ErrorContains(t, err, "WERBOT_WEB_API_URL")
}

func TestConfigVarsMatchLoadConfig(t *testing.T) {
	getenv, keys := recorder(map[string]string{"WERBOT_WEB_API_URL": apiURL})
	_, err := webui.LoadConfig(getenv)
	require.NoError(t, err)
	vars := webui.ConfigVars()
	assert.Len(t, vars, 6)
	assert.Equal(t, len(vars), len(slices.Compact(slices.Sorted(slices.Values(vars)))), "duplicates in ConfigVars")
	assert.ElementsMatch(t, vars, slices.Compact(slices.Sorted(slices.Values(keys()))))
}

// newWeb serves webui.Handler for cfg (APIURL defaults to upstream) and returns its URL.
func newWeb(t *testing.T, cfg webui.Config, ui fstest.MapFS, reg *metrics.Registry) *httptest.Server {
	t.Helper()
	if ui == nil {
		ui = fstest.MapFS{}
	}
	if reg == nil {
		reg = metrics.NewRegistry()
	}
	h, err := webui.Handler(cfg, ui, new(atomic.Bool), reg, slog.New(slog.DiscardHandler))
	require.NoError(t, err)
	web := httptest.NewServer(h)
	t.Cleanup(web.Close)
	return web
}

func do(t *testing.T, method, url string, hdr map[string]string, body string) (int, http.Header, string) {
	t.Helper()
	req, err := http.NewRequest(method, url, strings.NewReader(body))
	require.NoError(t, err)
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	res, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { _ = res.Body.Close() }()
	b, err := io.ReadAll(res.Body)
	require.NoError(t, err)
	return res.StatusCode, res.Header, string(b)
}

func TestProxyForwardsClientIP(t *testing.T) {
	var (
		mu  sync.Mutex
		got []string
	)
	upstream := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		mu.Lock()
		got = r.Header.Values("X-Forwarded-For")
		mu.Unlock()
	}))
	defer upstream.Close()
	local := []netip.Prefix{netip.MustParsePrefix("127.0.0.1/32")}
	for _, c := range []struct {
		trusted []netip.Prefix
		xff     string
		want    []string
	}{
		{nil, "1.2.3.4", []string{"127.0.0.1"}},
		{local, "198.51.100.7", []string{"198.51.100.7"}},
		{local, "203.0.113.1, 198.51.100.7", []string{"198.51.100.7"}},
	} {
		web := newWeb(t, webui.Config{APIURL: upstream.URL, TrustedProxies: c.trusted}, nil, nil)
		code, _, _ := do(t, http.MethodPost, web.URL+"/werbot.v1.SystemService/GetVersion", map[string]string{"X-Forwarded-For": c.xff}, "{}")
		assert.Equal(t, http.StatusOK, code)
		mu.Lock()
		assert.Equal(t, c.want, got, c.xff)
		mu.Unlock()
	}
}

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

	for _, p := range []string{"/", "/login"} {
		code, hdr, body := do(t, http.MethodGet, web.URL+p, map[string]string{"Accept": "text/html"}, "")
		assert.Equal(t, http.StatusOK, code, p)
		assert.Equal(t, "<html>app</html>", body, p)
		assert.NotEmpty(t, hdr.Get("Content-Security-Policy"), p)
	}

	n := calls.Load()
	code, _, _ := do(t, http.MethodPost, web.URL+"/werbot.v2.X/Y", map[string]string{"Content-Type": "application/json"}, "{}")
	assert.Equal(t, http.StatusNotFound, code)
	assert.Equal(t, n, calls.Load(), "upstream must not be called")
	assert.EqualValues(t, 2, n)
}

func metricsText(t *testing.T, reg *metrics.Registry) string {
	t.Helper()
	rec := httptest.NewRecorder()
	reg.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	return rec.Body.String()
}

func TestUpstreamDown(t *testing.T) {
	reg := metrics.NewRegistry()
	web := newWeb(t, webui.Config{APIURL: "http://127.0.0.1:1"}, nil, reg)

	code, hdr, body := do(t, http.MethodPost, web.URL+"/werbot.v1.SystemService/GetVersion", map[string]string{"Content-Type": "application/json"}, "{}")
	assert.Equal(t, http.StatusServiceUnavailable, code)
	assert.Equal(t, "application/json", hdr.Get("Content-Type"))
	assert.Equal(t, `{"code":"unavailable","message":"api unavailable"}`, body)

	_, err := werbotv1connect.NewSystemServiceClient(http.DefaultClient, web.URL).
		GetVersion(context.Background(), connect.NewRequest(&werbotv1.GetVersionRequest{}))
	assert.Equal(t, connect.CodeUnavailable, connect.CodeOf(err))

	code, hdr, body = do(t, http.MethodGet, web.URL+"/api/recordings/x.cast", nil, "")
	assert.Equal(t, http.StatusServiceUnavailable, code)
	assert.True(t, strings.HasPrefix(hdr.Get("Content-Type"), "text/plain"), hdr.Get("Content-Type"))
	assert.Contains(t, body, "api unavailable")

	assert.Contains(t, metricsText(t, reg), "\nwerbot_web_upstream_errors_total 3\n")

	for _, p := range []string{"/readyz", "/healthz"} {
		code, _, body = do(t, http.MethodGet, web.URL+p, nil, "")
		assert.Equal(t, http.StatusOK, code, p)
		assert.Equal(t, "ok", body, p)
	}
}

// syncBuffer is a bytes.Buffer safe for concurrent writes by the server and reads by the test.
type syncBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *syncBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}
func (s *syncBuffer) String() string { s.mu.Lock(); defer s.mu.Unlock(); return s.b.String() }

func TestClientCancelNotCounted(t *testing.T) {
	received := make(chan struct{})
	release := make(chan struct{})
	upstream := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		close(received)
		<-release
	}))
	defer upstream.Close()
	reg := metrics.NewRegistry()
	var logs syncBuffer
	h, err := webui.Handler(webui.Config{APIURL: upstream.URL}, fstest.MapFS{}, new(atomic.Bool), reg, slog.New(slog.NewTextHandler(&logs, nil)))
	require.NoError(t, err)
	web := httptest.NewServer(h)

	ctx, cancel := context.WithCancel(context.Background())
	go func() { <-received; cancel() }()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, web.URL+"/werbot.v1.SystemService/GetVersion", strings.NewReader("{}"))
	require.NoError(t, err)
	_, err = http.DefaultClient.Do(req) //nolint:bodyclose // request is canceled, no response
	require.ErrorIs(t, err, context.Canceled)

	close(release)
	web.Close() // waits for the proxy handler (and its ErrorHandler) to finish
	assert.Contains(t, metricsText(t, reg), "\nwerbot_web_upstream_errors_total 0\n")
	assert.NotContains(t, logs.String(), "upstream error")
}

func TestRunRedactsAPIURL(t *testing.T) {
	var logs syncBuffer
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	cfg := webui.Config{Addr: "127.0.0.1:0", APIURL: "http://u:s3cret@127.0.0.1:1"} //nolint:gosec // fake credential checked for redaction
	require.NoError(t, webui.Run(ctx, cfg, fstest.MapFS{}, slog.New(slog.NewTextHandler(&logs, nil))))
	assert.Contains(t, logs.String(), "web started")
	assert.NotContains(t, logs.String(), "s3cret")
}

func TestConnectRejected(t *testing.T) {
	reg := metrics.NewRegistry()
	web := newWeb(t, webui.Config{APIURL: "http://127.0.0.1:1"}, nil, reg)

	code, hdr, _ := do(t, http.MethodConnect, web.URL+"/werbot.v1.SystemService/GetVersion", nil, "")
	assert.Equal(t, http.StatusMethodNotAllowed, code)
	assert.NotEmpty(t, hdr.Get("Allow"))
	assert.Contains(t, metricsText(t, reg), "\nwerbot_web_upstream_errors_total 0\n")
}

func TestRunAddrInUse(t *testing.T) {
	busy, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer func() { _ = busy.Close() }()
	addr := busy.Addr().String()

	var logs syncBuffer
	done := make(chan error, 1)
	go func() {
		cfg := webui.Config{Addr: "127.0.0.1:0", MetricsAddr: addr, APIURL: apiURL}
		done <- webui.Run(context.Background(), cfg, fstest.MapFS{}, slog.New(slog.NewTextHandler(&logs, nil)))
	}()
	select {
	case err := <-done:
		assert.ErrorContains(t, err, addr)
	case <-time.After(5 * time.Second):
		t.Fatal("webui.Run did not return on a busy address")
	}
	assert.NotContains(t, logs.String(), "web started")
}
