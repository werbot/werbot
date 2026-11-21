package identity

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/werbot/werbot/internal/identity/dbq"
	"github.com/werbot/werbot/internal/platform/apperr"
	"github.com/werbot/werbot/internal/platform/auditlog"
	"github.com/werbot/werbot/internal/platform/clientip"
	"github.com/werbot/werbot/internal/platform/dbx"
)

// RequestPasswordReset emails a reset link when the address belongs to a
// user. It returns nil either way, so the answer never reveals an account.
func (s *Service) RequestPasswordReset(ctx context.Context, email string) error {
	email = normalizeEmail(email)
	return dbx.System(ctx, s.pool, dbx.ReasonLogin, func(tx pgx.Tx) error {
		st := newStore(tx)
		u, err := st.GetResetUser(ctx, email)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		raw, err := issueToken(ctx, st, tokenPasswordReset, tokenClaims{UserID: &u.ID})
		if err != nil {
			return err
		}
		data := map[string]string{"url": s.cfg.WebURL + "/reset-password/" + raw, "name": u.Name}
		if err := s.emails.EnqueueTx(ctx, tx, email, "password_reset", data); err != nil {
			return err
		}
		return writeUserAudit(ctx, tx, "auth.password_reset_request", u.OrgID, u.ID)
	})
}

// ResetPassword sets a new password with a reset token and signs the user out
// everywhere, CLI sessions included. A bad password leaves the token unspent.
func (s *Service) ResetPassword(ctx context.Context, token, newPassword string) error {
	if err := validatePassword(newPassword); err != nil {
		return err
	}
	hash, err := hashPassword(ctx, newPassword) // outside the transaction: argon2 holds no connection
	if err != nil {
		return err
	}
	return dbx.System(ctx, s.pool, dbx.ReasonLogin, func(tx pgx.Tx) error {
		st := newStore(tx)
		c, err := consumeToken(ctx, st, tokenPasswordReset, token)
		if err != nil {
			return err
		}
		if c.UserID == nil {
			return fmt.Errorf("invalid or expired token: %w", apperr.ErrNotFound)
		}
		if err := st.UpdateUserPassword(ctx, dbq.UpdateUserPasswordParams{ID: *c.UserID, PasswordHash: hash}); err != nil {
			return err
		}
		if err := st.DeleteUserSessions(ctx, *c.UserID); err != nil {
			return err
		}
		if err := st.RevokeUserCLISessions(ctx, dbq.RevokeUserCLISessionsParams{UserID: *c.UserID}); err != nil {
			return err
		}
		var orgID *uuid.UUID
		switch id, err := st.GetUserOrg(ctx, *c.UserID); {
		case err == nil:
			orgID = &id
		case !errors.Is(err, pgx.ErrNoRows):
			return err
		}
		return writeUserAudit(ctx, tx, "auth.password_reset", orgID, *c.UserID)
	})
}

// writeUserAudit is writeAudit for users who may have no organization.
func writeUserAudit(ctx context.Context, tx pgx.Tx, action string, orgID *uuid.UUID, userID uuid.UUID) error {
	return auditlog.Write(ctx, tx, auditlog.Event{
		OrgID: orgID, ActorUserID: &userID, ActorKind: "user", Action: action, IP: clientip.From(ctx),
	})
}
