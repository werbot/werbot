package relay

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/werbot/werbot/internal/platform/apperr"
	"github.com/werbot/werbot/internal/platform/dbx"
)

// register upserts this replica's server_instance row; a heartbeat after GC
// deleted the row registers it again.
func (r *Relay) register(ctx context.Context, host string) error {
	err := dbx.System(ctx, r.pool, dbx.ReasonCluster, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `INSERT INTO server_instance (id, hostname, internal_url, version) VALUES ($1, $2, $3, $4)
ON CONFLICT (id) DO UPDATE SET last_seen_at = now(), internal_url = EXCLUDED.internal_url`,
			r.cfg.InstanceID, host, r.cfg.InternalURL, r.cfg.Version)
		return err
	})
	if err != nil {
		return fmt.Errorf("relay: register: %w", err)
	}
	r.registered.Store(true)
	return nil
}

// deregister removes this replica's row, even after ctx ended.
func (r *Relay) deregister(ctx context.Context) {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	err := dbx.System(ctx, r.pool, dbx.ReasonCluster, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, "DELETE FROM server_instance WHERE id = $1", r.cfg.InstanceID)
		return err
	})
	if err != nil {
		r.log.Warn("relay: deregister failed", "error", err)
	}
}

// known reports whether replica id has a server_instance row (online or not).
func (r *Relay) known(ctx context.Context, id uuid.UUID) (bool, error) {
	var ok bool
	err := dbx.System(ctx, r.pool, dbx.ReasonCluster, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM server_instance WHERE id = $1)", id).Scan(&ok)
	})
	return ok, err
}

// onlineURL returns the internal_url of replica id when it sent a heartbeat
// within OnlineWindow; otherwise apperr.ErrUnavailable.
func (r *Relay) onlineURL(ctx context.Context, id uuid.UUID) (string, error) {
	var u string
	err := dbx.System(ctx, r.pool, dbx.ReasonCluster, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, `SELECT internal_url FROM server_instance
WHERE id = $1 AND last_seen_at > now() - make_interval(secs => $2)`, id, OnlineWindow.Seconds()).Scan(&u)
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return "", fmt.Errorf("%w: replica is offline", apperr.ErrUnavailable)
	}
	return u, err
}
