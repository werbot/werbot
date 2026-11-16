// Package pgnotify listens to several Postgres NOTIFY channels on one
// dedicated connection kept outside the pool.
package pgnotify

import (
	"context"
	"fmt"
	"log/slog"
	"maps"
	"slices"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// NOTIFY channels: OrgChanged carries an org id or "*" (all orgs), GeoIPChanged has no payload.
const (
	OrgChanged   = "werbot_org_changed"
	GeoIPChanged = "werbot_geoip_changed"
)

const (
	minBackoff = time.Second
	maxBackoff = 30 * time.Second
)

// closeTimeout bounds the connection close: a stalled socket must not hang session.
var closeTimeout = 5 * time.Second

func closeConn(ctx context.Context, conn *pgx.Conn) {
	cctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), closeTimeout)
	defer cancel()
	_ = conn.Close(cctx)
}

// Listen holds one dedicated connection outside the pool with LISTEN on every
// channel of handlers. onReconnect runs after each successful LISTEN of all
// channels (the first one included): notifications sent during a gap may be lost.
// handlers values and onReconnect must be non-nil.
// Handlers run sequentially in the Listen goroutine and must be fast.
// Returns nil after ctx.Done.
func Listen(ctx context.Context, pool *pgxpool.Pool, handlers map[string]func(payload string), onReconnect func()) error {
	channels := slices.Sorted(maps.Keys(handlers))
	backoff := minBackoff
	for ctx.Err() == nil {
		err := session(ctx, pool, channels, handlers, onReconnect, &backoff)
		if ctx.Err() != nil {
			return nil
		}
		slog.Warn("pgnotify: connection lost", "err", err, "retry_in", backoff)
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(backoff):
		}
		backoff = min(backoff*2, maxBackoff)
	}
	return nil
}

// session runs one connection until it fails; backoff is reset once LISTEN succeeded.
func session(ctx context.Context, pool *pgxpool.Pool, channels []string, handlers map[string]func(string), onReconnect func(), backoff *time.Duration) error {
	conn, err := pgx.ConnectConfig(ctx, pool.Config().ConnConfig.Copy())
	if err != nil {
		return err
	}
	defer closeConn(ctx, conn)

	for _, ch := range channels {
		if _, err := conn.Exec(ctx, "LISTEN "+pgx.Identifier{ch}.Sanitize()); err != nil {
			return fmt.Errorf("listen %s: %w", ch, err)
		}
	}
	*backoff = minBackoff
	onReconnect()

	for {
		n, err := conn.WaitForNotification(ctx)
		if err != nil {
			return err
		}
		handlers[n.Channel](n.Payload)
	}
}
