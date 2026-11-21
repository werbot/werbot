package app

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/stdlib"

	"github.com/werbot/werbot/internal/identity/oauth"
	"github.com/werbot/werbot/internal/platform/db"
	"github.com/werbot/werbot/internal/platform/jobs"
	"github.com/werbot/werbot/internal/platform/secretbox"
	"github.com/werbot/werbot/internal/platform/tlsca"
	"github.com/werbot/werbot/internal/session"
	"github.com/werbot/werbot/migrations"
)

// bootstrapped is what werbot server takes from bootstrap.
type bootstrapped struct {
	CA       *tlsca.CA
	Signer   *session.Signer
	OAuthKey *oauth.SigningKey
}

// bootstrap is the only place for one-off startup work (contract §1.6). It runs
// the steps under db.BootstrapLock, so replicas starting together take turns:
// waiting for the lock is bounded only by ctx, and a second replica sees the
// finished schema instead of failing. Later stages append steps to the closure
// in this order: (1) goose Up, (2) jobs.Migrate, (3) tlsca.LoadOrCreate,
// (4) session.LoadSigner, (5) oauth.LoadSigningKey. box == nil is werbot
// migrate: steps (1)-(2) only.
func bootstrap(ctx context.Context, databaseURL string, box *secretbox.Box) (bootstrapped, error) {
	octx, cancel := context.WithTimeout(ctx, 10*time.Second)
	bpool, err := db.OpenBootstrap(octx, databaseURL)
	cancel()
	if err != nil {
		return bootstrapped{}, fmt.Errorf("bootstrap: %w", err)
	}
	defer bpool.Close()
	var out bootstrapped
	err = db.WithLock(ctx, bpool, db.BootstrapLock, func(ctx context.Context) error {
		// (1) goose Up. sqlDB is closed before the closure returns, otherwise
		// bpool.Close waits for its connection.
		sqlDB := stdlib.OpenDBFromPool(bpool)
		err := db.Migrate(ctx, sqlDB, migrations.FS)
		if cerr := sqlDB.Close(); err == nil {
			err = cerr
		}
		if err != nil {
			return fmt.Errorf("bootstrap: %w", err)
		}
		// (2) River schema. Shares bpool (MaxConns 2: lock + this connection).
		if err := jobs.Migrate(ctx, bpool); err != nil {
			return fmt.Errorf("bootstrap: river migrate: %w", err)
		}
		if box == nil {
			return nil
		}
		// (3) gateway tunnel CA: the first replica creates it, the others load it.
		if out.CA, err = tlsca.LoadOrCreate(ctx, tlsca.PostgresStore(bpool), box); err != nil {
			return fmt.Errorf("bootstrap: %w", err)
		}
		// (4) recording manifest signing key: created once, loaded by every replica.
		if out.Signer, err = session.LoadSigner(ctx, bpool, box); err != nil {
			return fmt.Errorf("bootstrap: %w", err)
		}
		// (5) OAuth token signing key: created once, loaded by every replica.
		if out.OAuthKey, err = oauth.LoadSigningKey(ctx, bpool, box); err != nil {
			return fmt.Errorf("bootstrap: %w", err)
		}
		return nil
	})
	if err != nil {
		return bootstrapped{}, err
	}
	return out, nil
}
