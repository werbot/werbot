package oidclogin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/werbot/werbot/internal/identity/oidclogin/dbq"
	"github.com/werbot/werbot/internal/platform/apperr"
	"github.com/werbot/werbot/internal/platform/dbx"
	"github.com/werbot/werbot/internal/platform/secretbox"
)

// Store keeps the IdPs (client secrets sealed) and the links' refresh tokens.
type Store struct {
	pool *pgxpool.Pool
	box  *secretbox.Box
}

var _ IdPStore = (*Store)(nil)

// NewStore returns the store; box seals client secrets and refresh tokens.
func NewStore(pool *pgxpool.Pool, box *secretbox.Box) *Store { return &Store{pool: pool, box: box} }

// errNotFound answers an unknown IdP.
var errNotFound = fmt.Errorf("identity provider not found: %w", apperr.ErrNotFound)

// Get returns IdP id of any organization with its client secret opened: the
// public login routes look it up by id (dbx.ReasonLogin).
func (s *Store) Get(ctx context.Context, id uuid.UUID) (IdP, error) {
	var row dbq.IdpOidc
	err := dbx.System(ctx, s.pool, dbx.ReasonLogin, func(tx pgx.Tx) (err error) {
		row, err = dbq.New(tx).GetIdP(ctx, id)
		return err
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return IdP{}, errNotFound
	}
	if err != nil {
		return IdP{}, err
	}
	idp, err := fromRow(row)
	if err != nil {
		return IdP{}, err
	}
	secret, err := s.box.Open(row.ClientSecretCiphertext, row.KeyID)
	idp.ClientSecret = string(secret)
	return idp, err
}

// fromRow is the IdP of row without its secret.
func fromRow(r dbq.IdpOidc) (IdP, error) {
	idp := IdP{ID: r.ID, OrgID: r.OrgID, Name: r.Name, Issuer: r.Issuer, ClientID: r.ClientID, Scopes: r.Scopes,
		AllowSignup: r.AllowSignup, Enabled: r.Enabled, CreatedAt: r.CreatedAt}
	return idp, json.Unmarshal(r.ClaimsMapping, &idp.Mapping)
}

// SealRefresh seals an IdP refresh token.
func (s *Store) SealRefresh(refresh string) ([]byte, string, error) {
	return s.box.Seal([]byte(refresh))
}

// StaleLinks returns up to limit links with a refresh token whose last check
// is older than olderThan, oldest first, refresh tokens opened (dbx.ReasonMaintenance).
func (s *Store) StaleLinks(ctx context.Context, olderThan time.Duration, limit int) ([]Link, error) {
	var rows []dbq.StaleLinksRow
	err := dbx.System(ctx, s.pool, dbx.ReasonMaintenance, func(tx pgx.Tx) (err error) {
		rows, err = dbq.New(tx).StaleLinks(ctx, dbq.StaleLinksParams{OlderSecs: olderThan.Seconds(), Lim: int32(min(limit, 1000))}) //nolint:gosec // capped
		return err
	})
	if err != nil {
		return nil, err
	}
	out := make([]Link, 0, len(rows))
	for _, r := range rows {
		refresh, err := s.box.Open(r.RefreshCiphertext, deref(r.KeyID))
		if err != nil {
			return nil, err
		}
		out = append(out, Link{IdPID: r.IdpID, OrgID: r.OrgID, UserID: r.UserID, Subject: r.Subject, Refresh: string(refresh)})
	}
	return out, nil
}

// UpdateLink sets the last check of l to checkedAt; refresh nil keeps the
// stored token, "" drops it, anything else is sealed and stored.
func (s *Store) UpdateLink(ctx context.Context, l Link, refresh *string, checkedAt time.Time) error {
	arg := dbq.UpdateLinkParams{CheckedAt: checkedAt, KeepRefresh: refresh == nil, IdpID: l.IdPID, Subject: l.Subject, OrgID: l.OrgID}
	if refresh != nil && *refresh != "" {
		sealed, keyID, err := s.SealRefresh(*refresh)
		if err != nil {
			return err
		}
		arg.RefreshCiphertext, arg.KeyID = sealed, &keyID
	}
	return dbx.System(ctx, s.pool, dbx.ReasonMaintenance, func(tx pgx.Tx) error {
		return dbq.New(tx).UpdateLink(ctx, arg)
	})
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
