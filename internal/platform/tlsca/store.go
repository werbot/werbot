package tlsca

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/werbot/werbot/internal/platform/dbx"
)

// StoredCA is the persisted CA: the PKCS#8 key sealed by secretbox plus the root certificate.
type StoredCA struct {
	Ciphertext []byte
	KeyID      string
	CertPEM    []byte
}

// CAStore persists the single CA row.
type CAStore interface {
	Load(ctx context.Context) (StoredCA, bool, error)
	// Insert is a no-op when a CA already exists (first insert wins).
	Insert(ctx context.Context, s StoredCA) error
}

type pgStore struct{ pool *pgxpool.Pool }

// PostgresStore keeps the CA in the ca_key table.
func PostgresStore(pool *pgxpool.Pool) CAStore { return pgStore{pool} }

func (s pgStore) Load(ctx context.Context) (StoredCA, bool, error) {
	var st StoredCA
	found := true
	err := dbx.System(ctx, s.pool, dbx.ReasonBootstrap, func(tx pgx.Tx) error {
		err := tx.QueryRow(ctx, "SELECT ciphertext, key_id, cert_pem FROM ca_key WHERE id = 1").
			Scan(&st.Ciphertext, &st.KeyID, &st.CertPEM)
		if errors.Is(err, pgx.ErrNoRows) {
			found = false
			return nil
		}
		return err
	})
	if err != nil {
		return StoredCA{}, false, fmt.Errorf("tlsca: load ca_key: %w", err)
	}
	return st, found, nil
}

func (s pgStore) Insert(ctx context.Context, st StoredCA) error {
	err := dbx.System(ctx, s.pool, dbx.ReasonBootstrap, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx,
			"INSERT INTO ca_key (id, ciphertext, key_id, cert_pem) VALUES (1, $1, $2, $3) ON CONFLICT (id) DO NOTHING",
			st.Ciphertext, st.KeyID, string(st.CertPEM))
		return err
	})
	if err != nil {
		return fmt.Errorf("tlsca: insert ca_key: %w", err)
	}
	return nil
}
