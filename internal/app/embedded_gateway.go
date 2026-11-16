package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"path/filepath"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/werbot/werbot/internal/gateway"
	"github.com/werbot/werbot/internal/inventory"
	"github.com/werbot/werbot/internal/platform/config"
	"github.com/werbot/werbot/internal/platform/dbx"
	"github.com/werbot/werbot/internal/platform/metrics"
)

// orgPollInterval is how often an unenrolled embedded gateway looks for the
// first organization (created by Setup) and the first wait before it retries
// a failed enrollment; tests shorten it.
var orgPollInterval = 5 * time.Second

// maxEnrollRetry caps the doubling wait between failed enrollments. The token
// is reused until the server rejects it or it is half way to expiry.
const maxEnrollRetry = 5 * time.Minute

// tokenRefresh is the age at which an unused join token is replaced; tests
// shorten it.
var tokenRefresh = inventory.SystemJoinTokenTTL / 2

// runGateway runs the gateway instance; tests replace it.
var runGateway = gateway.Run

// embeddedGateway is what serve runs for --with-gateway; tests replace it.
var embeddedGateway = runEmbeddedGateway

// runEmbeddedGateway runs this replica's instance of the system pool "local"
// (werbot server --with-gateway) until ctx ends. It reaches the server over
// loopback: enrollment on the port of cfg.HTTPAddr, the tunnel on the port of
// cfg.GatewayAddr. An unenrolled instance waits for the first organization,
// then joins with a single-use system token. Database and enrollment errors
// before the instance is enrolled are logged and retried; a listen error or
// a failure after enrollment is returned. It returns nil when ctx ends.
func runEmbeddedGateway(ctx context.Context, cfg config.Config, getenv func(string) string, pool *pgxpool.Pool,
	gws *inventory.Gateways, reg *metrics.Registry, logger *slog.Logger) error {
	gcfg, err := gateway.LoadConfig(nil, getenv)
	if err != nil {
		return err
	}
	if gcfg.ServerURL, err = loopbackURL("http", cfg.HTTPAddr); err != nil {
		return err
	}
	if gcfg.TunnelURL, err = loopbackURL("https", cfg.GatewayAddr); err != nil {
		return err
	}
	if getenv("WERBOT_GATEWAY_DATA_DIR") == "" {
		gcfg.DataDir = filepath.Join(cfg.DataDir, "gateway") // one identity per replica
	}
	gcfg.Metrics = reg
	var minted time.Time // when gcfg.JoinToken was issued; zero without one
	for wait := orgPollInterval; ; wait = min(2*wait, maxEnrollRetry) {
		if !gateway.Enrolled(gcfg.DataDir) && (gcfg.JoinToken == "" || time.Since(minted) > tokenRefresh) {
			org := firstOrganization(ctx, pool, logger)
			if org == uuid.Nil {
				return nil // ctx ended while waiting
			}
			if gcfg.JoinToken, err = gws.CreateSystemJoinToken(ctx, org); err != nil {
				gcfg.JoinToken = ""
				logger.Warn("embedded gateway: join token failed, retrying", "error", err, "retry_in", wait)
				if !sleep(ctx, wait) {
					return nil
				}
				continue
			}
			minted = time.Now()
		}
		err = runGateway(ctx, gcfg, logger)
		expired := errors.Is(err, gateway.ErrCertExpired)
		if expired || connect.CodeOf(err) == connect.CodePermissionDenied {
			gcfg.JoinToken = "" // spent or rejected, or a new identity is needed: mint a new one
		}
		if ctx.Err() != nil || gateway.Enrolled(gcfg.DataDir) || isListenError(err) {
			return err
		}
		if expired {
			logger.Warn("embedded gateway: certificate expired, enrolling again", "retry_in", wait)
		} else {
			logger.Warn("embedded gateway: enrollment failed, retrying", "error", err, "retry_in", wait)
		}
		if !sleep(ctx, wait) {
			return nil
		}
	}
}

// isListenError reports whether err comes from binding a listener (port in
// use, bad address): retrying does not help.
func isListenError(err error) bool {
	var op *net.OpError
	return errors.As(err, &op) && op.Op == "listen"
}

// sleep waits d and reports false when ctx ended first.
func sleep(ctx context.Context, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}

// loopbackURL is scheme://127.0.0.1:<port of addr>.
func loopbackURL(scheme, addr string) (string, error) {
	_, port, err := net.SplitHostPort(addr)
	if err != nil || port == "" {
		return "", fmt.Errorf("embedded gateway: want [host]:port, got %q", addr)
	}
	return scheme + "://" + net.JoinHostPort("127.0.0.1", port), nil
}

// firstOrganization checks every orgPollInterval until an organization exists
// and returns the oldest one; database errors are logged and retried. It
// returns uuid.Nil when ctx ends first.
func firstOrganization(ctx context.Context, pool *pgxpool.Pool, logger *slog.Logger) uuid.UUID {
	for {
		var id uuid.UUID
		err := dbx.System(ctx, pool, dbx.ReasonBootstrap, func(tx pgx.Tx) error {
			return tx.QueryRow(ctx, "SELECT id FROM organization ORDER BY created_at LIMIT 1").Scan(&id)
		})
		switch {
		case ctx.Err() != nil:
			return uuid.Nil
		case err == nil:
			return id
		case !errors.Is(err, pgx.ErrNoRows):
			logger.Warn("embedded gateway: organization lookup failed, retrying", "error", err)
		}
		if !sleep(ctx, orgPollInterval) {
			return uuid.Nil
		}
	}
}
