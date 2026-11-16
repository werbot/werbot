// Package webui runs the werbot web role: the embedded UI plus a reverse proxy
// to the API server. It has no database access.
package webui

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"net/http/httputil"
	"net/netip"
	"net/url"
	"slices"
	"strings"
	"sync/atomic"
	"time"

	"github.com/werbot/werbot/internal/platform/clientip"
	"github.com/werbot/werbot/internal/platform/httpserver"
	"github.com/werbot/werbot/internal/platform/metrics"
)

// Config is the validated werbot web configuration.
type Config struct {
	Addr           string         // WERBOT_WEB_ADDR, default ":8090"
	APIURL         string         // WERBOT_WEB_API_URL, required: http/https URL of the API server
	TrustedProxies []netip.Prefix // WERBOT_TRUSTED_PROXIES: comma-separated CIDRs; empty -> nil
	LogLevel       string         // WERBOT_LOG_LEVEL: debug|info|warn|error, default "info"
	LogFormat      string         // WERBOT_LOG_FORMAT: json|text, default "json"
	MetricsAddr    string         // WERBOT_METRICS_ADDR, default "" (metrics disabled)
}

// ConfigVars lists the environment variables LoadConfig reads.
func ConfigVars() []string {
	return []string{"WERBOT_WEB_ADDR", "WERBOT_WEB_API_URL", "WERBOT_TRUSTED_PROXIES", "WERBOT_LOG_LEVEL", "WERBOT_LOG_FORMAT", "WERBOT_METRICS_ADDR"}
}

// LoadConfig reads and validates configuration using getenv.
func LoadConfig(getenv func(string) string) (Config, error) {
	c := Config{
		Addr:        or(getenv("WERBOT_WEB_ADDR"), ":8090"),
		APIURL:      strings.TrimSuffix(getenv("WERBOT_WEB_API_URL"), "/"),
		LogLevel:    or(getenv("WERBOT_LOG_LEVEL"), "info"),
		LogFormat:   or(getenv("WERBOT_LOG_FORMAT"), "json"),
		MetricsAddr: getenv("WERBOT_METRICS_ADDR"),
	}
	proxies := getenv("WERBOT_TRUSTED_PROXIES")
	if _, err := parseAPIURL(c.APIURL); err != nil {
		return Config{}, err
	}
	if proxies != "" {
		for s := range strings.SplitSeq(proxies, ",") {
			p, err := netip.ParsePrefix(strings.TrimSpace(s))
			if err != nil {
				return Config{}, fmt.Errorf("WERBOT_TRUSTED_PROXIES: %w", err)
			}
			c.TrustedProxies = append(c.TrustedProxies, p)
		}
	}
	// Same values and messages as config.Load.
	if !slices.Contains([]string{"debug", "info", "warn", "error"}, c.LogLevel) {
		return Config{}, fmt.Errorf("WERBOT_LOG_LEVEL: unsupported value %q", c.LogLevel)
	}
	if !slices.Contains([]string{"json", "text"}, c.LogFormat) {
		return Config{}, fmt.Errorf("WERBOT_LOG_FORMAT: unsupported value %q", c.LogFormat)
	}
	return c, nil
}

func or(v, def string) string {
	if v == "" {
		return def
	}
	return v
}

func parseAPIURL(s string) (*url.URL, error) {
	if s == "" {
		return nil, errors.New("WERBOT_WEB_API_URL: required")
	}
	u, err := url.Parse(s)
	if err != nil {
		return nil, fmt.Errorf("WERBOT_WEB_API_URL: %w", err)
	}
	if (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return nil, fmt.Errorf("WERBOT_WEB_API_URL: want http(s)://host, got %q", s)
	}
	return u, nil
}

const connectPrefix = "/werbot.v1."

// Handler serves /healthz, /readyz, proxies Connect (/werbot.v1.*) and /api/
// requests to cfg.APIURL and serves the UI from ui for everything else.
func Handler(cfg Config, ui fs.FS, draining *atomic.Bool, reg *metrics.Registry, logger *slog.Logger) (http.Handler, error) {
	api, err := parseAPIURL(cfg.APIURL)
	if err != nil {
		return nil, err
	}
	upstreamErrors := reg.Counter("werbot_web_upstream_errors_total", "Proxied requests that failed to reach the API server")
	tr := http.DefaultTransport.(*http.Transport).Clone()
	tr.DialContext = (&net.Dialer{Timeout: 5 * time.Second}).DialContext
	tr.ResponseHeaderTimeout = 30 * time.Second
	proxy := &httputil.ReverseProxy{
		Rewrite: func(r *httputil.ProxyRequest) {
			r.SetURL(api)
			if ip := clientip.From(r.In.Context()); ip.IsValid() {
				r.Out.Header.Set("X-Forwarded-For", ip.String())
			}
		},
		Transport:     tr,
		FlushInterval: -1,
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			if r.Context().Err() != nil { // client went away: not an upstream failure
				return
			}
			upstreamErrors.Inc()
			logger.Warn("upstream error", "err", err, "path", r.URL.Path)
			// r is the outgoing request: its path carries the APIURL path prefix.
			if strings.HasPrefix(strings.TrimPrefix(r.URL.Path, api.Path), connectPrefix) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusServiceUnavailable)
				_, _ = io.WriteString(w, `{"code":"unavailable","message":"api unavailable"}`)
				return
			}
			http.Error(w, "api unavailable", http.StatusServiceUnavailable)
		},
	}
	spa := httpserver.SPA(ui)
	mux := http.NewServeMux()
	mux.Handle("/healthz", httpserver.Health())
	mux.Handle("/readyz", httpserver.Ready(func(context.Context) error { return nil }, draining))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, connectPrefix) || strings.HasPrefix(r.URL.Path, "/api/") {
			proxy.ServeHTTP(w, r)
			return
		}
		spa.ServeHTTP(w, r)
	})
	return clientip.Middleware(cfg.TrustedProxies)(mux), nil
}

// Run serves Handler on cfg.Addr (and /metrics on cfg.MetricsAddr when set)
// until ctx ends, then drains and shuts both down within 10s.
func Run(ctx context.Context, cfg Config, ui fs.FS, logger *slog.Logger) error {
	var draining atomic.Bool
	reg := metrics.NewRegistry()
	h, err := Handler(cfg, ui, &draining, reg, logger)
	if err != nil {
		return err
	}
	servers := []*http.Server{httpserver.New(cfg.Addr, h)}
	if cfg.MetricsAddr != "" {
		mmux := http.NewServeMux()
		mmux.Handle("/metrics", reg.Handler())
		servers = append(servers, httpserver.New(cfg.MetricsAddr, mmux))
	}
	errc := make(chan error, len(servers))
	for _, s := range servers {
		go func() { errc <- s.ListenAndServe() }()
	}
	logger.Info("web started", "addr", cfg.Addr, "api_url", cfg.APIURL, "metrics_addr", cfg.MetricsAddr)

	select {
	case err = <-errc:
	case <-ctx.Done():
		draining.Store(true)
	}
	sctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancel()
	for _, s := range servers {
		if serr := s.Shutdown(sctx); serr != nil {
			logger.Error("server shutdown", "addr", s.Addr, "error", serr)
		}
	}
	return err
}
