package identity

import (
	"context"
	"encoding/hex"
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
)

// MFATokenTTL is how long the mfa_token of a password login can be finished
// with CompleteMFA.
const MFATokenTTL = 5 * time.Minute

// LoginResult answers Login: Token is the new session, or, for a user with a
// second factor, MFAToken with the Methods to finish the login with (a
// subset of "webauthn", "totp", "recovery" in that order) and, with
// "webauthn", the assertion options as JSON.
type LoginResult struct {
	Token, MFAToken string
	Methods         []string
	WebAuthnOptions []byte
}

// MFAProof is the second factor of CompleteMFA: exactly one field is set.
type MFAProof struct {
	TOTP, RecoveryCode string
	Assertion          []byte // the browser's WebAuthn assertion (JSON)
}

// method names the factor of the proof; anything but exactly one is ErrInvalidArgument.
func (p MFAProof) method() (string, error) {
	var set []string
	if p.TOTP != "" {
		set = append(set, factorTOTP)
	}
	if p.RecoveryCode != "" {
		set = append(set, factorRecovery)
	}
	if len(p.Assertion) > 0 {
		set = append(set, factorWebAuthn)
	}
	if len(set) != 1 {
		return "", fmt.Errorf("exactly one second factor is required: %w", apperr.ErrInvalidArgument)
	}
	return set[0], nil
}

// finishLogin opens the session of a user who passed the password check
// and has no second factor, or issues the mfa_token and offers the factors.
// auth.login is written only when a session is opened.
func (s *Service) finishLogin(ctx context.Context, userID, orgID uuid.UUID, userAgent string) (LoginResult, error) {
	var (
		res LoginResult
		f   dbq.GetLoginFactorsRow
	)
	err := dbx.System(ctx, s.pool, dbx.ReasonLogin, func(tx pgx.Tx) (err error) {
		st := newStore(tx)
		if f, err = st.GetLoginFactors(ctx, userID); err != nil {
			return err
		}
		if !f.HasTotp && !f.HasPasskey {
			if res.Token, err = s.createSession(ctx, st, userID, orgID, userAgent); err != nil {
				return err
			}
			return writeLoginAudit(ctx, tx, orgID, userID, "")
		}
		res.MFAToken, err = issueToken(ctx, st, tokenMFALogin, tokenClaims{OrgID: &orgID, UserID: &userID})
		return err
	})
	if err != nil || res.Token != "" {
		return res, err
	}
	return s.offerFactors(ctx, res, orgID, userID, f)
}

// offerFactors lists the factors res.MFAToken can be finished with and starts
// the WebAuthn login ceremony bound to the token when passkeys are on.
func (s *Service) offerFactors(ctx context.Context, res LoginResult, orgID, userID uuid.UUID, f dbq.GetLoginFactorsRow) (LoginResult, error) {
	if f.HasPasskey && s.wa != nil {
		opts, err := s.BeginWebAuthnLogin(ctx, orgID, userID, "login", mfaRef(res.MFAToken), MFATokenTTL)
		if err != nil {
			return LoginResult{}, err
		}
		res.Methods, res.WebAuthnOptions = append(res.Methods, factorWebAuthn), opts
	}
	if f.HasTotp {
		res.Methods = append(res.Methods, factorTOTP)
	}
	if f.HasRecovery {
		res.Methods = append(res.Methods, factorRecovery)
	}
	return res, nil
}

// mfaRef binds the WebAuthn login ceremony to its mfa_token by the token's hash.
func mfaRef(mfaToken string) string { return hex.EncodeToString(hashSecret(mfaToken)) }

// CompleteMFA finishes a password login: it spends mfaToken and opens a
// session that has passed a second factor (mfa_at = now()). An unknown, used
// or expired token is ErrNotFound. A TOTP or recovery code attempt counts
// against mfa:<user_id> first (WebAuthn does not). A wrong factor is
// ErrInvalidCode: the token stays usable and auth.mfa.fail is written.
func (s *Service) CompleteMFA(ctx context.Context, mfaToken string, proof MFAProof, userAgent string) (string, error) {
	method, err := proof.method()
	if err != nil {
		return "", err
	}
	var who dbq.GetMFALoginTokenRow
	err = dbx.System(ctx, s.pool, dbx.ReasonLogin, func(tx pgx.Tx) (err error) {
		who, err = newStore(tx).GetMFALoginToken(ctx, hashSecret(mfaToken))
		return err
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return "", fmt.Errorf("invalid or expired token: %w", apperr.ErrNotFound)
	}
	if err != nil {
		return "", err
	}
	if method != factorWebAuthn {
		if err := s.limit(ctx, "mfa:"+who.UserID.String(), MFAAttempts, MFAWindow); err != nil {
			return "", err
		}
	}
	var token string
	err = dbx.System(ctx, s.pool, dbx.ReasonLogin, func(tx pgx.Tx) (err error) {
		token, err = s.mfaSession(ctx, tx, who, mfaToken, proof, method, userAgent)
		return err
	})
	if errors.Is(err, apperr.ErrPermissionDenied) { // a wrong factor or a locked passkey
		return "", errors.Join(err, s.auditMFAFail(ctx, who.OrgID, who.UserID, method))
	}
	return token, err
}

// mfaSession is CompleteMFA's effect in tx: it spends the token, checks the
// factor of the token's user and opens the session; a passkey or TOTP check
// is recorded as mfa_verification of the session (web:<id>).
func (s *Service) mfaSession(ctx context.Context, tx pgx.Tx, who dbq.GetMFALoginTokenRow, mfaToken string, proof MFAProof, method, userAgent string) (string, error) {
	st := newStore(tx)
	if _, err := consumeToken(ctx, st, tokenMFALogin, mfaToken); err != nil {
		return "", err
	}
	if err := s.checkProof(ctx, tx, who, mfaRef(mfaToken), proof, method); err != nil {
		return "", err
	}
	token, err := s.createSession(ctx, st, who.UserID, who.OrgID, userAgent)
	if err != nil {
		return "", err
	}
	sid, err := st.SetSessionMFA(ctx, hashSecret(token))
	if err != nil {
		return "", err
	}
	if method != factorRecovery {
		err := st.CreateMFAVerification(ctx, dbq.CreateMFAVerificationParams{OrgID: who.OrgID, UserID: who.UserID, Binding: "web:" + sid.String(), Method: method})
		if err != nil {
			return "", err
		}
	}
	return token, writeLoginAudit(ctx, tx, who.OrgID, who.UserID, method)
}

// checkProof spends the factor of method in tx.
func (s *Service) checkProof(ctx context.Context, tx pgx.Tx, who dbq.GetMFALoginTokenRow, ref string, proof MFAProof, method string) error {
	switch method {
	case factorTOTP:
		return s.checkTOTP(ctx, tx, who.UserID, proof.TOTP)
	case factorRecovery:
		return s.useRecoveryCode(ctx, tx, who.UserID, proof.RecoveryCode)
	}
	_, err := s.FinishWebAuthnLogin(ctx, tx, who.OrgID, who.UserID, "login", ref, proof.Assertion)
	return err
}

// writeLoginAudit records auth.login of a password login; mfa names the
// second factor ("totp", "webauthn" or "recovery"), empty for none.
func writeLoginAudit(ctx context.Context, tx pgx.Tx, orgID, userID uuid.UUID, mfa string) error {
	data := map[string]any{"method": "password"}
	if mfa != "" {
		data["mfa"] = mfa
	}
	return auditlog.Write(ctx, tx, auditlog.Event{
		OrgID: &orgID, ActorUserID: &userID, ActorKind: "user", Action: "auth.login", IP: clientip.From(ctx), Data: data,
	})
}
