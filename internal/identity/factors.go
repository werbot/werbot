package identity

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/werbot/werbot/internal/identity/dbq"
	"github.com/werbot/werbot/internal/platform/apperr"
	"github.com/werbot/werbot/internal/platform/auditlog"
	"github.com/werbot/werbot/internal/platform/clientip"
	"github.com/werbot/werbot/internal/platform/dbx"
	"github.com/werbot/werbot/internal/platform/principal"
)

// Factor kinds; the TOTP factor and the recovery code set use the kind as id.
const (
	factorWebAuthn = "webauthn"
	factorTOTP     = "totp"
	factorRecovery = "recovery"
)

// Factor is a second factor of a user. ID is a passkey's uuid, "totp" or
// "recovery"; Remaining counts unused recovery codes.
type Factor struct {
	ID, Kind, Name string
	CreatedAt      time.Time // zero for recovery codes
	LastUsedAt     *time.Time
	Remaining      int
}

// ListFactors returns p's passkeys, confirmed TOTP secret and recovery code set.
func (s *Service) ListFactors(ctx context.Context, p principal.Principal) ([]Factor, error) {
	var fs []Factor
	err := dbx.InOrg(ctx, s.pool, p.OrgID, p.UserID, func(tx pgx.Tx) (err error) {
		fs, err = listFactors(ctx, newStore(tx), p.UserID)
		return err
	})
	return fs, err
}

func listFactors(ctx context.Context, st Store, userID uuid.UUID) ([]Factor, error) {
	keys, err := st.ListWebAuthnFactors(ctx, userID)
	if err != nil {
		return nil, err
	}
	fs := make([]Factor, 0, len(keys)+2)
	for _, k := range keys {
		fs = append(fs, Factor{ID: k.ID.String(), Kind: factorWebAuthn, Name: k.Name, CreatedAt: k.CreatedAt, LastUsedAt: k.LastUsedAt})
	}
	confirmed, err := st.GetTOTPFactor(ctx, userID)
	switch {
	case err == nil:
		fs = append(fs, Factor{ID: factorTOTP, Kind: factorTOTP, CreatedAt: confirmed})
	case !errors.Is(err, pgx.ErrNoRows):
		return nil, err
	}
	rec, err := st.GetRecoveryFactor(ctx, userID)
	if err != nil {
		return nil, err
	}
	if rec.Total > 0 {
		fs = append(fs, Factor{ID: factorRecovery, Kind: factorRecovery, Remaining: int(rec.Remaining)})
	}
	return fs, nil
}

// DeleteFactor removes p's factor id: "totp", "recovery" (the whole set) or a
// passkey's uuid. Removing the last TOTP secret or passkey while the
// organization requires MFA of p's role is ErrFailedPrecondition.
func (s *Service) DeleteFactor(ctx context.Context, p principal.Principal, id string) error {
	kind, keyID := id, uuid.Nil
	if id != factorTOTP && id != factorRecovery {
		var err error
		if keyID, err = parseID(id); err != nil {
			return err
		}
		kind = factorWebAuthn
	}
	return dbx.InOrg(ctx, s.pool, p.OrgID, p.UserID, func(tx pgx.Tx) error {
		st := newStore(tx)
		n, err := deleteFactor(ctx, st, p.UserID, kind, keyID)
		if err != nil {
			return err
		}
		if n == 0 {
			return fmt.Errorf("factor: %w", apperr.ErrNotFound)
		}
		if kind != factorRecovery {
			if err := keepRequiredFactor(ctx, st, p); err != nil {
				return err
			}
		}
		return writeFactorAudit(ctx, tx, "mfa.factor.delete", p, kind, id)
	})
}

func deleteFactor(ctx context.Context, st Store, userID uuid.UUID, kind string, keyID uuid.UUID) (int64, error) {
	switch kind {
	case factorTOTP:
		return st.DeleteTOTPSecret(ctx, userID)
	case factorRecovery:
		return st.DeleteRecoveryCodes(ctx, userID)
	}
	return st.DeleteWebAuthnCredential(ctx, dbq.DeleteWebAuthnCredentialParams{ID: keyID, UserID: userID})
}

// keepRequiredFactor refuses a state where p has no factor left although
// org_setting.require_mfa asks one of p's role ("all", or "admins" for owners
// and admins).
func keepRequiredFactor(ctx context.Context, st Store, p principal.Principal) error {
	has, err := st.HasFactor(ctx, p.UserID)
	if err != nil || has {
		return err
	}
	mode, err := st.GetRequireMFA(ctx, p.OrgID)
	if err != nil {
		return err
	}
	if mode == "all" || mode == "admins" && p.Manager() {
		return fmt.Errorf("mfa is required by the organization: %w", apperr.ErrFailedPrecondition)
	}
	return nil
}

// writeFactorAudit records mfa.factor.{add,delete} of p's factor id; the
// data holds only its kind.
func writeFactorAudit(ctx context.Context, tx pgx.Tx, action string, p principal.Principal, kind, id string) error {
	return auditlog.Write(ctx, tx, auditlog.Event{
		OrgID: &p.OrgID, ActorUserID: &p.UserID, ActorKind: "user", Action: action,
		TargetType: "mfa_factor", TargetID: id, IP: clientip.From(ctx),
		Data: map[string]any{"kind": kind},
	})
}

// auditMFAFail records auth.mfa.fail (data: the method) in its own
// transaction: the failed attempt's effect transaction has rolled back.
func (s *Service) auditMFAFail(ctx context.Context, orgID, userID uuid.UUID, method string) error {
	return dbx.InOrg(ctx, s.pool, orgID, userID, func(tx pgx.Tx) error {
		return auditlog.Write(ctx, tx, auditlog.Event{
			OrgID: &orgID, ActorUserID: &userID, ActorKind: "user", Action: "auth.mfa.fail",
			IP: clientip.From(ctx), Data: map[string]any{"method": method},
		})
	})
}
