package gateway

import (
	"net/http"

	"github.com/werbot/werbot/internal/platform/metrics"
)

// newHTTPHandler serves /readyz (200 "ok" only while the tunnel is up with a
// snapshot, a slot is free and no drain runs; else 503 with the reason) and /metrics.
func newHTTPHandler(p *Pipeline, reg *metrics.Registry) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, _ *http.Request) {
		if ok, reason := p.ready(); !ok {
			http.Error(w, reason, http.StatusServiceUnavailable)
			return
		}
		_, _ = w.Write([]byte("ok"))
	})
	mux.Handle("GET /metrics", reg.Handler())
	return mux
}

// ready is the /readyz state: a reason is "not_ready", "busy" or "draining".
func (p *Pipeline) ready() (bool, string) {
	switch {
	case p.active.isDraining():
		return false, "draining"
	case p.ctl.Policy() == nil || !p.ctl.Connected():
		return false, "not_ready"
	case p.active.used() >= p.cfg.MaxSessions:
		return false, "busy"
	}
	return true, ""
}
