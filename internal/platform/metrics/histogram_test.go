package metrics_test

import (
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/werbot/werbot/internal/platform/metrics"
)

var testBuckets = []float64{0.00005, 0.0001, 0.00025, 0.0005, 0.001, 0.0025, 0.005, 0.01}

func scrape(r *metrics.Registry) string {
	rec := httptest.NewRecorder()
	r.Handler().ServeHTTP(rec, httptest.NewRequest("GET", "/metrics", nil))
	return rec.Body.String()
}

func TestHistogramExposition(t *testing.T) {
	r := metrics.NewRegistry()
	h := r.Histogram("werbot_test_seconds", "Test histogram.", testBuckets, "type", "postgres")
	h.Observe(0.00004)
	h.Observe(0.02)
	assert.Equal(t, `# HELP werbot_test_seconds Test histogram.
# TYPE werbot_test_seconds histogram
werbot_test_seconds_bucket{type="postgres",le="5e-05"} 1
werbot_test_seconds_bucket{type="postgres",le="0.0001"} 1
werbot_test_seconds_bucket{type="postgres",le="0.00025"} 1
werbot_test_seconds_bucket{type="postgres",le="0.0005"} 1
werbot_test_seconds_bucket{type="postgres",le="0.001"} 1
werbot_test_seconds_bucket{type="postgres",le="0.0025"} 1
werbot_test_seconds_bucket{type="postgres",le="0.005"} 1
werbot_test_seconds_bucket{type="postgres",le="0.01"} 1
werbot_test_seconds_bucket{type="postgres",le="+Inf"} 2
werbot_test_seconds_sum{type="postgres"} 0.02004
werbot_test_seconds_count{type="postgres"} 2
`, scrape(r))
}

func TestHistogramNoLabelsAndBoundary(t *testing.T) {
	r := metrics.NewRegistry()
	r.Histogram("h", "H.", []float64{1, 2}).Observe(1) // le is inclusive
	assert.Contains(t, scrape(r), "h_bucket{le=\"1\"} 1\nh_bucket{le=\"2\"} 1\nh_bucket{le=\"+Inf\"} 1\nh_sum 1\nh_count 1\n")
}

func TestHistogramSameObject(t *testing.T) {
	r := metrics.NewRegistry()
	h := r.Histogram("h", "H.", testBuckets, "type", "postgres")
	assert.Same(t, h, r.Histogram("h", "H.", testBuckets, "type", "postgres"))
	assert.NotSame(t, h, r.Histogram("h", "H.", testBuckets, "type", "mysql"))
}

func TestHistogramPanics(t *testing.T) {
	r := metrics.NewRegistry()
	r.Histogram("h", "H.", []float64{1, 2})
	r.Counter("c", "C.")
	for name, fn := range map[string]func(){
		"empty":                      func() { r.Histogram("e", "E.", nil) },
		"not ascending":              func() { r.Histogram("e", "E.", []float64{0.1, 0.1}) },
		"other bounds":               func() { r.Histogram("h", "H.", []float64{1, 3}) },
		"other bounds, other labels": func() { r.Histogram("h", "H.", []float64{1, 3}, "a", "b") },
		"counter name":               func() { r.Histogram("c", "C.", []float64{1}) },
		"histogram name as counter":  func() { r.Counter("h", "H.") },
		"odd labels":                 func() { r.Histogram("o", "O.", []float64{1}, "a") },
	} {
		assert.Panics(t, fn, name)
	}
	assert.NotContains(t, scrape(r), "# TYPE e ")
}

func TestHistogramObserveNoAlloc(t *testing.T) {
	h := metrics.NewRegistry().Histogram("h", "H.", testBuckets)
	assert.Zero(t, testing.AllocsPerRun(1000, func() { h.Observe(0.0003) }))
}

func TestHistogramConcurrent(t *testing.T) {
	r := metrics.NewRegistry()
	h := r.Histogram("h", "H.", testBuckets)
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			for range 10000 {
				h.Observe(0.0009765625) // 2^-10: exact sum in any order
			}
		})
	}
	wg.Wait()
	out := scrape(r)
	assert.True(t, strings.Contains(out, "h_count 80000\n"))
	assert.True(t, strings.Contains(out, "h_sum 78.125\n"), out)
	assert.True(t, strings.Contains(out, "h_bucket{le=\"+Inf\"} 80000\n"))
}

func BenchmarkHistogramObserve(b *testing.B) {
	h := metrics.NewRegistry().Histogram("h", "H.", testBuckets)
	b.ReportAllocs()
	for b.Loop() {
		h.Observe(0.0003)
	}
}
