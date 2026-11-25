package access_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/access"
	"github.com/werbot/werbot/internal/platform/apperr"
	"github.com/werbot/werbot/internal/platform/db/dbtest"
)

// passkey inserts a passkey of user.
func passkey(t *testing.T, pool *pgxpool.Pool, user uuid.UUID) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	require.NoError(t, pool.QueryRow(bg, `INSERT INTO webauthn_credential (user_id, name, credential_id, public_key, aaguid)
		VALUES ($1, 'YubiKey', uuid_send(gen_random_uuid()), '\x00', '\x00') RETURNING id`, user).Scan(&id))
	return id
}

// TestCreateLockMFADevice: an owner or admin locks a passkey of a member of
// the organization; a passkey of a user of another organization, an unknown
// id and a lock without an id are refused.
func TestCreateLockMFADevice(t *testing.T) {
	svc, pool, f, _, _ := newTestService(t)
	admin := f.Principal(f.AdminID)
	key := passkey(t, pool, f.MemberID)
	l, err := svc.CreateLock(bg, admin, access.LockInput{TargetType: "mfa_device", TargetID: &key, Reason: "lost key"})
	require.NoError(t, err)
	assert.Equal(t, "mfa_device", l.TargetType)
	assert.Equal(t, &key, l.TargetID)
	assert.Equal(t, []map[string]any{{"target_type": "mfa_device", "target_id": key.String()}}, eventData(t, pool, "lock.create", l.ID))

	other := dbtest.Seed(t, pool)
	for name, id := range map[string]uuid.UUID{
		"passkey of another org's user": passkey(t, pool, other.MemberID),
		"unknown id":                    uuid.New(),
		"a user id":                     f.MemberID,
	} {
		_, err := svc.CreateLock(bg, admin, access.LockInput{TargetType: "mfa_device", TargetID: &id, Reason: "x"})
		require.ErrorIs(t, err, apperr.ErrNotFound, name)
	}
	_, err = svc.CreateLock(bg, admin, access.LockInput{TargetType: "mfa_device", Reason: "x"})
	require.ErrorIs(t, err, apperr.ErrInvalidArgument, "no id")
	_, err = svc.CreateLock(bg, f.Principal(f.MemberID), access.LockInput{TargetType: "mfa_device", TargetID: &key, Reason: "x"})
	require.ErrorIs(t, err, apperr.ErrPermissionDenied, "a member never locks")
}
