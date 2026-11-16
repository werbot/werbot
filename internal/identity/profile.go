package identity

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/werbot/werbot/internal/identity/dbq"
	"github.com/werbot/werbot/internal/platform/apperr"
	"github.com/werbot/werbot/internal/platform/dbx"
	"github.com/werbot/werbot/internal/platform/principal"
)

func readProfile(ctx context.Context, st Store, p principal.Principal) (User, error) {
	r, err := st.GetProfile(ctx, p.UserID)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, apperr.ErrUnauthenticated
	}
	return User{ID: r.ID, Email: r.Email, Login: r.Login, Name: r.Name, CreatedAt: r.CreatedAt}, err
}

// Profile returns the caller's user.
func (s *Service) Profile(ctx context.Context) (User, error) {
	p, err := principal.Require(ctx)
	if err != nil {
		return User{}, err
	}
	var u User
	err = dbx.InOrg(ctx, s.pool, p.OrgID, p.UserID, func(tx pgx.Tx) error {
		u, err = readProfile(ctx, newStore(tx), p)
		return err
	})
	return u, err
}

// UpdateProfile sets the caller's name and email. An email used by another
// user is ErrAlreadyExists.
func (s *Service) UpdateProfile(ctx context.Context, name, email string) (User, error) {
	p, err := principal.Require(ctx)
	if err != nil {
		return User{}, err
	}
	var u User
	err = dbx.InOrg(ctx, s.pool, p.OrgID, p.UserID, func(tx pgx.Tx) error {
		st := newStore(tx)
		if err := st.UpdateProfile(ctx, dbq.UpdateProfileParams{ID: p.UserID, Name: name, Email: normalizeEmail(email)}); err != nil {
			return err
		}
		if u, err = readProfile(ctx, st, p); err != nil {
			return err
		}
		return writeAudit(ctx, tx, "profile.update", p.OrgID, p.UserID)
	})
	if isUniqueViolation(err) {
		return User{}, fmt.Errorf("email already in use: %w", apperr.ErrAlreadyExists)
	}
	return u, err
}

// ChangePassword sets a new password after checking the current one and
// signs the user out everywhere except the current session.
func (s *Service) ChangePassword(ctx context.Context, current, next string) error {
	p, err := principal.Require(ctx)
	if err != nil {
		return err
	}
	if err := validatePassword(next); err != nil {
		return err
	}
	return dbx.InOrg(ctx, s.pool, p.OrgID, p.UserID, func(tx pgx.Tx) error {
		st := newStore(tx)
		hash, err := st.GetPasswordHash(ctx, p.UserID)
		if err != nil {
			return err
		}
		ok, err := verifyPassword(ctx, hash, current)
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("current password is wrong: %w", apperr.ErrInvalidArgument)
		}
		if hash, err = hashPassword(ctx, next); err != nil {
			return err
		}
		if err := st.UpdateUserPassword(ctx, dbq.UpdateUserPasswordParams{ID: p.UserID, PasswordHash: hash}); err != nil {
			return err
		}
		if err := st.DeleteOtherUserSessions(ctx, dbq.DeleteOtherUserSessionsParams{UserID: p.UserID, KeepID: p.SessionID}); err != nil {
			return err
		}
		return writeAudit(ctx, tx, "profile.password_change", p.OrgID, p.UserID)
	})
}

// RequestAccountDeletion emails the caller a confirmation link. The sole
// owner of any organization cannot delete the account.
func (s *Service) RequestAccountDeletion(ctx context.Context) error {
	p, err := principal.Require(ctx)
	if err != nil {
		return err
	}
	return dbx.InOrg(ctx, s.pool, p.OrgID, p.UserID, func(tx pgx.Tx) error {
		st := newStore(tx)
		if err := ensureNotSoleOwner(ctx, st, p.UserID); err != nil {
			return err
		}
		u, err := readProfile(ctx, st, p)
		if err != nil {
			return err
		}
		raw, err := issueToken(ctx, st, tokenAccountDeletion, tokenClaims{UserID: &p.UserID, OrgID: &p.OrgID})
		if err != nil {
			return err
		}
		data := map[string]string{"url": s.cfg.WebURL + "/profile/delete/" + raw, "name": u.Name}
		if err := s.emails.EnqueueTx(ctx, tx, u.Email, "account_deletion", data); err != nil {
			return err
		}
		return writeAudit(ctx, tx, "profile.deletion_request", p.OrgID, p.UserID)
	})
}

// ConfirmAccountDeletion deletes the caller's account with a token from
// RequestAccountDeletion. Another user's token is ErrNotFound and stays unspent.
func (s *Service) ConfirmAccountDeletion(ctx context.Context, token string) error {
	p, err := principal.Require(ctx)
	if err != nil {
		return err
	}
	return dbx.InOrg(ctx, s.pool, p.OrgID, p.UserID, func(tx pgx.Tx) error {
		st := newStore(tx)
		c, err := consumeToken(ctx, st, tokenAccountDeletion, token)
		if err != nil {
			return err
		}
		if c.UserID == nil || *c.UserID != p.UserID {
			return fmt.Errorf("invalid or expired token: %w", apperr.ErrNotFound)
		}
		if err := ensureNotSoleOwner(ctx, st, p.UserID); err != nil {
			return err
		}
		if err := st.DeleteUser(ctx, p.UserID); err != nil {
			return err
		}
		return writeAudit(ctx, tx, "profile.delete", p.OrgID, p.UserID)
	})
}
