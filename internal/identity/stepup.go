package identity

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/werbot/werbot/internal/identity/dbq"
	"github.com/werbot/werbot/internal/platform/apperr"
	"github.com/werbot/werbot/internal/platform/clientip"
	"github.com/werbot/werbot/internal/platform/dbx"
	"github.com/werbot/werbot/internal/platform/principal"
)

// StepUp answers BeginStepUp: the Methods the caller can step up with (a
// subset of "webauthn", "totp" in that order). A browser session with a
// passkey gets the assertion options (JSON); a CLI token with a passkey gets
// the URL and Code of a challenge that, confirmed in the browser, records
// the step-up for the token's CLI session.
type StepUp struct {
	Methods         []string
	WebAuthnOptions []byte
	URL, Code       string
}

var (
	errAgentStepUp     = fmt.Errorf("agents cannot step up: %w", apperr.ErrPermissionDenied)
	errNoStepUpFactor  = fmt.Errorf("no second factor to step up with: %w", apperr.ErrFailedPrecondition)
	errRecoveryStepUp  = fmt.Errorf("recovery codes do not step up: %w", apperr.ErrInvalidArgument)
	errCLIPasskeyStep  = fmt.Errorf("a CLI token steps up with a passkey through the challenge url: %w", apperr.ErrInvalidArgument)
	errStepUpPrincipal = fmt.Errorf("step-up needs a session or a CLI token: %w", apperr.ErrUnauthenticated)
)

// BeginStepUp starts a step-up of p's credential. A user without a passkey
// (or with passkeys off) or a TOTP secret is ErrFailedPrecondition; an agent
// ErrPermissionDenied.
func (s *Service) BeginStepUp(ctx context.Context, p principal.Principal) (StepUp, error) {
	if p.IsAgent() {
		return StepUp{}, errAgentStepUp
	}
	var f dbq.GetLoginFactorsRow
	err := dbx.ReadInOrg(s.pool, p.OrgID, p.UserID, func(r *dbx.Read) (err error) {
		f, err = newStore(r).GetLoginFactors(ctx, p.UserID)
		return err
	})
	if err != nil {
		return StepUp{}, err
	}
	var su StepUp
	if f.HasPasskey && s.wa != nil {
		su.Methods = append(su.Methods, factorWebAuthn)
		if err := s.passkeyStepUp(ctx, p, &su); err != nil {
			return StepUp{}, err
		}
	}
	if f.HasTotp {
		su.Methods = append(su.Methods, factorTOTP)
	}
	if len(su.Methods) == 0 {
		return StepUp{}, errNoStepUpFactor
	}
	return su, nil
}

// passkeyStepUp starts the passkey part of su: the stepup ceremony bound to
// a browser session, or a challenge bound to a CLI token's session.
func (s *Service) passkeyStepUp(ctx context.Context, p principal.Principal, su *StepUp) error {
	if p.AuthMethod == principal.AuthMethodCookie {
		opts, err := s.BeginWebAuthnLogin(ctx, p.OrgID, p.UserID, "stepup", p.SessionID.String(), CeremonyTTL)
		su.WebAuthnOptions = opts
		return err
	}
	c, err := s.CreateMFAChallenge(ctx, MFAChallengeInput{OrgID: p.OrgID, UserID: p.UserID, ClientIP: clientip.From(ctx),
		Binding: "cli:" + p.CLISessionID.String(), Strength: "any"})
	su.URL, su.Code = c.URL, c.Code
	return err
}

// CompleteStepUp records a second factor for p's credential: a browser
// session gets mfa_at = now() and the verification web:<session id>, a CLI
// token the verification cli:<cli_session id>. A TOTP attempt counts against
// mfa:<user_id> first. A recovery code, and a passkey of a CLI token (it
// goes through the challenge of BeginStepUp), are ErrInvalidArgument; an
// agent is ErrPermissionDenied. A wrong factor is ErrInvalidCode and writes
// auth.mfa.fail.
func (s *Service) CompleteStepUp(ctx context.Context, p principal.Principal, proof MFAProof) error {
	method, binding, err := stepUpTarget(p, proof)
	if err != nil {
		return err
	}
	if method == factorTOTP {
		if err := s.limit(ctx, "mfa:"+p.UserID.String(), MFAAttempts, MFAWindow); err != nil {
			return err
		}
	}
	err = dbx.InOrg(ctx, s.pool, p.OrgID, p.UserID, func(tx pgx.Tx) error {
		return s.stepUp(ctx, tx, p, proof, method, binding)
	})
	if errors.Is(err, apperr.ErrPermissionDenied) { // a wrong factor or a locked passkey
		return errors.Join(err, s.auditMFAFail(ctx, p.OrgID, p.UserID, method))
	}
	return err
}

// stepUpTarget checks the proof p may step up with and names its method and
// the binding the verification is recorded for.
func stepUpTarget(p principal.Principal, proof MFAProof) (method, binding string, err error) {
	if p.IsAgent() {
		return "", "", errAgentStepUp
	}
	if method, err = proof.method(); err != nil {
		return "", "", err
	}
	switch {
	case method == factorRecovery:
		return "", "", errRecoveryStepUp
	case p.AuthMethod == principal.AuthMethodCookie:
		return method, "web:" + p.SessionID.String(), nil
	case p.AuthMethod != principal.AuthMethodOAuth:
		return "", "", errStepUpPrincipal
	case method == factorWebAuthn:
		return "", "", errCLIPasskeyStep
	}
	return method, "cli:" + p.CLISessionID.String(), nil
}

// stepUp is CompleteStepUp's effect in tx: it spends the factor, marks a
// browser session and records the verification for binding.
func (s *Service) stepUp(ctx context.Context, tx pgx.Tx, p principal.Principal, proof MFAProof, method, binding string) error {
	var err error
	if method == factorTOTP {
		err = s.checkTOTP(ctx, tx, p.UserID, proof.TOTP)
	} else {
		_, err = s.FinishWebAuthnLogin(ctx, tx, p.OrgID, p.UserID, "stepup", p.SessionID.String(), proof.Assertion)
	}
	if err != nil {
		return err
	}
	st := newStore(tx)
	if p.AuthMethod == principal.AuthMethodCookie {
		if err := st.SetSessionMFAByID(ctx, dbq.SetSessionMFAByIDParams{ID: p.SessionID, OrgID: p.OrgID, UserID: p.UserID}); err != nil {
			return err
		}
	}
	return st.CreateMFAVerification(ctx, dbq.CreateMFAVerificationParams{OrgID: p.OrgID, UserID: p.UserID, Binding: binding, Method: method})
}
