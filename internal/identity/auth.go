package identity

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"github.com/werbot/werbot/internal/identity/dbq"
	"github.com/werbot/werbot/internal/platform/dbx"
	"github.com/werbot/werbot/internal/platform/principal"
)

// Login checks email and password and opens a session in the user's first
// organization. Unknown email, wrong password and no membership all return
// errInvalidCredentials; the password is verified in every case.
func (s *Service) Login(ctx context.Context, email, password, userAgent string) (string, error) {
	var u dbq.GetLoginUserRow
	err := dbx.System(ctx, s.pool, dbx.ReasonLogin, func(tx pgx.Tx) error {
		var err error
		u, err = newStore(tx).GetLoginUser(ctx, normalizeEmail(email))
		return err
	})
	found := err == nil
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return "", err
	}
	// argon2 runs outside the transaction: no pooled connection waits on it.
	hash := dummyHash
	if found {
		hash = u.PasswordHash
	}
	ok, err := verifyPassword(ctx, hash, password)
	if err != nil {
		return "", err
	}
	if !found || !ok || u.OrgID == nil {
		return "", errInvalidCredentials
	}
	var token string
	err = dbx.System(ctx, s.pool, dbx.ReasonLogin, func(tx pgx.Tx) error {
		var err error
		if token, err = s.createSession(ctx, newStore(tx), u.ID, *u.OrgID, userAgent); err != nil {
			return err
		}
		return writeAudit(ctx, tx, "auth.login", *u.OrgID, u.ID)
	})
	if err != nil {
		return "", err
	}
	return token, nil
}

// Logout deletes the caller's session.
func (s *Service) Logout(ctx context.Context) error {
	p, err := principal.Require(ctx)
	if err != nil {
		return err
	}
	return dbx.InOrg(ctx, s.pool, p.OrgID, p.UserID, func(tx pgx.Tx) error {
		if err := newStore(tx).DeleteSession(ctx, dbq.DeleteSessionParams{ID: p.SessionID, OrgID: p.OrgID}); err != nil {
			return err
		}
		return writeAudit(ctx, tx, "auth.logout", p.OrgID, p.UserID)
	})
}
