package webui_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/webui"
)

// TestProxyWebSocketUpgrade: werbot web proxies a WebSocket upgrade on /api/
// to the API server unchanged (Origin kept, client IP forwarded).
func TestProxyWebSocketUpgrade(t *testing.T) {
	var (
		mu          sync.Mutex
		origin, xff string
	)
	mux := http.NewServeMux()
	mux.HandleFunc("/api/terminal", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		origin, xff = r.Header.Get("Origin"), r.Header.Get("X-Forwarded-For")
		mu.Unlock()
		c, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
		if err != nil {
			return
		}
		defer func() { _ = c.CloseNow() }()
		for {
			typ, b, err := c.Read(r.Context())
			if err != nil || c.Write(r.Context(), typ, b) != nil {
				return
			}
		}
	})
	mux.HandleFunc("POST /werbot.v1.SystemService/GetVersion", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	upstream := httptest.NewServer(mux)
	defer upstream.Close()
	web := newWeb(t, webui.Config{APIURL: upstream.URL}, nil, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	c, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(web.URL, "http")+"/api/terminal", //nolint:bodyclose // websocket.Dial owns the body
		&websocket.DialOptions{HTTPHeader: http.Header{"Origin": {"https://ui.example.com"}}})
	require.NoError(t, err)
	defer func() { _ = c.CloseNow() }()
	for i := range 10 {
		msg := "msg " + strconv.Itoa(i)
		require.NoError(t, c.Write(ctx, websocket.MessageBinary, []byte(msg)))
		typ, b, err := c.Read(ctx)
		require.NoError(t, err)
		assert.Equal(t, websocket.MessageBinary, typ)
		assert.Equal(t, msg, string(b))
	}
	require.NoError(t, c.Close(websocket.StatusNormalClosure, ""))
	mu.Lock()
	assert.Equal(t, "https://ui.example.com", origin, "Origin reaches the API unchanged")
	assert.Equal(t, "127.0.0.1", xff)
	mu.Unlock()

	code, _, _ := do(t, http.MethodPost, web.URL+"/werbot.v1.SystemService/GetVersion", nil, "{}")
	assert.Equal(t, http.StatusOK, code, "Connect is still proxied")
}
