package identity

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/platform/apperr"
)

// The gateway side of MFA challenges: TOTP codes, status, resend and age.

func TestVerifyChallengeTOTP(t *testing.T) {
	e := newChallengeEnv(t)
	ctx := context.Background()
	secret := e.f.TOTP(t, e.pool, e.svc.cfg.Box, e.p.UserID)
	step := currentStep()
	c := e.create(t, "any")
	org, inst := e.f.OrgID, e.f.InstanceID

	require.ErrorIs(t, e.svc.VerifyChallengeTOTP(ctx, org, e.f.OtherInstanceID, c.ID, codeAt(t, secret, step)), apperr.ErrNotFound,
		"another instance's challenge")
	require.ErrorIs(t, e.svc.VerifyChallengeTOTP(ctx, org, inst, c.ID, wrongCode(t, secret, step)), ErrInvalidCode)
	assert.Equal(t, "pending", e.status(t, c.ID))
	assert.Equal(t, 1, countAudit(t, e.pool, "auth.mfa.fail", `{"method":"totp"}`))

	notify := listenFor(t, e.pool, "werbot_mfa_confirmed", 5*time.Second)
	require.NoError(t, e.svc.VerifyChallengeTOTP(ctx, org, inst, c.ID, codeAt(t, secret, step)))
	assert.Equal(t, inst.String()+":"+c.ID.String(), <-notify)
	assert.Equal(t, "confirmed", e.status(t, c.ID))
	assert.Equal(t, 1, countRows(t, e.pool, "SELECT count(*) FROM mfa_verification WHERE binding = $1 AND method = 'totp'", testBinding))
	assert.Equal(t, 1, countAudit(t, e.pool, "mfa.challenge.confirm", `{"challenge_id":"`+c.ID.String()+`","method":"totp"}`))
	require.ErrorIs(t, e.svc.VerifyChallengeTOTP(ctx, org, inst, c.ID, codeAt(t, secret, step+1)), apperr.ErrFailedPrecondition, "confirmed")

	again := e.create(t, "any")
	require.ErrorIs(t, e.svc.VerifyChallengeTOTP(ctx, org, inst, again.ID, codeAt(t, secret, step)), ErrInvalidCode,
		"a step is spent once")
	strong := e.create(t, "webauthn")
	err := e.svc.VerifyChallengeTOTP(ctx, org, inst, strong.ID, codeAt(t, secret, step+1))
	require.ErrorIs(t, err, apperr.ErrPermissionDenied, "a passkey-only challenge takes no code")
	require.NotErrorIs(t, err, ErrInvalidCode)
	assert.Equal(t, "pending", e.status(t, strong.ID))
}

// TOTP codes of a challenge count toward mfa:<user_id>: past the limit the
// attempt is refused, even with a valid code, and the challenge is denied.
func TestVerifyChallengeTOTPLimit(t *testing.T) {
	e := newChallengeEnv(t)
	ctx := context.Background()
	secret := e.f.TOTP(t, e.pool, e.svc.cfg.Box, e.p.UserID)
	step := currentStep()
	c := e.create(t, "any")
	for range MFAAttempts {
		require.ErrorIs(t, e.svc.VerifyChallengeTOTP(ctx, e.f.OrgID, e.f.InstanceID, c.ID, wrongCode(t, secret, step)), ErrInvalidCode)
	}
	err := e.svc.VerifyChallengeTOTP(ctx, e.f.OrgID, e.f.InstanceID, c.ID, codeAt(t, secret, step))
	require.ErrorIs(t, err, apperr.ErrResourceExhausted)
	assert.Equal(t, "denied", e.status(t, c.ID))
	assert.Zero(t, countRows(t, e.pool, "SELECT count(*) FROM mfa_verification"))
}

// The gateway's status read turns an expired pending challenge into expired;
// another instance's challenge is not found.
func TestGetMFAChallengeExpires(t *testing.T) {
	e := newChallengeEnv(t)
	ctx := context.Background()
	c := e.create(t, "any")
	s, err := e.svc.ChallengeStatus(ctx, e.f.OrgID, e.f.InstanceID, c.ID)
	require.NoError(t, err)
	assert.Equal(t, "pending", s)
	_, err = e.svc.ChallengeStatus(ctx, e.f.OrgID, e.f.OtherInstanceID, c.ID)
	require.ErrorIs(t, err, apperr.ErrNotFound)

	e.expire(t, c.ID)
	s, err = e.svc.ChallengeStatus(ctx, e.f.OrgID, e.f.InstanceID, c.ID)
	require.NoError(t, err)
	assert.Equal(t, "expired", s)
	assert.Equal(t, "expired", e.status(t, c.ID), "the row is updated")
}

// Confirmed challenges are resent to their own instance while they live.
func TestConfirmedChallenges(t *testing.T) {
	e := newChallengeEnv(t)
	ctx := context.Background()
	live, old, pending := e.create(t, "any"), e.create(t, "any"), e.create(t, "any")
	_, err := e.pool.Exec(ctx, "UPDATE mfa_challenge SET status = 'confirmed' WHERE id = ANY($1)", []uuid.UUID{live.ID, old.ID})
	require.NoError(t, err)
	e.expire(t, old.ID)
	ids, err := e.svc.ConfirmedChallenges(ctx, e.f.OrgID, e.f.InstanceID)
	require.NoError(t, err)
	assert.Equal(t, []uuid.UUID{live.ID}, ids, "not %v (expired) nor %v (pending)", old.ID, pending.ID)
	ids, err = e.svc.ConfirmedChallenges(ctx, e.f.OrgID, e.f.OtherInstanceID)
	require.NoError(t, err)
	assert.Empty(t, ids)
}

func TestMFAAge(t *testing.T) {
	e := newChallengeEnv(t)
	ctx := context.Background()
	_, _, ok, err := e.svc.MFAAge(ctx, e.f.OrgID, e.p.UserID, testBinding)
	require.NoError(t, err)
	assert.False(t, ok, "no verification")

	_, err = e.pool.Exec(ctx, `INSERT INTO mfa_verification (org_id, user_id, binding, method, verified_at)
		VALUES ($1, $2, $3, 'totp', now() - interval '100 seconds'), ($1, $2, $3, 'webauthn', now() - interval '10 seconds')`,
		e.f.OrgID, e.p.UserID, testBinding)
	require.NoError(t, err)
	age, method, ok, err := e.svc.MFAAge(ctx, e.f.OrgID, e.p.UserID, testBinding)
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, "webauthn", method, "the last verification")
	assert.InDelta(t, 10*time.Second, age, float64(2*time.Second))

	for name, c := range map[string][3]string{
		"another binding": {e.f.OrgID.String(), e.p.UserID.String(), "cert:SHA256:" + strings.Repeat("A", 43)},
		"another user":    {e.f.OrgID.String(), e.f.OtherMemberID.String(), testBinding},
	} {
		_, _, ok, err := e.svc.MFAAge(ctx, uuid.MustParse(c[0]), uuid.MustParse(c[1]), c[2])
		require.NoError(t, err)
		assert.False(t, ok, name)
	}
}
