package session

import (
	"bytes"
	"context"
	"crypto/ed25519"
	crand "crypto/rand"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/werbot/werbot/internal/platform/dbx"
	"github.com/werbot/werbot/internal/platform/secretbox"
	"github.com/werbot/werbot/internal/session/dbq"
)

// Signer signs recording manifests with the install-wide ed25519 key.
type Signer struct{ key ed25519.PrivateKey }

// LoadSigner loads the signing key, creating it on first use. Concurrent
// first calls converge on one key: the first insert wins, the rest re-read.
func LoadSigner(ctx context.Context, pool *pgxpool.Pool, box *secretbox.Box) (*Signer, error) {
	var s *Signer
	err := dbx.System(ctx, pool, dbx.ReasonBootstrap, func(tx pgx.Tx) error {
		st := newStore(tx)
		row, err := st.GetSigningKey(ctx)
		if errors.Is(err, pgx.ErrNoRows) {
			if row, err = createSigningKey(ctx, st, box); err != nil {
				return err
			}
		} else if err != nil {
			return err
		}
		s, err = openSigner(box, row)
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("load signing key: %w", err)
	}
	return s, nil
}

func createSigningKey(ctx context.Context, st Store, box *secretbox.Box) (dbq.GetSigningKeyRow, error) {
	pub, priv, err := ed25519.GenerateKey(crand.Reader)
	if err != nil {
		return dbq.GetSigningKeyRow{}, err
	}
	ct, keyID, err := box.Seal(priv.Seed())
	if err != nil {
		return dbq.GetSigningKeyRow{}, err
	}
	if err := st.InsertSigningKey(ctx, dbq.InsertSigningKeyParams{Ciphertext: ct, KeyID: keyID, PublicKey: pub}); err != nil {
		return dbq.GetSigningKeyRow{}, err
	}
	return st.GetSigningKey(ctx) // a concurrent insert may have won
}

func openSigner(box *secretbox.Box, row dbq.GetSigningKeyRow) (*Signer, error) {
	seed, err := box.Open(row.Ciphertext, row.KeyID)
	if err != nil {
		return nil, err
	}
	key := ed25519.NewKeyFromSeed(seed)
	if !bytes.Equal(key.Public().(ed25519.PublicKey), row.PublicKey) {
		return nil, errors.New("stored public key does not match the private key")
	}
	return &Signer{key: key}, nil
}

// Sign returns the ed25519 signature of msg.
func (s *Signer) Sign(msg []byte) []byte { return ed25519.Sign(s.key, msg) }

// Verify reports whether sig is a valid signature of msg.
func (s *Signer) Verify(msg, sig []byte) bool { return ed25519.Verify(s.PublicKey(), msg, sig) }

// PublicKey returns the verification key.
func (s *Signer) PublicKey() ed25519.PublicKey { return s.key.Public().(ed25519.PublicKey) }
