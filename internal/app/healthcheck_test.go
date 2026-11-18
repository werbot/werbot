package app

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunHealthcheck(t *testing.T) {
	cases := []struct {
		name, host string
		status     int
		wantErr    string
	}{
		{"ok", "", http.StatusOK, ""},
		{"unspecified host", "0.0.0.0", http.StatusOK, ""},
		{"unhealthy", "", http.StatusServiceUnavailable, "503"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/healthz" {
					http.NotFound(w, r)
					return
				}
				w.WriteHeader(c.status)
			}))
			defer srv.Close()
			u, err := url.Parse(srv.URL)
			require.NoError(t, err)
			err = Run(context.Background(), []string{"healthcheck"}, env(map[string]string{"WERBOT_HTTP_ADDR": c.host + ":" + u.Port()}), io.Discard)
			if c.wantErr == "" {
				assert.NoError(t, err)
				return
			}
			assert.ErrorContains(t, err, c.wantErr)
		})
	}
}

func TestRunHealthcheckDown(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	_, port, err := net.SplitHostPort(l.Addr().String())
	require.NoError(t, err)
	require.NoError(t, l.Close())

	start := time.Now()
	err = Run(context.Background(), []string{"healthcheck"}, env(map[string]string{"WERBOT_HTTP_ADDR": "127.0.0.1:" + port}), io.Discard)
	assert.Error(t, err)
	assert.Less(t, time.Since(start), 5*time.Second)
}
