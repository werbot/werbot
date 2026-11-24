package identity

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/identity/webauthntest"
	"github.com/werbot/werbot/internal/platform/apperr"
	"github.com/werbot/werbot/internal/platform/db/dbtest"
	"github.com/werbot/werbot/internal/platform/principal"
)

// stepUpUser is alice of newMFATest signed in with a session (before any
// factor) and her cookie principal.
func stepUpUser(t *testing.T) (*Service, principal.Principal) {
	t.Helper()
	svc, _, _ := newMFATest(t)
	ctx, _ := loginCtx(t, svc, "alice")
	p, _ := principal.From(ctx)
	return svc, p
}

// sessionMFAAt is the mfa_at of p's session.
func sessionMFAAt(t *testing.T, svc *Service, p principal.Principal) *time.Time {
	t.Helper()
	var at *time.Time
	require.NoError(t, svc.pool.QueryRow(context.Background(), "SELECT mfa_at FROM user_session WHERE id = $1", p.SessionID).Scan(&at))
	return at
}

func TestCompleteStepUpCookie(t *testing.T) {
	svc, p := stepUpUser(t)
	pool := svc.pool
	ctx := context.Background()
	secret := dbtest.Fixture{}.TOTP(t, pool, svc.cfg.Box, p.UserID)
	step := currentStep()
	web := "web:" + p.SessionID.String()

	su, err := svc.BeginStepUp(ctx, p)
	require.NoError(t, err)
	assert.Equal(t, StepUp{Methods: []string{"totp"}}, su)

	err = svc.CompleteStepUp(ctx, p, MFAProof{TOTP: wrongCode(t, secret, step)})
	require.ErrorIs(t, err, ErrInvalidCode)
	assert.Equal(t, 1, countAudit(t, pool, "auth.mfa.fail", `{"method":"totp"}`))
	assert.Nil(t, sessionMFAAt(t, svc, p))
	err = svc.CompleteStepUp(ctx, p, MFAProof{RecoveryCode: "abcd-efgh"})
	require.ErrorIs(t, err, apperr.ErrInvalidArgument, "recovery codes never step up")

	require.NoError(t, svc.CompleteStepUp(ctx, p, MFAProof{TOTP: codeAt(t, secret, step)}))
	at := sessionMFAAt(t, svc, p)
	require.NotNil(t, at)
	assert.WithinDuration(t, time.Now(), *at, time.Minute)
	assert.Equal(t, 1, countRows(t, pool, "SELECT count(*) FROM mfa_verification WHERE user_id = $1 AND binding = $2 AND method = 'totp'", p.UserID, web))

	a := webauthntest.New(t, testOrigin)
	registerPasskey(t, svc, p, a)
	su, err = svc.BeginStepUp(ctx, p)
	require.NoError(t, err)
	assert.Equal(t, []string{"webauthn", "totp"}, su.Methods)
	assert.Empty(t, su.URL, "a browser session confirms in place")
	require.NoError(t, svc.CompleteStepUp(ctx, p, MFAProof{Assertion: a.Get(su.WebAuthnOptions)}))
	assert.Equal(t, 1, countRows(t, pool, "SELECT count(*) FROM mfa_verification WHERE binding = $1 AND method = 'webauthn'", web))
	err = svc.CompleteStepUp(ctx, p, MFAProof{Assertion: a.Get(su.WebAuthnOptions)})
	require.ErrorIs(t, err, ErrInvalidCode, "the ceremony is single use")
}

func TestBeginStepUpBearer(t *testing.T) {
	s := newCLIStand(t)
	org := seedOrg(t, s.pool, "Acme")
	alice := seedUser(t, s.pool, org, "alice", principal.RoleMember)
	p := s.asBearer(t, s.cliLogin(t, org, alice).AccessToken, principal.RoleMember)
	cli := "cli:" + p.CLISessionID.String()
	ctx := context.Background()

	_, err := s.svc.BeginStepUp(ctx, p)
	require.ErrorIs(t, err, apperr.ErrFailedPrecondition, "no factor to step up with")
	dbtest.Fixture{}.TOTP(t, s.pool, s.svc.cfg.Box, alice)
	su, err := s.svc.BeginStepUp(ctx, p)
	require.NoError(t, err)
	assert.Equal(t, StepUp{Methods: []string{"totp"}}, su, "TOTP only: no challenge page")

	a := webauthntest.New(t, testOrigin)
	registerPasskey(t, s.svc, p, a)
	su, err = s.svc.BeginStepUp(ctx, p)
	require.NoError(t, err)
	assert.Equal(t, []string{"webauthn", "totp"}, su.Methods)
	assert.Empty(t, su.WebAuthnOptions, "a CLI token confirms the passkey on the challenge page")
	assert.Regexp(t, `^[BCDFGHJKLMNPQRSTVWXZ]{4}-[BCDFGHJKLMNPQRSTVWXZ]{4}$`, su.Code)
	require.Regexp(t, `^https://werbot\.test/mfa/[0-9a-f-]{36}$`, su.URL)
	id := uuid.MustParse(su.URL[len("https://werbot.test/mfa/"):])
	assert.Equal(t, 1, countRows(t, s.pool, `SELECT count(*) FROM mfa_challenge
		WHERE id = $1 AND binding = $2 AND gateway_instance_id IS NULL AND status = 'pending'`, id, cli))
	err = s.svc.CompleteStepUp(ctx, p, MFAProof{Assertion: []byte("{}")})
	require.ErrorIs(t, err, apperr.ErrInvalidArgument, "a CLI token steps up with a passkey through the challenge")

	notified := listenFor(t, s.pool, 500*time.Millisecond)
	c, err := s.svc.GetMFAChallenge(ctx, p, id)
	require.NoError(t, err)
	require.NoError(t, s.svc.ConfirmMFAChallenge(ctx, p, id, a.Get(c.WebAuthnOptions)))
	_, ok := <-notified
	assert.False(t, ok, "no gateway waits for a step-up challenge")
	assert.Equal(t, 1, countRows(t, s.pool, "SELECT count(*) FROM mfa_verification WHERE binding = $1 AND method = 'webauthn'", cli))

	agent := uuid.New()
	p.AgentID, p.RunID = &agent, &agent
	_, err = s.svc.BeginStepUp(ctx, p)
	require.ErrorIs(t, err, apperr.ErrPermissionDenied, "agents never step up")
	require.ErrorIs(t, s.svc.CompleteStepUp(ctx, p, MFAProof{TOTP: "123456"}), apperr.ErrPermissionDenied)
}

func TestVerifyTOTP_AttemptLimit(t *testing.T) {
	svc, p := stepUpUser(t)
	secret := dbtest.Fixture{}.TOTP(t, svc.pool, svc.cfg.Box, p.UserID)
	ctx := context.Background()
	step := currentStep()

	for range MFAAttempts {
		require.ErrorIs(t, svc.CompleteStepUp(ctx, p, MFAProof{TOTP: wrongCode(t, secret, step)}), ErrInvalidCode)
	}
	err := svc.CompleteStepUp(ctx, p, MFAProof{TOTP: codeAt(t, secret, step)})
	require.ErrorIs(t, err, apperr.ErrResourceExhausted, "the 6th attempt in the window is refused, even with a valid code")
	assert.Nil(t, sessionMFAAt(t, svc, p))
}

func TestFailedAttemptCountSurvivesRollback(t *testing.T) {
	svc, p := stepUpUser(t)
	secret := dbtest.Fixture{}.TOTP(t, svc.pool, svc.cfg.Box, p.UserID)
	attempts := func() int {
		t.Helper()
		return countRows(t, svc.pool, "SELECT coalesce(sum(count), 0) FROM rate_limit WHERE key = $1", "mfa:"+p.UserID.String())
	}

	require.ErrorIs(t, svc.CompleteStepUp(context.Background(), p, MFAProof{TOTP: wrongCode(t, secret, currentStep())}), ErrInvalidCode)
	assert.Equal(t, 1, attempts(), "the failed attempt stays counted")
	assert.Zero(t, countRows(t, svc.pool, "SELECT count(*) FROM mfa_verification"), "its effect rolled back")
	assert.Nil(t, sessionMFAAt(t, svc, p))
}
