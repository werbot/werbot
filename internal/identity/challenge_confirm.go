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
	"github.com/werbot/werbot/internal/platform/principal"
)

var errPasskeyChallenge = fmt.Errorf("the challenge needs a passkey: %w", apperr.ErrPermissionDenied)

// ConfirmMFAChallenge confirms challenge id of p's user with assertion (the
// browser's JSON) of its passkey ceremony (GetMFAChallenge). Another user's
// challenge is ErrNotFound, one that is not pending or has expired
// ErrFailedPrecondition, an agent ErrPermissionDenied. A wrong assertion is
// ErrInvalidCode: the challenge stays pending and auth.mfa.fail is written.
func (s *Service) ConfirmMFAChallenge(ctx context.Context, p principal.Principal, id uuid.UUID, assertion []byte) error {
	if p.IsAgent() {
		return errAgentChallenge
	}
	err := dbx.InOrg(ctx, s.pool, p.OrgID, p.UserID, func(tx pgx.Tx) error {
		_, err := newStore(tx).GetUserMFAChallenge(ctx, dbq.GetUserMFAChallengeParams{ID: id, OrgID: p.OrgID, UserID: p.UserID})
		if errors.Is(err, pgx.ErrNoRows) {
			return errChallengeNotFound
		}
		if err != nil {
			return err
		}
		return s.spendChallenge(ctx, tx, dbq.ConfirmMFAChallengeParams{ID: id, OrgID: p.OrgID, UserID: p.UserID}, factorWebAuthn, func() error {
			_, err := s.FinishWebAuthnLogin(ctx, tx, p.OrgID, p.UserID, "challenge", id.String(), assertion)
			return err
		})
	})
	if errors.Is(err, apperr.ErrPermissionDenied) { // a wrong assertion or a locked passkey
		return errors.Join(err, s.auditMFAFail(ctx, p.OrgID, p.UserID, factorWebAuthn))
	}
	return err
}

// VerifyChallengeTOTP confirms challenge id of instanceID with a TOTP code
// of its user. Another instance's challenge is ErrNotFound, one that is not
// pending ErrFailedPrecondition, one of strength webauthn
// ErrPermissionDenied. The attempt counts toward mfa:<user_id> first: past
// the limit the challenge is denied and the answer is ErrResourceExhausted.
// A wrong or reused code is ErrInvalidCode and writes auth.mfa.fail.
func (s *Service) VerifyChallengeTOTP(ctx context.Context, orgID, instanceID, id uuid.UUID, code string) error {
	c, err := s.instanceChallenge(ctx, orgID, instanceID, id)
	if err != nil {
		return err
	}
	if err := s.limit(ctx, "mfa:"+c.UserID.String(), MFAAttempts, MFAWindow); err != nil {
		if errors.Is(err, apperr.ErrResourceExhausted) {
			err = errors.Join(err, dbx.InOrg(ctx, s.pool, orgID, c.UserID, func(tx pgx.Tx) error {
				return newStore(tx).DenyMFAChallenge(ctx, dbq.DenyMFAChallengeParams{ID: id, OrgID: orgID})
			}))
		}
		return err
	}
	arg := dbq.ConfirmMFAChallengeParams{ID: id, OrgID: orgID, UserID: c.UserID, InstanceID: &instanceID}
	err = dbx.InOrg(ctx, s.pool, orgID, c.UserID, func(tx pgx.Tx) error {
		return s.spendChallenge(ctx, tx, arg, factorTOTP, func() error { return s.checkTOTP(ctx, tx, c.UserID, code) })
	})
	if errors.Is(err, ErrInvalidCode) {
		return errors.Join(err, s.auditMFAFail(ctx, orgID, c.UserID, factorTOTP))
	}
	return err
}

// instanceChallenge reads challenge id of instanceID that a TOTP code may
// confirm: pending, unexpired, of strength any.
func (s *Service) instanceChallenge(ctx context.Context, orgID, instanceID, id uuid.UUID) (dbq.GetInstanceMFAChallengeRow, error) {
	var c dbq.GetInstanceMFAChallengeRow
	err := dbx.ReadInOrg(s.pool, orgID, uuid.Nil, func(r *dbx.Read) (err error) {
		c, err = newStore(r).GetInstanceMFAChallenge(ctx, dbq.GetInstanceMFAChallengeParams{ID: id, OrgID: orgID, InstanceID: instanceID})
		return err
	})
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return c, errChallengeNotFound
	case err != nil:
		return c, err
	case c.Strength == factorWebAuthn:
		return c, errPasskeyChallenge
	case c.Status != "pending" || !c.Live:
		return c, errChallengeNotPending
	}
	return c, nil
}

// spendChallenge confirms the pending, unexpired challenge of arg in tx,
// runs check (the factor of method), records the verification for the
// challenge's binding, audits mfa.challenge.confirm and notifies the
// challenge's instance. Any error rolls all of it back with tx.
func (s *Service) spendChallenge(ctx context.Context, tx pgx.Tx, arg dbq.ConfirmMFAChallengeParams, method string, check func() error) error {
	st := newStore(tx)
	row, err := st.ConfirmMFAChallenge(ctx, arg)
	if errors.Is(err, pgx.ErrNoRows) {
		return errChallengeNotPending
	}
	if err != nil {
		return err
	}
	if err := check(); err != nil {
		return err
	}
	err = st.CreateMFAVerification(ctx, dbq.CreateMFAVerificationParams{OrgID: arg.OrgID, UserID: arg.UserID, Binding: row.Binding, Method: method})
	if err != nil {
		return err
	}
	err = auditlog.Write(ctx, tx, auditlog.Event{OrgID: &arg.OrgID, ActorUserID: &arg.UserID, ActorKind: "user", Action: "mfa.challenge.confirm",
		TargetType: "mfa_challenge", TargetID: arg.ID.String(), IP: clientip.From(ctx), Data: map[string]any{"challenge_id": arg.ID, "method": method}})
	if err != nil || row.GatewayInstanceID == nil {
		return err
	}
	return st.NotifyMFAConfirmed(ctx, row.GatewayInstanceID.String()+":"+arg.ID.String())
}
