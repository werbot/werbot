// Package app is the composition root of the werbot binary.
package app

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"sync/atomic"
	"time"

	"connectrpc.com/connect"
	"connectrpc.com/validate"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/werbot/werbot/gen/werbot/v1/werbotv1connect"
	"github.com/werbot/werbot/internal/agentic"
	"github.com/werbot/werbot/internal/identity"
	"github.com/werbot/werbot/internal/identity/oauth"
	"github.com/werbot/werbot/internal/inventory"
	"github.com/werbot/werbot/internal/platform/clientip"
	"github.com/werbot/werbot/internal/platform/config"
	"github.com/werbot/werbot/internal/platform/httpserver"
	"github.com/werbot/werbot/internal/platform/metrics"
	"github.com/werbot/werbot/internal/platform/secretbox"
	"github.com/werbot/werbot/internal/platform/tlsca"
	"github.com/werbot/werbot/internal/session"
	"github.com/werbot/werbot/internal/system"
	"github.com/werbot/werbot/internal/tunnel"
	"github.com/werbot/werbot/sdk"
)

// host is the Community sdk.Host: no feature is ever entitled.
type host struct {
	pool   *pgxpool.Pool
	mux    *http.ServeMux
	subs   *subscriptions
	module string
}

func (h host) DB() *pgxpool.Pool                      { return h.pool }
func (h host) Handle(pattern string, hd http.Handler) { h.mux.Handle(pattern, hd) }
func (host) Entitled(string) bool                     { return false }

func (h host) Subscribe(channel string, fn func(context.Context, string)) {
	h.subs.add(h.module, channel, fn)
}

// serveOpts are what Run hands serve besides the config: the tunnel CA, the
// recording signer and the OAuth signing key from bootstrap, the secret box it
// was opened with and, for --with-gateway, the environment of the embedded gateway.
type serveOpts struct {
	CA          *tlsca.CA
	Signer      *session.Signer
	OAuthKey    *oauth.SigningKey
	Box         *secretbox.Box
	WithGateway bool
	Getenv      func(string) string
}

// serve runs the main HTTP server (and the metrics server when cfg.MetricsAddr
// is set), the gateway tunnel on cfg.GatewayAddr, the relay on
// cfg.InternalAddr, the NOTIFY listener, the River client and, with
// o.WithGateway, the embedded gateway until ctx ends. Then it drains /readyz,
// lets the embedded gateway drain its sessions (it still needs the tunnel),
// closes the tunnel streams, stops the relay (its browser streams close and
// its replica row goes), shuts the HTTP servers down and stops River, each
// within 10s.
func serve(ctx context.Context, cfg config.Config, logger *slog.Logger, pool *pgxpool.Pool, modules []sdk.Module, ui fs.FS, o serveOpts) error {
	turl, err := tunnelURL(cfg.PublicURL, cfg.GatewayAddr)
	if err != nil {
		return err
	}
	reg := metrics.NewRegistry()
	reg.GaugeFunc("werbot_db_pool_acquired_conns", "Database pool connections in use.",
		func() int64 { return int64(pool.Stat().AcquiredConns()) })
	reg.GaugeFunc("werbot_db_pool_total_conns", "Database pool connections open.",
		func() int64 { return int64(pool.Stat().TotalConns()) })

	s, err := newServices(ctx, cfg, pool, o.Box, o.CA, o.Signer, reg, logger)
	if err != nil {
		return err
	}
	idh := identity.NewHandler(s.Identity)
	opts := connect.WithHandlerOptions(
		connect.WithInterceptors(validate.NewInterceptor(), logInternalErrors(logger), agentic.RestrictAgentTokens(s.Agents)),
		connect.WithReadMaxBytes(1<<20), // public RPCs: no body is read past 1 MiB
	)

	var draining atomic.Bool
	mux := http.NewServeMux()
	mux.Handle("/healthz", httpserver.Health())
	mux.Handle("/readyz", httpserver.Ready(pool.Ping, &draining))
	mux.Handle(werbotv1connect.NewSystemServiceHandler(system.New(cfg.GeoIPURL), opts))
	mux.Handle(werbotv1connect.NewAuthServiceHandler(idh, opts))
	mux.Handle(werbotv1connect.NewProfileServiceHandler(idh, opts))
	mux.Handle(werbotv1connect.NewSSHKeyServiceHandler(idh, opts))
	mux.Handle(werbotv1connect.NewMemberServiceHandler(idh, opts))
	mux.Handle(werbotv1connect.NewTeamServiceHandler(idh, opts))
	mux.Handle(werbotv1connect.NewMFAServiceHandler(idh, opts))
	mux.Handle(werbotv1connect.NewMFAChallengeServiceHandler(idh, opts))
	s.mount(mux, opts)
	ws := s.Relay.WebSocketHandler() // inside identity.Middleware and agentRoutes, like every route of mux
	mux.Handle("GET /api/terminal", ws)
	mux.Handle("GET /api/sessions/{id}/live", ws)
	bearer, err := mountOAuth(mux, cfg, pool, o.OAuthKey, s.Identity, oauthAgents{s.Agents, logger}, opts)
	if err != nil {
		return err
	}
	// Enroll returns the pool's private host key: opts log no bodies, only
	// the masked cause of CodeInternal.
	gw := inventory.NewGatewayRPC(s.Gateways, turl)
	mux.Handle(werbotv1connect.NewGatewayServiceHandler(gw, opts))
	mux.Handle(werbotv1connect.NewGatewayEnrollServiceHandler(gw, opts))
	subs := &subscriptions{}
	for _, m := range modules {
		h := host{pool: pool, mux: mux, subs: subs, module: m.Name}
		if m.Feature != "" && !h.Entitled(m.Feature) {
			logger.Info("module skipped: not entitled", "module", m.Name, "feature", m.Feature)
			continue
		}
		if err := m.Init(ctx, h); err != nil {
			return fmt.Errorf("module %s: %w", m.Name, err)
		}
	}
	if cfg.WebEmbedded {
		mux.Handle("/", httpserver.SPA(ui))
	} else {
		mux.Handle("/", http.NotFoundHandler())
	}

	servers := []*http.Server{httpserver.New(cfg.HTTPAddr, clientip.Middleware(cfg.TrustedProxies)(identity.Middleware(s.Identity, bearer)(agentRoutes(mux))))}
	if cfg.MetricsAddr != "" {
		mmux := http.NewServeMux()
		mmux.Handle("/metrics", reg.Handler())
		servers = append(servers, httpserver.New(cfg.MetricsAddr, mmux))
	}
	lns, err := httpserver.Listen(servers)
	if err != nil {
		return err
	}
	gln, err := net.Listen("tcp", cfg.GatewayAddr)
	if err != nil {
		closeAll(lns)
		return err
	}
	// Not ctx: cancelling it aborts running jobs; only Stop below ends River.
	if err := s.Jobs.Start(context.WithoutCancel(ctx)); err != nil {
		closeAll(append(lns, gln))
		return err
	}
	errc := make(chan error, len(servers)+1) // and the relay's
	for i, srv := range servers {
		go func() { errc <- srv.Serve(lns[i]) }()
	}
	// pctx ends the tunnel and the NOTIFY listener, after the embedded gateway
	// drained (its EndSession calls need the tunnel) or when a server fails.
	pctx, pcancel := context.WithCancel(context.WithoutCancel(ctx))
	defer pcancel()
	hub := tunnel.NewHub()
	plane := make(chan error, 1)
	go func() { plane <- runGatewayPlane(pctx, gln, cfg, s, hub, reg, logger) }()
	relayDone := startRelay(pctx, s.Relay, errc)
	listening := listenNotify(pctx, pool, hub, s.Policy.ReloadGeoIP, subs, logger)
	gctx, gcancel := context.WithCancel(ctx)
	defer gcancel()
	embedded := make(chan error, 1)
	if o.WithGateway {
		go func() { embedded <- embeddedGateway(gctx, cfg, o.Getenv, pool, s.Gateways, reg, logger) }()
	}
	logger.Info("server started", "addr", cfg.HTTPAddr, "metrics_addr", cfg.MetricsAddr, "gateway_addr", cfg.GatewayAddr,
		"with_gateway", o.WithGateway)

	planeDone, embeddedDone := false, !o.WithGateway
	select {
	case err = <-errc:
	case err = <-plane:
		planeDone = true
	case err = <-embedded:
		embeddedDone = true
		if err == nil && ctx.Err() == nil {
			err = errors.New("embedded gateway stopped")
		}
	case <-ctx.Done():
	}
	draining.Store(true)
	gcancel()
	if !embeddedDone {
		if gerr := <-embedded; gerr != nil {
			logger.Error("embedded gateway shutdown", "error", gerr)
		}
	}
	hub.Shutdown()
	pcancel()
	<-relayDone // its WebSockets closed and its row removed
	sctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancel()
	for _, srv := range servers {
		if serr := srv.Shutdown(sctx); serr != nil {
			logger.Error("server shutdown", "addr", srv.Addr, "error", serr)
		}
	}
	if !planeDone {
		if perr := <-plane; perr != nil {
			logger.Error("gateway tunnel shutdown", "addr", cfg.GatewayAddr, "error", perr)
		}
	}
	<-listening
	rctx, rcancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer rcancel()
	if rerr := s.Jobs.Stop(rctx); rerr != nil {
		logger.Error("river stop", "error", rerr)
	}
	return err
}

func closeAll(lns []net.Listener) {
	for _, ln := range lns {
		_ = ln.Close()
	}
}

// logInternalErrors logs the masked cause of every CodeInternal response.
func logInternalErrors(logger *slog.Logger) connect.UnaryInterceptorFunc {
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			res, err := next(ctx, req)
			if ce := (*connect.Error)(nil); errors.As(err, &ce) && ce.Code() == connect.CodeInternal {
				logger.Error("rpc failed", "procedure", req.Spec().Procedure, "err", errors.Unwrap(ce.Unwrap()))
			}
			return res, err
		}
	}
}
