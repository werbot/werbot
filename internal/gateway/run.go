package gateway

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/werbot/werbot/internal/platform/metrics"
)

// Serve runs the gateway with the SSH front on ln and no HTTP endpoint.
func Serve(ctx context.Context, cfg Config, ln net.Listener, logger *slog.Logger) error {
	return serve(ctx, cfg, ln, nil, logger)
}

// Run listens on cfg.SSHAddr and, when set, cfg.HTTPAddr, then serves until ctx ends.
func Run(ctx context.Context, cfg Config, logger *slog.Logger) error {
	sshLn, err := net.Listen("tcp", cfg.SSHAddr)
	if err != nil {
		return fmt.Errorf("gateway ssh listen: %w", err)
	}
	var httpLn net.Listener
	if cfg.HTTPAddr != "" {
		if httpLn, err = net.Listen("tcp", cfg.HTTPAddr); err != nil {
			_ = sshLn.Close()
			return fmt.Errorf("gateway http listen: %w", err)
		}
	}
	return serve(ctx, cfg, sshLn, httpLn, logger)
}

// serve enrolls (or loads the identity), keeps the tunnel and serves SSH on sshLn
// and /readyz, /metrics on httpLn (nil: off) until ctx ends. Then it drains: new
// sessions are refused at once, running ones get cfg.DrainTimeout, and the tunnel
// stays up until their EndSession calls are delivered.
func serve(ctx context.Context, cfg Config, sshLn, httpLn net.Listener, logger *slog.Logger) error {
	defer sshLn.Close() //nolint:errcheck // closed twice on the normal path
	if httpLn != nil {
		defer httpLn.Close() //nolint:errcheck // closed twice on the normal path
	}
	if cfg.Metrics == nil {
		cfg.Metrics = metrics.NewRegistry() // shared by the pipeline and the tunnel
	}
	st, certs, roots, err := ensureEnrolled(ctx, cfg, &http.Client{Timeout: 30 * time.Second}, connectors())
	if err != nil {
		return err
	}
	hostKey, err := loadHostKey(cfg.DataDir)
	if err != nil {
		return err
	}
	tc := newTunnelClient(st, roots, certs, cfg.Metrics, logger)
	p := newPipeline(tc, cfg, logger)

	// the tunnel outlives ctx: drain still sends EndSession through it
	tunnelCtx, stopTunnel := context.WithCancel(context.WithoutCancel(ctx))
	tunnelDone := make(chan struct{})
	go func() {
		defer close(tunnelDone)
		_ = tc.Run(tunnelCtx, p.heartbeat) // returns nil once tunnelCtx ends
	}()

	var hs *http.Server
	if httpLn != nil {
		hs = &http.Server{Handler: newHTTPHandler(p, cfg.Metrics), ReadHeaderTimeout: 5 * time.Second}
		go func() {
			if err := hs.Serve(httpLn); !errors.Is(err, http.ErrServerClosed) {
				logger.Error("gateway: http server failed", "error", err)
			}
		}()
	}

	frontCtx, stopFront := context.WithCancel(ctx)
	drained := make(chan struct{})
	go func() {
		<-frontCtx.Done()
		p.drain(cfg.DrainTimeout)
		close(drained)
	}()
	logger.Info("gateway: serving", "ssh_addr", sshLn.Addr().String(), "instance_id", st.InstanceID)
	err = newSSHFront(cfg, hostKey, logger).Serve(frontCtx, wrapListener(sshLn, cfg.ProxyProtocol), p)
	stopFront() // a failed front drains too
	<-drained

	stopTunnel()
	<-tunnelDone
	if hs != nil {
		sctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = hs.Shutdown(sctx)
	}
	return err
}
