package gateway

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"path/filepath"
	"time"

	"github.com/google/uuid"

	"github.com/werbot/werbot/internal/gateway/recorder"
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
// and /readyz, /metrics on httpLn (nil: off) until ctx ends or the tunnel stops for
// good (expired certificate). Session ends and recordings go through the spool in
// DataDir/spool. On stop it drains: new sessions are refused at once, running ones
// get cfg.DrainTimeout, and the spool gets up to 10s to deliver their ends; what is
// left stays on disk for the next start. The tunnel error, if any, is returned after
// the drain.
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
	// Gauge, not GaugeFunc: the embedded gateway runs serve again on the same registry
	gauge := cfg.Metrics.Gauge("werbot_gateway_spool_bytes", "Unsent recording bytes in the gateway spool.")
	sp, err := recorder.OpenSpool(filepath.Join(cfg.DataDir, "spool"), tc, recorder.SpoolOptions{Gauge: gauge}, logger)
	if err != nil {
		return err
	}
	var p *Pipeline
	if cfg.Recording == nil { // tests bring their own
		cfg.Recording = func(id uuid.UUID, cols, rows int) RecordingSink { // called by Pipeline.Open, after p is set
			return recorder.New(id, cols, rows, sp, recorder.Options{Terminate: p.active.Terminate, Logger: logger})
		}
	}
	p = newPipeline(tc, cfg, logger)
	p.spool = sp
	tc.onTerminate = func(id uuid.UUID) { p.active.Terminate(id, "terminated") } // a gateway cut: no exit status
	spoolCtx, stopSpool := context.WithCancel(context.WithoutCancel(ctx))
	spoolDone := make(chan struct{})
	go func() {
		defer close(spoolDone)
		_ = sp.Run(spoolCtx) // nil once spoolCtx ends
	}()
	// the sweeper ends sessions and the prober checks targets until the drain is over,
	// then both stop before the spool closes
	sweepCtx, stopSweep := context.WithCancel(context.WithoutCancel(ctx))
	sweepDone, probeDone := make(chan struct{}), make(chan struct{})
	go func() {
		defer close(sweepDone)
		p.sweep(sweepCtx)
	}()
	go func() {
		defer close(probeDone)
		p.health.run(sweepCtx, st.InstanceID, tc.poolInstances)
	}()

	// the tunnel outlives ctx: the spool still sends EndSession through it after the drain
	tunnelCtx, stopTunnel := context.WithCancel(context.WithoutCancel(ctx))
	tunnelDone := make(chan struct{})
	var tunnelErr error // read after tunnelDone
	go func() {
		defer close(tunnelDone)
		tunnelErr = tc.Run(tunnelCtx, p.heartbeat) // nil once tunnelCtx ends
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

	// the front stops only after /readyz turned 503, so balancers see it first
	frontCtx, stopFront := context.WithCancel(context.WithoutCancel(ctx))
	defer stopFront()
	frontDone, drained := make(chan struct{}), make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
		case <-frontDone: // a failed front drains too
		case <-tunnelDone: // so does a tunnel that cannot reconnect
		}
		p.active.startDrain()
		stopFront()
		p.drain(cfg.DrainTimeout)
		close(drained)
	}()
	logger.Info("gateway: serving", "ssh_addr", sshLn.Addr().String(), "instance_id", st.InstanceID)
	err = newSSHFront(cfg, hostKey, logger).Serve(frontCtx, wrapListener(sshLn, cfg.ProxyProtocol), p)
	close(frontDone)
	<-drained
	stopSweep()
	<-sweepDone
	<-probeDone

	select {
	case <-tunnelDone: // the tunnel stopped for good: nothing gets through
	default:
		waitEnds(sp, p.flushTimeout)
	}
	stopSpool()
	<-spoolDone
	if err := sp.Shutdown(); err != nil { // a session that ignored the drain cannot write into the next serve's spool
		logger.Warn("gateway: close spool", "error", err)
	}
	stopTunnel()
	<-tunnelDone
	if hs != nil {
		// Close, not Shutdown: the drain is over, and Shutdown waits up to 5s for a
		// probe connection that was opened but never sent a request (StateNew).
		_ = hs.Close()
	}
	return errors.Join(err, tunnelErr)
}

// waitEnds waits up to d for the spool to deliver every pending session end.
func waitEnds(sp *recorder.Spool, d time.Duration) {
	deadline := time.Now().Add(d)
	tick := time.NewTicker(20 * time.Millisecond)
	defer tick.Stop()
	for len(sp.PendingEnds()) > 0 && time.Now().Before(deadline) {
		<-tick.C
	}
}
