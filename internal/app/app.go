// Package app is the composition root of the werbot binary.
package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"sync/atomic"
	"time"

	"connectrpc.com/connect"
	"connectrpc.com/validate"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"

	"github.com/werbot/werbot/gen/werbot/v1/werbotv1connect"
	"github.com/werbot/werbot/internal/identity"
	"github.com/werbot/werbot/internal/notify"
	"github.com/werbot/werbot/internal/platform/clientip"
	"github.com/werbot/werbot/internal/platform/config"
	"github.com/werbot/werbot/internal/platform/db"
	"github.com/werbot/werbot/internal/platform/httpserver"
	"github.com/werbot/werbot/internal/platform/jobs"
	"github.com/werbot/werbot/internal/platform/logging"
	"github.com/werbot/werbot/internal/platform/metrics"
	"github.com/werbot/werbot/internal/platform/secretbox"
	"github.com/werbot/werbot/internal/platform/version"
	"github.com/werbot/werbot/internal/system"
	"github.com/werbot/werbot/internal/webui"
	"github.com/werbot/werbot/sdk"
	"github.com/werbot/werbot/web"
)

// Run executes the command in args and returns when it is done or ctx ends.
func Run(ctx context.Context, args []string, getenv func(string) string, stdout io.Writer) error {
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
	case "migrate", "server":
	default:
		return errors.New("usage: werbot <server|web|migrate|version>")
	}

	cfg, err := config.Load(getenv)
	if err != nil {
		return err
	}
	if cmd == "migrate" {
		return bootstrap(ctx, cfg.DatabaseURL)
	}

	logger := logging.New(stdout, cfg.LogLevel, cfg.LogFormat)
	if err := bootstrap(ctx, cfg.DatabaseURL); err != nil {
		return err
	}
	octx, cancel := context.WithTimeout(ctx, 10*time.Second)
	pool, err := db.Open(octx, cfg.DatabaseURL)
	cancel()
	if err != nil {
		return err
	}
	defer pool.Close()
	return serve(ctx, cfg, logger, pool, sdk.Modules(), web.FS())
}

// host is the Community sdk.Host: no feature is ever entitled.
type host struct {
	pool *pgxpool.Pool
	mux  *http.ServeMux
}

func (h host) DB() *pgxpool.Pool                      { return h.pool }
func (h host) Handle(pattern string, hd http.Handler) { h.mux.Handle(pattern, hd) }
func (host) Entitled(string) bool                     { return false }

// serve runs the main HTTP server (and the metrics server when cfg.MetricsAddr
// is set) and the River client until ctx ends, then drains and shuts the HTTP
// servers down, then stops River, each within 10s.
func serve(ctx context.Context, cfg config.Config, logger *slog.Logger, pool *pgxpool.Pool, modules []sdk.Module, ui fs.FS) error {
	box, err := secretbox.New(cfg.SecretKey, cfg.PreviousSecretKeys)
	if err != nil {
		return err
	}
	mailer, err := notify.NewMailer(cfg.SMTP, cfg.Env == "development", logger)
	if err != nil {
		return err
	}
	workers := river.NewWorkers()
	river.AddWorker(workers, notify.NewWorker(mailer, box))
	rc, err := jobs.NewClient(pool, workers, nil, logger)
	if err != nil {
		return err
	}
	ids := identity.New(pool, notify.NewQueue(rc, box), identity.Config{
		WebURL: cfg.WebURL, SecureCookies: cfg.Env != "development", Logger: logger,
	})
	idh := identity.NewHandler(ids)
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

	servers := []*http.Server{httpserver.New(cfg.HTTPAddr, clientip.Middleware(cfg.TrustedProxies)(identity.Middleware(ids)(mux)))}
	if cfg.MetricsAddr != "" {
		mmux := http.NewServeMux()
		mmux.Handle("/metrics", reg.Handler())
		servers = append(servers, httpserver.New(cfg.MetricsAddr, mmux))
	}
	lns, err := httpserver.Listen(servers)
	if err != nil {
		return err
	}
	// Not ctx: cancelling it aborts running jobs; only Stop below ends River.
	if err := rc.Start(context.WithoutCancel(ctx)); err != nil {
		for _, ln := range lns {
			_ = ln.Close()
		}
		return err
	}
	errc := make(chan error, len(servers))
	for i, s := range servers {
		go func() { errc <- s.Serve(lns[i]) }()
	}
	logger.Info("server started", "addr", cfg.HTTPAddr, "metrics_addr", cfg.MetricsAddr)

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
	rctx, rcancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer rcancel()
	if rerr := rc.Stop(rctx); rerr != nil {
		logger.Error("river stop", "error", rerr)
	}
	return err
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
