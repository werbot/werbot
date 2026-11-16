// Package app is the composition root of the werbot binary.
package app

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
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
	"github.com/werbot/werbot/internal/gateway"
	_ "github.com/werbot/werbot/internal/gateway/sshconnector" // registers the ssh gateway connector
	"github.com/werbot/werbot/internal/identity"
	"github.com/werbot/werbot/internal/inventory"
	"github.com/werbot/werbot/internal/platform/clientip"
	"github.com/werbot/werbot/internal/platform/config"
	"github.com/werbot/werbot/internal/platform/db"
	"github.com/werbot/werbot/internal/platform/httpserver"
	"github.com/werbot/werbot/internal/platform/logging"
	"github.com/werbot/werbot/internal/platform/metrics"
	"github.com/werbot/werbot/internal/platform/secretbox"
	"github.com/werbot/werbot/internal/platform/tlsca"
	"github.com/werbot/werbot/internal/platform/version"
	"github.com/werbot/werbot/internal/system"
	"github.com/werbot/werbot/internal/tunnel"
	"github.com/werbot/werbot/internal/webui"
	"github.com/werbot/werbot/sdk"
	"github.com/werbot/werbot/web"
)

// Run executes the command in args and returns when it is done or ctx ends.
// A -help flag is not an error.
func Run(ctx context.Context, args []string, getenv func(string) string, stdout io.Writer) error {
	if err := run(ctx, args, getenv, stdout); !errors.Is(err, flag.ErrHelp) {
		return err
	}
	return nil
}

func run(ctx context.Context, args []string, getenv func(string) string, stdout io.Writer) error {
	cmd := ""
	if len(args) > 0 {
		cmd = args[0]
	}
	switch cmd {
	case "version":
		_, err := fmt.Fprintf(stdout, "werbot %s (commit %s, built %s)\n", version.Version, version.Commit, version.BuildDate)
		return err
	case "web":
		cfg, err := webui.LoadConfig(getenv)
		if err != nil {
			return err
		}
		return webui.Run(ctx, cfg, web.FS(), logging.New(stdout, cfg.LogLevel, cfg.LogFormat))
	case "gateway":
		gcfg, err := gateway.LoadConfig(args[1:], getenv)
		if err != nil {
			return err
		}
		level, format, err := config.LoadLogging(getenv)
		if err != nil {
			return err
		}
		return gateway.Run(ctx, gcfg, logging.New(stdout, level, format))
	case "migrate", "server":
	default:
		return errors.New("usage: werbot <server|gateway|web|migrate|version>")
	}
	var withGateway bool
	if cmd == "server" {
		fs := flag.NewFlagSet("server", flag.ContinueOnError)
		fs.BoolVar(&withGateway, "with-gateway", false, `also run an instance of the system gateway pool "local"`)
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if fs.NArg() > 0 {
			return fmt.Errorf("unexpected arguments: %q", fs.Args())
		}
	}

	cfg, err := config.Load(getenv)
	if err != nil {
		return err
	}
	if cmd == "migrate" {
		_, err := bootstrap(ctx, cfg.DatabaseURL, nil)
		return err
	}

	logger := logging.New(stdout, cfg.LogLevel, cfg.LogFormat)
	box, err := secretbox.New(cfg.SecretKey, cfg.PreviousSecretKeys)
	if err != nil {
		return err
	}
	boot, err := bootstrap(ctx, cfg.DatabaseURL, box)
	if err != nil {
		return err
	}
	octx, cancel := context.WithTimeout(ctx, 10*time.Second)
	pool, err := db.Open(octx, cfg.DatabaseURL)
	cancel()
	if err != nil {
		return err
	}
	defer pool.Close()
	return serve(ctx, cfg, logger, pool, sdk.Modules(), web.FS(), serveOpts{CA: boot.CA, Box: box, WithGateway: withGateway, Getenv: getenv})
}

// host is the Community sdk.Host: no feature is ever entitled.
type host struct {
	pool *pgxpool.Pool
	mux  *http.ServeMux
}

func (h host) DB() *pgxpool.Pool                      { return h.pool }
func (h host) Handle(pattern string, hd http.Handler) { h.mux.Handle(pattern, hd) }
func (host) Entitled(string) bool                     { return false }

// serveOpts are what Run hands serve besides the config: the tunnel CA from
// bootstrap, the secret box it was opened with and, for --with-gateway, the
// environment of the embedded gateway.
type serveOpts struct {
	CA          *tlsca.CA
	Box         *secretbox.Box
	WithGateway bool
	Getenv      func(string) string
}

// serve runs the main HTTP server (and the metrics server when cfg.MetricsAddr
// is set), the gateway tunnel on cfg.GatewayAddr, the NOTIFY listener, the
// River client and, with o.WithGateway, the embedded gateway until ctx ends.
// Then it drains /readyz, lets the embedded gateway drain its sessions (it
// still needs the tunnel), closes the tunnel streams, shuts the HTTP servers
// down and stops River, each within 10s.
func serve(ctx context.Context, cfg config.Config, logger *slog.Logger, pool *pgxpool.Pool, modules []sdk.Module, ui fs.FS, o serveOpts) error {
	turl, err := tunnelURL(cfg.PublicURL, cfg.GatewayAddr)
	if err != nil {
		return err
	}
	s, err := newServices(ctx, cfg, pool, o.Box, o.CA, logger)
	if err != nil {
		return err
	}
	idh := identity.NewHandler(s.Identity)
	opts := connect.WithHandlerOptions(
		connect.WithInterceptors(validate.NewInterceptor(), logInternalErrors(logger)),
		connect.WithReadMaxBytes(1<<20), // public RPCs: no body is read past 1 MiB
	)

	var draining atomic.Bool
	mux := http.NewServeMux()
	mux.Handle("/healthz", httpserver.Health())
	mux.Handle("/readyz", httpserver.Ready(pool.Ping, &draining))
	mux.Handle(werbotv1connect.NewSystemServiceHandler(system.New(), opts))
	mux.Handle(werbotv1connect.NewAuthServiceHandler(idh, opts))
	mux.Handle(werbotv1connect.NewProfileServiceHandler(idh, opts))
	mux.Handle(werbotv1connect.NewSSHKeyServiceHandler(idh, opts))
	mux.Handle(werbotv1connect.NewMemberServiceHandler(idh, opts))
	mux.Handle(werbotv1connect.NewTeamServiceHandler(idh, opts))
	s.mount(mux, opts)
	// Enroll returns the pool's private host key: opts log no bodies, only
	// the masked cause of CodeInternal.
	gw := inventory.NewGatewayRPC(s.Gateways, turl)
	mux.Handle(werbotv1connect.NewGatewayServiceHandler(gw, opts))
	mux.Handle(werbotv1connect.NewGatewayEnrollServiceHandler(gw, opts))
	h := host{pool: pool, mux: mux}
	for _, m := range modules {
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

	reg := metrics.NewRegistry()
	reg.GaugeFunc("werbot_db_pool_acquired_conns", "Database pool connections in use.",
		func() int64 { return int64(pool.Stat().AcquiredConns()) })
	reg.GaugeFunc("werbot_db_pool_total_conns", "Database pool connections open.",
		func() int64 { return int64(pool.Stat().TotalConns()) })

	servers := []*http.Server{httpserver.New(cfg.HTTPAddr, clientip.Middleware(cfg.TrustedProxies)(identity.Middleware(s.Identity)(mux)))}
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
	errc := make(chan error, len(servers))
	for i, srv := range servers {
		go func() { errc <- srv.Serve(lns[i]) }()
	}
	// pctx ends the tunnel and the NOTIFY listener also when a server fails.
	pctx, pcancel := context.WithCancel(ctx)
	defer pcancel()
	hub := tunnel.NewHub()
	plane := make(chan error, 1)
	go func() { plane <- runGatewayPlane(pctx, gln, cfg, s, hub, reg, logger) }()
	listening := listenNotify(pctx, pool, hub, s.Policy.ReloadGeoIP, logger)
	gctx, gcancel := context.WithCancel(ctx)
	defer gcancel()
	embedded := make(chan error, 1)
	if o.WithGateway {
		go func() { embedded <- runEmbeddedGateway(gctx, cfg, o.Getenv, pool, s.Gateways, reg, logger) }()
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
