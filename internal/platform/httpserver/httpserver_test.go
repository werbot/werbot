package httpserver_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/werbot/werbot/internal/platform/httpserver"
)

func get(h http.Handler, path string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", path, nil))
	return rec
}

func TestHealth(t *testing.T) {
	rec := get(httpserver.Health(), "/healthz")
	assert.Equal(t, 200, rec.Code)
	assert.Equal(t, "ok", rec.Body.String())
}

func TestReady(t *testing.T) {
	var draining atomic.Bool
	var pings atomic.Int32
	up := func(context.Context) error { pings.Add(1); return nil }
	down := func(context.Context) error { return errors.New("down") }

	rec := get(httpserver.Ready(up, &draining), "/readyz")
	assert.Equal(t, 200, rec.Code)
	assert.Equal(t, "ok", rec.Body.String())

	rec = get(httpserver.Ready(down, &draining), "/readyz")
	assert.Equal(t, 503, rec.Code)
	assert.Equal(t, "unavailable", rec.Body.String())

	draining.Store(true)
	rec = get(httpserver.Ready(up, &draining), "/readyz")
	assert.Equal(t, 503, rec.Code)
	assert.Equal(t, "draining", rec.Body.String())
	assert.EqualValues(t, 1, pings.Load())

	rec = get(httpserver.Health(), "/healthz")
	assert.Equal(t, 200, rec.Code)
	assert.Equal(t, "ok", rec.Body.String())
}

func TestNewServerSettings(t *testing.T) {
	s := httpserver.New(":0", http.NotFoundHandler())
	assert.Equal(t, 10*time.Second, s.ReadHeaderTimeout)
	assert.Equal(t, 120*time.Second, s.IdleTimeout)
	assert.True(t, s.Protocols.UnencryptedHTTP2())
	assert.True(t, s.Protocols.HTTP1())
}
