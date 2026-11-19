package oauth

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	crand "crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"errors"
	"fmt"

	jose "github.com/go-jose/go-jose/v4"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/werbot/werbot/internal/identity/oauth/dbq"
	"github.com/werbot/werbot/internal/platform/dbx"
	"github.com/werbot/werbot/internal/platform/secretbox"
)

// SigningKey is the install-wide ES256 key that signs access and ID tokens.
// It implements op.SigningKey; its public half is served at /oauth/keys.
type SigningKey struct {
	key    *ecdsa.PrivateKey
	id     string   // RFC 7638 thumbprint of the public key
	crypto [32]byte // op.Config.CryptoKey: the same on every replica, no extra secret
}

// LoadSigningKey loads the signing key, creating it on first use. It runs
// under db.BootstrapLock; concurrent first calls still converge on one key:
// the first insert wins, the rest re-read.
func LoadSigningKey(ctx context.Context, pool *pgxpool.Pool, box *secretbox.Box) (*SigningKey, error) {
	var k *SigningKey
	err := dbx.System(ctx, pool, dbx.ReasonBootstrap, func(tx pgx.Tx) error {
		q := dbq.New(tx)
		row, err := q.GetSigningKey(ctx)
		if errors.Is(err, pgx.ErrNoRows) {
			if row, err = createSigningKey(ctx, q, box); err != nil {
				return err
			}
		} else if err != nil {
			return err
		}
		k, err = openSigningKey(box, row)
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("load oauth signing key: %w", err)
	}
	return k, nil
}

func createSigningKey(ctx context.Context, q *dbq.Queries, box *secretbox.Box) (dbq.GetSigningKeyRow, error) {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), crand.Reader)
	if err != nil {
		return dbq.GetSigningKeyRow{}, err
	}
	der, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		return dbq.GetSigningKeyRow{}, err
	}
	id, err := thumbprint(&priv.PublicKey)
	if err != nil {
		return dbq.GetSigningKeyRow{}, err
	}
	jwk, err := (&jose.JSONWebKey{Key: &priv.PublicKey, KeyID: id, Algorithm: string(jose.ES256), Use: "sig"}).MarshalJSON()
	if err != nil {
		return dbq.GetSigningKeyRow{}, err
	}
	ct, keyID, err := box.Seal(der)
	if err != nil {
		return dbq.GetSigningKeyRow{}, err
	}
	if err := q.InsertSigningKey(ctx, dbq.InsertSigningKeyParams{Ciphertext: ct, KeyID: keyID, PublicJwk: jwk}); err != nil {
		return dbq.GetSigningKeyRow{}, err
	}
	return q.GetSigningKey(ctx) // a concurrent insert may have won
}

func openSigningKey(box *secretbox.Box, row dbq.GetSigningKeyRow) (*SigningKey, error) {
	der, err := box.Open(row.Ciphertext, row.KeyID)
	if err != nil {
		return nil, err
	}
	parsed, err := x509.ParsePKCS8PrivateKey(der)
	if err != nil {
		return nil, err
	}
	priv, ok := parsed.(*ecdsa.PrivateKey)
	if !ok || priv.Curve != elliptic.P256() {
		return nil, errors.New("stored key is not an ECDSA P-256 key")
	}
	id, err := thumbprint(&priv.PublicKey)
	if err != nil {
		return nil, err
	}
	return &SigningKey{key: priv, id: id, crypto: sha256.Sum256(append([]byte("werbot-oauth-crypto"), der...))}, nil
}

// thumbprint is the RFC 7638 JWK thumbprint (SHA-256, base64url).
func thumbprint(pub *ecdsa.PublicKey) (string, error) {
	sum, err := (&jose.JSONWebKey{Key: pub}).Thumbprint(crypto.SHA256)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(sum), nil
}

// SignatureAlgorithm implements op.SigningKey.
func (*SigningKey) SignatureAlgorithm() jose.SignatureAlgorithm { return jose.ES256 }

// Key implements op.SigningKey: the private key.
func (k *SigningKey) Key() any { return k.key }

// ID implements op.SigningKey: the JWK kid.
func (k *SigningKey) ID() string { return k.id }

// publicKey is the verification half of a SigningKey (op.Key).
type publicKey struct{ k *SigningKey }

func (p publicKey) ID() string                       { return p.k.id }
func (publicKey) Algorithm() jose.SignatureAlgorithm { return jose.ES256 }
func (publicKey) Use() string                        { return "sig" }
func (p publicKey) Key() any                         { return &p.k.key.PublicKey }

// VerifySignature implements oidc.KeySet for the access token verifier.
func (p publicKey) VerifySignature(_ context.Context, jws *jose.JSONWebSignature) ([]byte, error) {
	return jws.Verify(&p.k.key.PublicKey)
}
