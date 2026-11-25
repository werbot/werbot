package identity

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"net/netip"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/werbot/werbot/internal/identity/dbq"
	"github.com/werbot/werbot/internal/platform/apperr"
	"github.com/werbot/werbot/internal/platform/dbx"
	"github.com/werbot/werbot/internal/platform/principal"
)

// MFA challenges (contract §3.6): a gateway asks a member to confirm an SSH
// login within ChallengeTTL; the terminal and the confirmation page show the
// same code of 8 letters of ChallengeAlphabet (XXXX-XXXX).
const (
	ChallengeTTL      = 60 * time.Second
	ChallengeAlphabet = "BCDFGHJKLMNPQRSTVWXZ"
)

var (
	errChallengeNotFound   = fmt.Errorf("mfa challenge not found: %w", apperr.ErrNotFound)
	errChallengeNotPending = fmt.Errorf("challenge is not pending: %w", apperr.ErrFailedPrecondition)
	errAgentChallenge      = fmt.Errorf("agents cannot confirm mfa challenges: %w", apperr.ErrPermissionDenied)
)

// MFAChallengeInput is a challenge a gateway instance asks for: Binding is
// the verified key or certificate the MFA is recorded for, Strength "any" or
// "webauthn". A nil InstanceID or ResourceID is stored as NULL.
type MFAChallengeInput struct {
	OrgID, UserID          uuid.UUID
	InstanceID, ResourceID *uuid.UUID
	SSHSessionID           []byte
	ClientIP               netip.Addr
	Binding, Strength      string
}

// MFAChallenge is a challenge as its user and gateway see it: Host is the
// instance's hostname, Resource the resource name; WebAuthnOptions (JSON) is
// set by GetMFAChallenge when the user has a passkey.
type MFAChallenge struct {
	ID                                          uuid.UUID
	Code, URL, Status, Strength, Host, Resource string
	ClientIP                                    netip.Addr
	ExpiresAt                                   time.Time
	WebAuthnOptions                             []byte
}

// CreateMFAChallenge stores a pending challenge of in.UserID that lives
// ChallengeTTL. A user who is not a member of in.OrgID, and a resource or
// instance outside it, is ErrNotFound; an empty binding or an unknown
// strength is ErrInvalidArgument.
func (s *Service) CreateMFAChallenge(ctx context.Context, in MFAChallengeInput) (MFAChallenge, error) {
	if in.Binding == "" || in.Strength != "any" && in.Strength != factorWebAuthn {
		return MFAChallenge{}, fmt.Errorf("binding and strength any or webauthn are required: %w", apperr.ErrInvalidArgument)
	}
	arg := dbq.CreateMFAChallengeParams{Code: challengeCode(), SshSessionID: in.SSHSessionID, ResourceID: in.ResourceID, Binding: in.Binding,
		Strength: in.Strength, GatewayInstanceID: in.InstanceID, TtlSecs: ChallengeTTL.Seconds(), OrgID: in.OrgID, UserID: in.UserID}
	if in.ClientIP.IsValid() {
		arg.ClientIp = &in.ClientIP
	}
	var row dbq.CreateMFAChallengeRow
	err := dbx.InOrg(ctx, s.pool, in.OrgID, in.UserID, func(tx pgx.Tx) (err error) {
		row, err = newStore(tx).CreateMFAChallenge(ctx, arg)
		return err
	})
	if errors.Is(err, pgx.ErrNoRows) || isForeignKeyViolation(err) {
		return MFAChallenge{}, errChallengeNotFound
	}
	if err != nil {
		return MFAChallenge{}, err
	}
	return MFAChallenge{ID: row.ID, Code: arg.Code, URL: s.challengeURL(row.ID), Status: "pending", Strength: in.Strength,
		ClientIP: in.ClientIP, ExpiresAt: row.ExpiresAt}, nil
}

// challengeCode draws 8 letters of ChallengeAlphabet as XXXX-XXXX.
func challengeCode() string {
	b := make([]byte, 0, 9)
	var r [1]byte
	for len(b) < cap(b) {
		if len(b) == 4 {
			b = append(b, '-')
		}
		_, _ = rand.Read(r[:]) // never returns an error
		// Bytes from 240 on are skipped: 240 = 12 × 20 leaves no modulo bias.
		if r[0] < 240 {
			b = append(b, ChallengeAlphabet[int(r[0])%len(ChallengeAlphabet)])
		}
	}
	return string(b)
}

func (s *Service) challengeURL(id uuid.UUID) string { return s.cfg.WebURL + "/mfa/" + id.String() }

// GetMFAChallenge returns challenge id of p's user while it is pending and
// unexpired, and starts its passkey ceremony when the user has a passkey.
// Another user's challenge is ErrNotFound, any other ErrFailedPrecondition;
// an agent is ErrPermissionDenied.
func (s *Service) GetMFAChallenge(ctx context.Context, p principal.Principal, id uuid.UUID) (MFAChallenge, error) {
	if p.IsAgent() {
		return MFAChallenge{}, errAgentChallenge
	}
	var row dbq.GetUserMFAChallengeRow
	err := dbx.ReadInOrg(s.pool, p.OrgID, p.UserID, func(r *dbx.Read) (err error) {
		row, err = newStore(r).GetUserMFAChallenge(ctx, dbq.GetUserMFAChallengeParams{ID: id, OrgID: p.OrgID, UserID: p.UserID})
		return err
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return MFAChallenge{}, errChallengeNotFound
	}
	if err != nil {
		return MFAChallenge{}, err
	}
	if row.Status != "pending" || !row.Live {
		return MFAChallenge{}, errChallengeNotPending
	}
	c := MFAChallenge{ID: id, Code: row.Code, URL: s.challengeURL(id), Status: row.Status, Strength: row.Strength, Host: row.Host,
		Resource: row.Resource, ExpiresAt: row.ExpiresAt}
	if row.ClientIp != nil {
		c.ClientIP = *row.ClientIp
	}
	c.WebAuthnOptions, err = s.BeginWebAuthnLogin(ctx, p.OrgID, p.UserID, "challenge", id.String(), ChallengeTTL)
	if errors.Is(err, apperr.ErrFailedPrecondition) { // passkeys are off or the user has none: TOTP in the terminal
		return c, nil
	}
	return c, err
}

// ChallengeStatus returns the status of challenge id of instanceID; a
// pending challenge past its expiry is stored and returned as expired.
// Another instance's challenge is ErrNotFound.
func (s *Service) ChallengeStatus(ctx context.Context, orgID, instanceID, id uuid.UUID) (string, error) {
	var status string
	err := dbx.InOrg(ctx, s.pool, orgID, uuid.Nil, func(tx pgx.Tx) (err error) {
		status, err = newStore(tx).MFAChallengeStatus(ctx, dbq.MFAChallengeStatusParams{ID: id, OrgID: orgID, InstanceID: instanceID})
		return err
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return "", errChallengeNotFound
	}
	return status, err
}

// ConfirmedChallenges lists the confirmed, unexpired challenges of
// instanceID, which a reconnected gateway may have missed.
func (s *Service) ConfirmedChallenges(ctx context.Context, orgID, instanceID uuid.UUID) ([]uuid.UUID, error) {
	var ids []uuid.UUID
	err := dbx.InOrg(ctx, s.pool, orgID, uuid.Nil, func(tx pgx.Tx) (err error) {
		ids, err = newStore(tx).ListConfirmedMFAChallenges(ctx, dbq.ListConfirmedMFAChallengesParams{OrgID: orgID, InstanceID: instanceID})
		return err
	})
	return ids, err
}

// MFAAge returns how long ago userID last passed a second factor for
// binding, and its method; ok is false when never.
func (s *Service) MFAAge(ctx context.Context, orgID, userID uuid.UUID, binding string) (age time.Duration, method string, ok bool, err error) {
	var row dbq.LastMFAVerificationRow
	err = dbx.ReadInOrg(s.pool, orgID, userID, func(r *dbx.Read) (err error) {
		row, err = newStore(r).LastMFAVerification(ctx, dbq.LastMFAVerificationParams{OrgID: orgID, UserID: userID, Binding: binding})
		return err
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, "", false, nil
	}
	if err != nil {
		return 0, "", false, err
	}
	return time.Duration(row.AgeSecs * float64(time.Second)), row.Method, true, nil
}
