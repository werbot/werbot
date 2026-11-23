package identity

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	"github.com/werbot/werbot/internal/identity/dbq"
	"github.com/werbot/werbot/internal/platform/dbx"
	"github.com/werbot/werbot/internal/platform/principal"
)

// Login checks email and password in the user's first organization. A user
// without a second factor gets a session (LoginResult.Token); a user with one
// gets an mfa_token for CompleteMFA and no session. Unknown email, wrong
// password and no membership all return errInvalidCredentials; the password
// is verified in every case. Past the login rate limit it returns
// apperr.ErrResourceExhausted before the check.
func (s *Service) Login(ctx context.Context, email, password, userAgent string) (LoginResult, error) {
	u, found, err := s.loginUser(ctx, normalizeEmail(email))
	if err != nil {
		return LoginResult{}, err
	}
	// argon2 runs outside the transaction: no pooled connection waits on it.
	hash := dummyHash
	if found {
		hash = u.PasswordHash
	}
	ok, err := verifyPassword(ctx, hash, password)
	if err != nil {
		return LoginResult{}, err
	}
	if !found || !ok || u.OrgID == nil {
		return LoginResult{}, errInvalidCredentials
	}
	return s.finishLogin(ctx, u.ID, *u.OrgID, userAgent)
}

// loginUser counts the attempt and finds the user of email; found is false
// for an unknown email.
func (s *Service) loginUser(ctx context.Context, email string) (u dbq.GetLoginUserRow, found bool, err error) {
	if err := s.limitLogin(ctx, email); err != nil {
		return u, false, err
	}
	err = dbx.System(ctx, s.pool, dbx.ReasonLogin, func(tx pgx.Tx) error {
		var err error
		u, err = newStore(tx).GetLoginUser(ctx, email)
		return err
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return u, false, nil
	}
	return u, err == nil, err
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
