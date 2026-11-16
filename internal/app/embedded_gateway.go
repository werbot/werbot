package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"path/filepath"
	"time"

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
// first organization (created by Setup).
const orgPollInterval = 5 * time.Second

// runEmbeddedGateway runs this replica's instance of the system pool "local"
// (werbot server --with-gateway) until ctx ends. It reaches the server over
// loopback: enrollment on the port of cfg.HTTPAddr, the tunnel on the port of
// cfg.GatewayAddr. An unenrolled instance waits for the first organization,
// then joins with a single-use system token. It returns nil when ctx ends
// before enrollment.
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
	if !gateway.Enrolled(gcfg.DataDir) {
		org, err := firstOrganization(ctx, pool)
		if err != nil || org == uuid.Nil {
			return err // nil: ctx ended while waiting
		}
		if gcfg.JoinToken, err = gws.CreateSystemJoinToken(ctx, org); err != nil {
			return fmt.Errorf("embedded gateway: %w", err)
		}
	}
	return gateway.Run(ctx, gcfg, logger)
}

// loopbackURL is scheme://127.0.0.1:<port of addr>.
func loopbackURL(scheme, addr string) (string, error) {
	_, port, err := net.SplitHostPort(addr)
	if err != nil || port == "" {
		return "", fmt.Errorf("embedded gateway: want [host]:port, got %q", addr)
	}
	return scheme + "://" + net.JoinHostPort("127.0.0.1", port), nil
}

// firstOrganization polls every orgPollInterval until an organization exists
// and returns the oldest one; uuid.Nil and no error when ctx ends first.
func firstOrganization(ctx context.Context, pool *pgxpool.Pool) (uuid.UUID, error) {
	t := time.NewTicker(orgPollInterval)
	defer t.Stop()
	for {
		var id uuid.UUID
		err := dbx.System(ctx, pool, dbx.ReasonBootstrap, func(tx pgx.Tx) error {
			return tx.QueryRow(ctx, "SELECT id FROM organization ORDER BY created_at LIMIT 1").Scan(&id)
		})
		switch {
		case ctx.Err() != nil:
			return uuid.Nil, nil
		case err == nil:
			return id, nil
		case !errors.Is(err, pgx.ErrNoRows):
			return uuid.Nil, fmt.Errorf("embedded gateway: %w", err)
		}
		select {
		case <-ctx.Done():
			return uuid.Nil, nil
		case <-t.C:
		}
	}
}
