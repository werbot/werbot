package gateway

import (
	"io"
	"log/slog"
	"net"
	"testing"
	"time"

	"github.com/werbot/werbot/internal/platform/metrics"
)

// BenchmarkPipelineOpenStream measures what a PreSession adds to Open: the
// stream accepted, its first 8 bytes peeked under the timeout, the peeked
// stream handed on. Budget: 0.2 ms per connection (added-ms).
func BenchmarkPipelineOpenStream(b *testing.B) {
	p := &Pipeline{log: slog.New(slog.DiscardHandler), panics: metrics.NewRegistry().Counter("werbot_gateway_panics_total", "")}
	res := Resource{Name: "pre", Type: "preconn"}
	start := make([]byte, 8)
	b.ReportAllocs()
	begin := time.Now()
	for b.Loop() {
		srv, cli := net.Pipe()
		go func() { _, _ = cli.Write(start) }()
		r := OpenRequest{Kind: KindStream, Accept: func() (io.ReadWriteCloser, error) { return srv, nil }}
		if _, _, err := p.preSession(b.Context(), r, res, startArgs{}); err != nil {
			b.Fatal(err)
		}
		_ = cli.Close()
		_ = srv.Close()
	}
	added := time.Since(begin).Seconds() * 1000 / float64(b.N)
	b.ReportMetric(added, "added-ms")
	if added > 0.2 {
		b.Fatalf("PreSession adds %.3f ms per connection, budget 0.2 ms", added)
	}
}
