package identity

import (
	"bytes"
	"context"
	"fmt"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/platform/apperr"
	"github.com/werbot/werbot/internal/platform/db/dbtest"
	"github.com/werbot/werbot/internal/platform/dbx"
	"github.com/werbot/werbot/internal/platform/principal"
)

// newMFATest returns a service and alice@example.com, a member of a new organization.
func newMFATest(t *testing.T) (*Service, *pgxpool.Pool, principal.Principal) {
	t.Helper()
	svc, pool, _ := newTestService(t)
	org := seedOrg(t, pool, "Acme")
	id := seedUser(t, pool, org, "alice", principal.RoleMember)
	return svc, pool, principal.Principal{UserID: id, OrgID: org, Role: principal.RoleMember}
}

// codeAt is the TOTP code of secret for the 30-second step.
func codeAt(t *testing.T, secret string, step int64) string {
	t.Helper()
	code, err := totp.GenerateCodeCustom(secret, time.Unix(step*30, 0),
		totp.ValidateOpts{Period: 30, Digits: otp.DigitsSix, Algorithm: otp.AlgorithmSHA1})
	require.NoError(t, err)
	return code
}

// currentStep waits out the last 3 seconds of a step, so the steps a test
// derives from it match the ones the service sees.
func currentStep() int64 {
	if left := 30 - time.Now().Unix()%30; left <= 3 {
		time.Sleep(time.Duration(left+1) * time.Second)
	}
	return time.Now().Unix() / 30
}

// wrongCode is a code that no step in the accepted window yields.
func wrongCode(t *testing.T, secret string, step int64) string {
	t.Helper()
	window := []string{codeAt(t, secret, step-1), codeAt(t, secret, step), codeAt(t, secret, step+1)}
	for i := 0; ; i++ {
		if c := fmt.Sprintf("%06d", i); !slices.Contains(window, c) {
			return c
		}
	}
}

// checkInTx runs checkTOTP in its own transaction, as the callers do.
func checkInTx(svc *Service, p principal.Principal, code string) error {
	return dbx.InOrg(context.Background(), svc.pool, p.OrgID, p.UserID, func(tx pgx.Tx) error {
		return svc.checkTOTP(context.Background(), tx, p.UserID, code)
	})
}

// countAudit counts audit events of action whose data contains the JSON object data.
func countAudit(t *testing.T, pool *pgxpool.Pool, action, data string) int {
	t.Helper()
	var n int
	require.NoError(t, pool.QueryRow(context.Background(),
		"SELECT count(*) FROM audit_event WHERE action = $1 AND data @> $2::jsonb", action, data).Scan(&n))
	return n
}

func TestEnrollConfirmTOTP(t *testing.T) {
	svc, pool, p := newMFATest(t)
	ctx := context.Background()

	first, err := svc.EnrollTOTP(ctx, p)
	require.NoError(t, err)
	e, err := svc.EnrollTOTP(ctx, p)
	require.NoError(t, err, "an unconfirmed enrollment is replaced")
	assert.NotEqual(t, first.Secret, e.Secret)
	assert.Len(t, e.Secret, 32, "20 bytes in unpadded base32")
	assert.True(t, strings.HasPrefix(e.URL, "otpauth://totp/Werbot:alice@example.com?"), e.URL)
	assert.Contains(t, e.URL, "issuer=Werbot")
	assert.Contains(t, e.URL, "secret="+e.Secret)
	assert.True(t, strings.HasPrefix(e.QRPath, "M"))
	assert.Positive(t, e.QRSize)

	var ct []byte
	require.NoError(t, pool.QueryRow(ctx, "SELECT ciphertext FROM totp_secret WHERE user_id = $1", p.UserID).Scan(&ct))
	assert.False(t, bytes.Contains(ct, []byte(e.Secret)), "the secret is stored sealed")

	step := currentStep()
	require.ErrorIs(t, svc.ConfirmTOTP(ctx, p, wrongCode(t, e.Secret, step)), ErrInvalidCode)
	assert.Equal(t, 1, countAudit(t, pool, "auth.mfa.fail", `{"method":"totp"}`))
	fs, err := svc.ListFactors(ctx, p)
	require.NoError(t, err)
	assert.Empty(t, fs, "an unconfirmed secret is no factor")

	require.NoError(t, svc.ConfirmTOTP(ctx, p, codeAt(t, e.Secret, step)))
	var confirmed bool
	require.NoError(t, pool.QueryRow(ctx, "SELECT confirmed_at IS NOT NULL FROM totp_secret WHERE user_id = $1", p.UserID).Scan(&confirmed))
	assert.True(t, confirmed)
	assert.Equal(t, 1, countAudit(t, pool, "mfa.factor.add", `{"kind":"totp"}`))

	_, err = svc.EnrollTOTP(ctx, p)
	require.ErrorIs(t, err, apperr.ErrAlreadyExists)
	require.ErrorIs(t, svc.ConfirmTOTP(ctx, p, codeAt(t, e.Secret, step+1)), apperr.ErrFailedPrecondition, "nothing left to confirm")
}

func TestVerifyTOTP_AcceptsAdjacentSteps(t *testing.T) {
	svc, pool, p := newMFATest(t)
	secret := dbtest.Fixture{}.TOTP(t, pool, svc.cfg.Box, p.UserID)
	now := currentStep()

	require.ErrorIs(t, checkInTx(svc, p, codeAt(t, secret, now-2)), ErrInvalidCode)
	require.ErrorIs(t, checkInTx(svc, p, codeAt(t, secret, now+2)), ErrInvalidCode)
	for _, step := range []int64{now - 1, now, now + 1} {
		require.NoError(t, checkInTx(svc, p, codeAt(t, secret, step)), "step %+d", step-now)
	}
}

func TestVerifyTOTP_RejectsReuseOfSameCode(t *testing.T) {
	svc, pool, p := newMFATest(t)
	secret := dbtest.Fixture{}.TOTP(t, pool, svc.cfg.Box, p.UserID)
	now := currentStep()

	require.NoError(t, checkInTx(svc, p, codeAt(t, secret, now)))
	require.ErrorIs(t, checkInTx(svc, p, codeAt(t, secret, now)), ErrInvalidCode, "same step twice")
	require.NoError(t, checkInTx(svc, p, codeAt(t, secret, now+1)))
	require.ErrorIs(t, checkInTx(svc, p, codeAt(t, secret, now-1)), ErrInvalidCode, "older step after a newer one")
}

func TestVerifyTOTP_ConcurrentSameCode(t *testing.T) {
	svc, pool, p := newMFATest(t)
	secret := dbtest.Fixture{}.TOTP(t, pool, svc.cfg.Box, p.UserID)
	code := codeAt(t, secret, currentStep())

	errs := make([]error, 10)
	var wg sync.WaitGroup
	for i := range errs {
		wg.Go(func() { errs[i] = checkInTx(svc, p, code) })
	}
	wg.Wait()
	ok := 0
	for _, err := range errs {
		if err == nil {
			ok++
		} else {
			require.ErrorIs(t, err, ErrInvalidCode)
		}
	}
	assert.Equal(t, 1, ok)
}

func TestConfirmTOTPAttemptLimit(t *testing.T) {
	svc, pool, p := newMFATest(t)
	ctx := context.Background()
	e, err := svc.EnrollTOTP(ctx, p)
	require.NoError(t, err)
	step := currentStep()

	for range MFAAttempts {
		require.ErrorIs(t, svc.ConfirmTOTP(ctx, p, wrongCode(t, e.Secret, step)), ErrInvalidCode)
	}
	require.ErrorIs(t, svc.ConfirmTOTP(ctx, p, codeAt(t, e.Secret, step)), apperr.ErrResourceExhausted,
		"the 6th attempt in the window is refused, even with a valid code")
	var confirmed bool
	require.NoError(t, pool.QueryRow(ctx, "SELECT confirmed_at IS NOT NULL FROM totp_secret WHERE user_id = $1", p.UserID).Scan(&confirmed))
	assert.False(t, confirmed)
}

func TestQRPath(t *testing.T) {
	url := "otpauth://totp/Werbot:alice%40example.com?algorithm=SHA1&digits=6&issuer=Werbot&period=30&secret=JBSWY3DPEHPK3PXPJBSWY3DPEHPK3PXP"
	size, path, err := qrPath(url)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, size, 29)
	assert.LessOrEqual(t, size, 57)
	assert.True(t, strings.HasPrefix(path, "M"))
	size2, path2, err := qrPath(url)
	require.NoError(t, err)
	assert.Equal(t, size, size2)
	assert.Equal(t, path, path2)
}
