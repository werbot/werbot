package metrics_test

import (
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/werbot/werbot/internal/platform/metrics"
)

func TestExposition(t *testing.T) {
	r := metrics.NewRegistry()
	c := r.Counter("werbot_test_total", "Test counter.", "reason", "busy")
	c.Inc()
	c.Add(2)
	assert.Same(t, c, r.Counter("werbot_test_total", "Test counter.", "reason", "busy"))
	r.Counter("werbot_test_total", "Test counter.", "reason", `a"b`).Inc()
	g := r.Gauge("werbot_test_active", "Test gauge.")
	g.Set(5)
	g.Add(-2)
	r.GaugeFunc("werbot_test_func", "Test func.", func() int64 { return 42 })
	rec := httptest.NewRecorder()
	r.Handler().ServeHTTP(rec, httptest.NewRequest("GET", "/metrics", nil))
	assert.Equal(t, 200, rec.Code)
	assert.Equal(t, "text/plain; version=0.0.4", rec.Header().Get("Content-Type"))
	assert.Equal(t, `# HELP werbot_test_active Test gauge.
# TYPE werbot_test_active gauge
werbot_test_active 3
# HELP werbot_test_func Test func.
# TYPE werbot_test_func gauge
werbot_test_func 42
# HELP werbot_test_total Test counter.
# TYPE werbot_test_total counter
werbot_test_total{reason="a\"b"} 1
werbot_test_total{reason="busy"} 3
`, rec.Body.String())
}

func TestRegistryPanics(t *testing.T) {
	r := metrics.NewRegistry()
	r.Counter("werbot_x_total", "x")
	assert.Panics(t, func() { r.Gauge("werbot_x_total", "x") })
	assert.Panics(t, func() { r.Counter("werbot_y_total", "y", "reason") })
	r.GaugeFunc("werbot_f", "f", func() int64 { return 0 })
	assert.Panics(t, func() { r.GaugeFunc("werbot_f", "f", func() int64 { return 0 }) })
}

func TestConcurrentUpdates(t *testing.T) {
	r := metrics.NewRegistry()
	var wg sync.WaitGroup
	for range 100 {
		wg.Go(func() {
			for range 1000 {
				r.Counter("werbot_c_total", "c").Inc()
				r.Gauge("werbot_g", "g").Add(1)
			}
		})
	}
	wg.Wait()
	rec := httptest.NewRecorder()
	r.Handler().ServeHTTP(rec, httptest.NewRequest("GET", "/metrics", nil))
	assert.Contains(t, rec.Body.String(), "werbot_c_total 100000\n")
	assert.Contains(t, rec.Body.String(), "werbot_g 100000\n")
}
