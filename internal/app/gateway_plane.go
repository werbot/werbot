package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/werbot/werbot/internal/platform/config"
	"github.com/werbot/werbot/internal/platform/metrics"
	"github.com/werbot/werbot/internal/platform/pgnotify"
	"github.com/werbot/werbot/internal/tunnel"
)

// tunnelURL is the tunnel address handed to enrolled gateways: the host of
// publicURL with the port of gatewayAddr. A load balancer in front of the
// replicas must pass that port through at L4, since mTLS ends on the replica.
func tunnelURL(publicURL, gatewayAddr string) (string, error) {
	u, err := url.Parse(publicURL)
	if err != nil || u.Hostname() == "" {
		return "", fmt.Errorf("WERBOT_PUBLIC_URL: want http(s)://host, got %q", publicURL)
	}
	_, port, err := net.SplitHostPort(gatewayAddr)
	if err != nil || port == "" {
		return "", fmt.Errorf("WERBOT_GATEWAY_ADDR: want [host]:port, got %q", gatewayAddr)
	}
	return "https://" + net.JoinHostPort(u.Hostname(), port), nil
}

// tunnelHosts are the SANs of this replica's tunnel certificate.
func tunnelHosts(publicURL string) []string {
	u, _ := url.Parse(publicURL) // validated by tunnelURL before serve binds anything
	return []string{u.Hostname(), "localhost", "127.0.0.1", "::1"}
}

// runGatewayPlane serves TunnelService over mTLS (HTTP/2 only) on ln until
// ctx ends, then shuts the server down within 10s. Streams must be closed
// with hub.Shutdown first, or the shutdown waits for them.
func runGatewayPlane(ctx context.Context, ln net.Listener, cfg config.Config, s *services, hub *tunnel.Hub, reg *metrics.Registry, logger *slog.Logger) error {
	tlsCfg, err := s.CA.ServerTLS(tunnelHosts(cfg.PublicURL))
	if err != nil {
		_ = ln.Close()
		return err
	}
	mux := http.NewServeMux()
	tunnel.New(newTunnelDeps(s, reg, logger), hub).Mount(mux)
	var protos http.Protocols
	protos.SetHTTP2(true)
	srv := &http.Server{Handler: mux, TLSConfig: tlsCfg, Protocols: &protos, ReadHeaderTimeout: 10 * time.Second,
		ErrorLog: slog.NewLogLogger(logger.Handler(), slog.LevelDebug)} // failed handshakes of port scanners
	errc := make(chan error, 1)
	go func() { errc <- srv.ServeTLS(ln, "", "") }()
	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
	}
	sctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancel()
	err = srv.Shutdown(sctx)
	if serr := <-errc; !errors.Is(serr, http.ErrServerClosed) && err == nil {
		err = serr
	}
	return err
}

// listenNotify runs the process's only pgnotify.Listen until ctx ends: org
// changes go to hub, GeoIP changes reload the database. A reconnect may have
// lost notifications, so it refreshes both. The channel closes on return.
func listenNotify(ctx context.Context, pool *pgxpool.Pool, hub *tunnel.Hub, reload func(context.Context) error, logger *slog.Logger) <-chan struct{} {
	geoip := geoIPReloader(ctx, reload, logger)
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = pgnotify.Listen(ctx, pool, map[string]func(string){pgnotify.OrgChanged: hub.OrgChanged, pgnotify.GeoIPChanged: geoip},
			func() { hub.Resync(); geoip("") })
	}()
	return done
}

// geoIPReloader returns a trigger that never blocks: one goroutine runs
// reload, one at a time, and a trigger during a reload runs one more.
func geoIPReloader(ctx context.Context, reload func(context.Context) error, logger *slog.Logger) func(string) {
	kick := make(chan struct{}, 1)
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-kick:
				if err := reload(ctx); err != nil && ctx.Err() == nil {
					logger.Warn("geoip reload failed", "err", err)
				}
			}
		}
	}()
	return func(string) {
		select {
		case kick <- struct{}{}:
		default:
		}
	}
}
