package identity

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/identity/webauthntest"
	"github.com/werbot/werbot/internal/platform/apperr"
	"github.com/werbot/werbot/internal/platform/db/dbtest"
	"github.com/werbot/werbot/internal/platform/principal"
)

// loginMFA logs alice in with her password; she must be asked for a second factor.
func loginMFA(t *testing.T, svc *Service) LoginResult {
	t.Helper()
	res, err := svc.Login(context.Background(), "alice@example.com", testPassword, "test")
	require.NoError(t, err)
	require.Empty(t, res.Token, "no session before the second factor")
	require.NotEmpty(t, res.MFAToken)
	return res
}

func TestLoginWithoutFactor(t *testing.T) {
	svc, pool, p := newMFATest(t)
	ctx := context.Background()

	res, err := svc.Login(ctx, "alice@example.com", testPassword, "test")
	require.NoError(t, err)
	assert.NotEmpty(t, res.Token)
	assert.Empty(t, res.MFAToken)
	assert.Empty(t, res.Methods)
	got, err := svc.AuthenticateSession(ctx, res.Token)
	require.NoError(t, err)
	assert.Nil(t, got.MFAAt, "no factor was checked")
	assert.Equal(t, 1, countAudit(t, pool, "auth.login", `{"method":"password"}`))
	assert.Zero(t, countRows(t, pool, "SELECT count(*) FROM audit_event WHERE action = 'auth.login' AND data ? 'mfa'"))

	_, err = svc.GenerateRecoveryCodes(ctx, p)
	require.NoError(t, err)
	res, err = svc.Login(ctx, "alice@example.com", testPassword, "test")
	require.NoError(t, err)
	assert.NotEmpty(t, res.Token, "recovery codes alone are no factor")
}

func TestLoginRequiresSecondFactor(t *testing.T) {
	svc, pool, p := newMFATest(t)
	dbtest.Fixture{}.TOTP(t, pool, svc.cfg.Box, p.UserID)

	res := loginMFA(t, svc)
	assert.Len(t, res.MFAToken, 43)
	assert.Equal(t, []string{"totp"}, res.Methods)
	assert.Empty(t, res.WebAuthnOptions)

	_, err := svc.GenerateRecoveryCodes(context.Background(), p)
	require.NoError(t, err)
	res = loginMFA(t, svc)
	assert.Equal(t, []string{"totp", "recovery"}, res.Methods)

	assert.Zero(t, countRows(t, pool, "SELECT count(*) FROM user_session"), "a password alone opens no session")
	assert.Zero(t, countRows(t, pool, "SELECT count(*) FROM audit_event WHERE action = 'auth.login'"), "written when the login completes")
	assert.Equal(t, 2, countRows(t, pool, `SELECT count(*) FROM one_time_token WHERE kind = 'mfa_login'
		AND expires_at > now() + interval '290 seconds' AND expires_at <= now() + interval '5 minutes'`))
	assert.Zero(t, countRows(t, pool, "SELECT count(*) FROM one_time_token WHERE token_hash = convert_to($1, 'UTF8')", res.MFAToken),
		"only the token's hash is stored")
}

func TestCompleteMFATOTP(t *testing.T) {
	svc, pool, p := newMFATest(t)
	secret := dbtest.Fixture{}.TOTP(t, pool, svc.cfg.Box, p.UserID)
	ctx := context.Background()
	res := loginMFA(t, svc)
	step := currentStep()

	token, err := svc.CompleteMFA(ctx, res.MFAToken, MFAProof{TOTP: codeAt(t, secret, step)}, "test")
	require.NoError(t, err)
	got, err := svc.AuthenticateSession(ctx, token)
	require.NoError(t, err)
	assert.Equal(t, p.UserID, got.UserID)
	require.NotNil(t, got.MFAAt)
	assert.WithinDuration(t, time.Now(), *got.MFAAt, time.Minute)
	assert.Equal(t, 1, countRows(t, pool, "SELECT count(*) FROM mfa_verification WHERE user_id = $1 AND binding = $2 AND method = 'totp'",
		p.UserID, "web:"+got.SessionID.String()))
	assert.Equal(t, 1, countAudit(t, pool, "auth.login", `{"method":"password","mfa":"totp"}`))

	_, err = svc.CompleteMFA(ctx, res.MFAToken, MFAProof{TOTP: codeAt(t, secret, step+1)}, "test")
	require.ErrorIs(t, err, apperr.ErrNotFound, "the mfa_token is single use")
	assert.Equal(t, 1, countRows(t, pool, "SELECT count(*) FROM user_session"))
}

func TestCompleteMFAWrongCodeKeepsToken(t *testing.T) {
	svc, pool, p := newMFATest(t)
	secret := dbtest.Fixture{}.TOTP(t, pool, svc.cfg.Box, p.UserID)
	ctx := context.Background()
	res := loginMFA(t, svc)
	step := currentStep()

	_, err := svc.CompleteMFA(ctx, res.MFAToken, MFAProof{TOTP: wrongCode(t, secret, step)}, "test")
	require.ErrorIs(t, err, ErrInvalidCode)
	assert.Equal(t, 1, countAudit(t, pool, "auth.mfa.fail", `{"method":"totp"}`))
	assert.Zero(t, countRows(t, pool, "SELECT count(*) FROM user_session"))

	token, err := svc.CompleteMFA(ctx, res.MFAToken, MFAProof{TOTP: codeAt(t, secret, step)}, "test")
	require.NoError(t, err, "a wrong factor does not spend the mfa_token")
	assert.NotEmpty(t, token)
}

func TestCompleteMFAAttemptLimit(t *testing.T) {
	svc, pool, p := newMFATest(t)
	secret := dbtest.Fixture{}.TOTP(t, pool, svc.cfg.Box, p.UserID)
	ctx := context.Background()
	res := loginMFA(t, svc)
	step := currentStep()

	for range MFAAttempts {
		_, err := svc.CompleteMFA(ctx, res.MFAToken, MFAProof{TOTP: wrongCode(t, secret, step)}, "test")
		require.ErrorIs(t, err, ErrInvalidCode)
	}
	_, err := svc.CompleteMFA(ctx, res.MFAToken, MFAProof{TOTP: codeAt(t, secret, step)}, "test")
	require.ErrorIs(t, err, apperr.ErrResourceExhausted, "wrong factors count: the 6th attempt is refused, even with a valid code")
	assert.Zero(t, countRows(t, pool, "SELECT count(*) FROM user_session"))
}

func TestCompleteMFAWebAuthn(t *testing.T) {
	svc, pool, p := newMFATest(t)
	a := webauthntest.New(t, testOrigin)
	registerPasskey(t, svc, p, a)
	ctx := context.Background()

	res := loginMFA(t, svc)
	assert.Equal(t, []string{"webauthn"}, res.Methods)
	require.NotEmpty(t, res.WebAuthnOptions)

	_, err := svc.CompleteMFA(ctx, res.MFAToken, MFAProof{Assertion: withOrigin(a, "https://evil.test", res.WebAuthnOptions)}, "test")
	require.ErrorIs(t, err, ErrInvalidCode)
	assert.Equal(t, 1, countAudit(t, pool, "auth.mfa.fail", `{"method":"webauthn"}`))

	token, err := svc.CompleteMFA(ctx, res.MFAToken, MFAProof{Assertion: a.Get(res.WebAuthnOptions)}, "test")
	require.NoError(t, err, "the failed assertion spent neither the token nor the ceremony")
	got, err := svc.AuthenticateSession(ctx, token)
	require.NoError(t, err)
	require.NotNil(t, got.MFAAt)
	assert.Equal(t, 1, countRows(t, pool, "SELECT count(*) FROM mfa_verification WHERE binding = $1 AND method = 'webauthn'",
		"web:"+got.SessionID.String()))
	assert.Equal(t, 1, countAudit(t, pool, "auth.login", `{"method":"password","mfa":"webauthn"}`))
}

func TestCompleteMFARecovery(t *testing.T) {
	svc, pool, p := newMFATest(t)
	dbtest.Fixture{}.TOTP(t, pool, svc.cfg.Box, p.UserID)
	ctx := context.Background()
	codes, err := svc.GenerateRecoveryCodes(ctx, p)
	require.NoError(t, err)

	res := loginMFA(t, svc)
	token, err := svc.CompleteMFA(ctx, res.MFAToken, MFAProof{RecoveryCode: strings.ToUpper(codes[0])}, "test")
	require.NoError(t, err)
	got, err := svc.AuthenticateSession(ctx, token)
	require.NoError(t, err)
	require.NotNil(t, got.MFAAt)
	assert.Zero(t, countRows(t, pool, "SELECT count(*) FROM mfa_verification"), "a recovery code writes no verification")
	assert.Equal(t, 1, countAudit(t, pool, "auth.login", `{"method":"password","mfa":"recovery"}`))

	res = loginMFA(t, svc)
	_, err = svc.CompleteMFA(ctx, res.MFAToken, MFAProof{RecoveryCode: codes[0]}, "test")
	require.ErrorIs(t, err, ErrInvalidCode, "a recovery code works once")
	assert.Equal(t, 1, countAudit(t, pool, "auth.mfa.fail", `{"method":"recovery"}`))
}

func TestCompleteMFAProofIsOneFactor(t *testing.T) {
	svc, pool, p := newMFATest(t)
	dbtest.Fixture{}.TOTP(t, pool, svc.cfg.Box, p.UserID)
	res := loginMFA(t, svc)
	for _, proof := range []MFAProof{{}, {TOTP: "123456", RecoveryCode: "abcd"}} {
		_, err := svc.CompleteMFA(context.Background(), res.MFAToken, proof, "test")
		require.ErrorIs(t, err, apperr.ErrInvalidArgument, "%+v", proof)
	}
}

func TestMFATokenExpires(t *testing.T) {
	svc, pool, p := newMFATest(t)
	secret := dbtest.Fixture{}.TOTP(t, pool, svc.cfg.Box, p.UserID)
	ctx := context.Background()
	res := loginMFA(t, svc)

	_, err := pool.Exec(ctx, "UPDATE one_time_token SET expires_at = now() - interval '1 second' WHERE kind = 'mfa_login'")
	require.NoError(t, err)
	_, err = svc.CompleteMFA(ctx, res.MFAToken, MFAProof{TOTP: codeAt(t, secret, currentStep())}, "test")
	require.ErrorIs(t, err, apperr.ErrNotFound)
	_, err = svc.CompleteMFA(ctx, "not-a-token", MFAProof{TOTP: codeAt(t, secret, currentStep())}, "test")
	require.ErrorIs(t, err, apperr.ErrNotFound)
	assert.Zero(t, countRows(t, pool, "SELECT count(*) FROM totp_secret WHERE last_step > 0"), "refused before the factor check")
	assert.Zero(t, countRows(t, pool, "SELECT count(*) FROM user_session"))
}

func TestCompleteMFAOtherUsersFactor(t *testing.T) {
	svc, pool, alice := newMFATest(t)
	bob := principal.Principal{UserID: seedUser(t, pool, alice.OrgID, "bob", principal.RoleMember), OrgID: alice.OrgID, Role: principal.RoleMember}
	aliceSecret := dbtest.Fixture{}.TOTP(t, pool, svc.cfg.Box, alice.UserID)
	bobSecret := dbtest.Fixture{}.TOTP(t, pool, svc.cfg.Box, bob.UserID)
	ctx := context.Background()
	bobCodes, err := svc.GenerateRecoveryCodes(ctx, bob)
	require.NoError(t, err)
	res := loginMFA(t, svc)
	step := currentStep()

	_, err = svc.CompleteMFA(ctx, res.MFAToken, MFAProof{TOTP: codeAt(t, bobSecret, step)}, "test")
	require.ErrorIs(t, err, ErrInvalidCode, "the factor is checked against the token's user")
	_, err = svc.CompleteMFA(ctx, res.MFAToken, MFAProof{RecoveryCode: bobCodes[0]}, "test")
	require.ErrorIs(t, err, ErrInvalidCode)
	assert.Equal(t, 10, countRows(t, pool, "SELECT count(*) FROM mfa_recovery_code WHERE user_id = $1 AND used_at IS NULL", bob.UserID))
	assert.Zero(t, countRows(t, pool, "SELECT count(*) FROM user_session"))

	token, err := svc.CompleteMFA(ctx, res.MFAToken, MFAProof{TOTP: codeAt(t, aliceSecret, step)}, "test")
	require.NoError(t, err)
	got, err := svc.AuthenticateSession(ctx, token)
	require.NoError(t, err)
	assert.Equal(t, alice.UserID, got.UserID)
}

func TestAcceptInviteExistingUserWithFactor(t *testing.T) {
	svc, pool, q := newTestService(t)
	org1, org2 := seedOrg(t, pool, "Acme"), seedOrg(t, pool, "Other")
	seedUser(t, pool, org1, "alice", principal.RoleOwner)
	dave := seedUser(t, pool, org2, "dave", principal.RoleOwner)
	dbtest.Fixture{}.TOTP(t, pool, svc.cfg.Box, dave)
	ctx, _ := loginCtx(t, svc, "alice")
	raw := invite(ctx, t, svc, q, "dave@example.com", principal.RoleMember)

	token, err := svc.AcceptInvite(context.Background(), AcceptInput{Token: raw, Password: testPassword}, "test")
	require.NoError(t, err)
	assert.Empty(t, token, "a user with a factor signs in through the login page")
	assert.Equal(t, principal.RoleMember, memberRole(t, pool, org1, dave))
	assert.Zero(t, countRows(t, pool, "SELECT count(*) FROM user_session WHERE user_id = $1", dave))
}
