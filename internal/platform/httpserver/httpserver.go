// Package httpserver builds the HTTP server and liveness/readiness handlers.
package httpserver

import (
	"context"
	"net"
	"net/http"
	"sync/atomic"
	"time"
)

// New returns a server speaking HTTP/1 and unencrypted HTTP/2 (h2c for Connect gRPC clients).
func New(addr string, h http.Handler) *http.Server {
	p := new(http.Protocols)
	p.SetHTTP1(true)
	p.SetUnencryptedHTTP2(true)
	return &http.Server{
		Addr:              addr,
		Handler:           h,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       120 * time.Second,
		Protocols:         p,
	}
}

// Listen binds every server's Addr. On error it closes the listeners already
// bound and returns the error, which names the address.
func Listen(servers []*http.Server) ([]net.Listener, error) {
	lns := make([]net.Listener, 0, len(servers))
	for _, s := range servers {
		ln, err := net.Listen("tcp", s.Addr)
		if err != nil {
			for _, l := range lns {
				_ = l.Close()
			}
			return nil, err
		}
		lns = append(lns, ln)
	}
	return lns, nil
}

// Health is the liveness probe: always 200. It never pings the DB and ignores drain.
func Health() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok"))
	})
}

// Ready is the readiness probe: 503 while draining or when ping fails within 2s.
func Ready(ping func(context.Context) error, draining *atomic.Bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if draining.Load() {
			unavailable(w, "draining")
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := ping(ctx); err != nil {
			unavailable(w, "unavailable")
			return
		}
		_, _ = w.Write([]byte("ok"))
	})
}

func unavailable(w http.ResponseWriter, body string) {
	w.WriteHeader(http.StatusServiceUnavailable)
	_, _ = w.Write([]byte(body))
}
