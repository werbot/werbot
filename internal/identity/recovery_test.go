package identity

import (
	"context"
	"crypto/sha256"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/platform/apperr"
	"github.com/werbot/werbot/internal/platform/db/dbtest"
	"github.com/werbot/werbot/internal/platform/dbx"
	"github.com/werbot/werbot/internal/platform/principal"
)

// recoverInTx runs useRecoveryCode in its own transaction, as the callers do.
func recoverInTx(svc *Service, p principal.Principal, code string) error {
	return dbx.InOrg(context.Background(), svc.pool, p.OrgID, p.UserID, func(tx pgx.Tx) error {
		return svc.useRecoveryCode(context.Background(), tx, p.UserID, code)
	})
}

// seedPasskey inserts a passkey row of userID and returns its id.
func seedPasskey(t *testing.T, pool *pgxpool.Pool, userID uuid.UUID, name string) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	require.NoError(t, pool.QueryRow(context.Background(),
		"INSERT INTO webauthn_credential (user_id, name, credential_id, public_key, aaguid) VALUES ($1, $2, $3, 'k', 'a') RETURNING id",
		userID, name, []byte(uuid.NewString())).Scan(&id))
	return id
}

func factorByID(fs []Factor, id string) (Factor, bool) {
	for _, f := range fs {
		if f.ID == id {
			return f, true
		}
	}
	return Factor{}, false
}

func TestRecoveryCodes(t *testing.T) {
	svc, pool, p := newMFATest(t)
	ctx := context.Background()

	codes, err := svc.GenerateRecoveryCodes(ctx, p)
	require.NoError(t, err)
	require.Len(t, codes, RecoveryCodeCount)
	seen := map[string]bool{}
	for _, c := range codes {
		assert.Regexp(t, `^[a-z2-7]{4}(-[a-z2-7]{4}){3}$`, c)
		seen[c] = true
	}
	assert.Len(t, seen, RecoveryCodeCount, "codes are distinct")

	h := sha256.Sum256([]byte(strings.ReplaceAll(codes[0], "-", "")))
	var stored, total int
	require.NoError(t, pool.QueryRow(ctx,
		"SELECT count(*) FILTER (WHERE code_hash = $2), count(*) FROM mfa_recovery_code WHERE user_id = $1",
		p.UserID, h[:]).Scan(&stored, &total))
	assert.Equal(t, 1, stored, "only the SHA-256 of a code is stored")
	assert.Equal(t, RecoveryCodeCount, total)

	again, err := svc.GenerateRecoveryCodes(ctx, p)
	require.NoError(t, err)
	require.ErrorIs(t, recoverInTx(svc, p, codes[0]), ErrInvalidCode, "a new set voids the old codes")
	require.NoError(t, recoverInTx(svc, p, again[0]))
	assert.Equal(t, 2, countAudit(t, pool, "mfa.factor.add", `{"kind":"recovery"}`))
}

func TestRecoveryCodeSingleUse(t *testing.T) {
	svc, _, p := newMFATest(t)
	ctx := context.Background()
	codes, err := svc.GenerateRecoveryCodes(ctx, p)
	require.NoError(t, err)

	typed := " " + strings.ToUpper(strings.ReplaceAll(codes[3], "-", " - ")) + " "
	require.NoError(t, recoverInTx(svc, p, typed), "case, spaces and dashes do not matter")
	require.ErrorIs(t, recoverInTx(svc, p, codes[3]), ErrInvalidCode, "a code works once")
	require.ErrorIs(t, recoverInTx(svc, p, ""), ErrInvalidCode)

	fs, err := svc.ListFactors(ctx, p)
	require.NoError(t, err)
	f, ok := factorByID(fs, "recovery")
	require.True(t, ok)
	assert.Equal(t, RecoveryCodeCount-1, f.Remaining)
}

func TestListAndDeleteFactors(t *testing.T) {
	svc, pool, p := newMFATest(t)
	ctx := context.Background()
	dbtest.Fixture{}.TOTP(t, pool, svc.cfg.Box, p.UserID)
	_, err := svc.GenerateRecoveryCodes(ctx, p)
	require.NoError(t, err)
	key := seedPasskey(t, pool, p.UserID, "YubiKey")
	other := seedPasskey(t, pool, seedUser(t, pool, p.OrgID, "bob", principal.RoleMember), "Bob's key")

	fs, err := svc.ListFactors(ctx, p)
	require.NoError(t, err)
	require.Len(t, fs, 3)
	assert.Equal(t, Factor{ID: key.String(), Kind: "webauthn", Name: "YubiKey", CreatedAt: fs[0].CreatedAt}, fs[0])
	totpF, _ := factorByID(fs, "totp")
	assert.Equal(t, "totp", totpF.Kind)
	assert.False(t, totpF.CreatedAt.IsZero())
	rec, _ := factorByID(fs, "recovery")
	assert.Equal(t, RecoveryCodeCount, rec.Remaining)

	require.NoError(t, svc.DeleteFactor(ctx, p, "totp"))
	var n int
	require.NoError(t, pool.QueryRow(ctx, "SELECT count(*) FROM totp_secret WHERE user_id = $1", p.UserID).Scan(&n))
	assert.Zero(t, n)
	assert.Equal(t, 1, countAudit(t, pool, "mfa.factor.delete", `{"kind":"totp"}`))

	require.ErrorIs(t, svc.DeleteFactor(ctx, p, "totp"), apperr.ErrNotFound)
	require.ErrorIs(t, svc.DeleteFactor(ctx, p, other.String()), apperr.ErrNotFound, "another user's passkey")
	require.ErrorIs(t, svc.DeleteFactor(ctx, p, "sms"), apperr.ErrInvalidArgument)
	require.NoError(t, svc.DeleteFactor(ctx, p, key.String()))
	require.NoError(t, svc.DeleteFactor(ctx, p, "recovery"))
	assert.Equal(t, 1, countAudit(t, pool, "mfa.factor.delete", `{"kind":"webauthn"}`))
	assert.Equal(t, 1, countAudit(t, pool, "mfa.factor.delete", `{"kind":"recovery"}`))

	fs, err = svc.ListFactors(ctx, p)
	require.NoError(t, err)
	assert.Empty(t, fs)
}

func TestDeleteLastFactorWhenRequired(t *testing.T) {
	svc, pool, p := newMFATest(t)
	ctx := context.Background()
	dbtest.Fixture{}.TOTP(t, pool, svc.cfg.Box, p.UserID)
	setRequire := func(v string) {
		_, err := pool.Exec(ctx, "UPDATE org_setting SET require_mfa = $1 WHERE org_id = $2", v, p.OrgID)
		require.NoError(t, err)
	}

	setRequire("all")
	err := svc.DeleteFactor(ctx, p, "totp")
	require.ErrorIs(t, err, apperr.ErrFailedPrecondition)
	assert.ErrorContains(t, err, "mfa is required by the organization")
	assert.Zero(t, countAudit(t, pool, "mfa.factor.delete", `{"kind":"totp"}`))

	key := seedPasskey(t, pool, p.UserID, "YubiKey")
	require.NoError(t, svc.DeleteFactor(ctx, p, "totp"), "another factor is left")
	require.ErrorIs(t, svc.DeleteFactor(ctx, p, key.String()), apperr.ErrFailedPrecondition)

	setRequire("admins")
	require.NoError(t, svc.DeleteFactor(ctx, p, key.String()), "admins only: a member may drop the last factor")
}
