package identity

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/identity/webauthntest"
)

func TestWebAuthnCloneWarning(t *testing.T) {
	svc, pool, p := newMFATest(t)
	a := webauthntest.New(t, testOrigin)
	f := registerPasskey(t, svc, p, a)
	for range 2 {
		_, err := finishLogin(svc, p, "login", "", a.Get(beginLogin(t, svc, p, "login", "")))
		require.NoError(t, err)
	}
	a.SetSignCount(2)
	_, err := finishLogin(svc, p, "login", "", a.Get(beginLogin(t, svc, p, "login", "")))
	require.ErrorIs(t, err, ErrInvalidCode, "a sign count not above the stored one")
	assert.Equal(t, 2, countRows(t, pool, "SELECT sign_count FROM webauthn_credential WHERE id = $1", f.ID))

	// An authenticator without a counter (synced passkeys) always sends 0.
	z := webauthntest.New(t, testOrigin)
	z.BackupEligible = true
	registerPasskey(t, svc, p, z)
	for range 2 {
		z.SetSignCount(0)
		_, err := finishLogin(svc, p, "login", "", z.Get(beginLogin(t, svc, p, "login", "")))
		require.NoError(t, err)
	}
}

// TestWebAuthnBackupEligible: the backup eligibility of a passkey is stored at
// registration; a synced passkey (BE=1) logs in, an assertion whose BE flag
// differs from the stored one is refused.
func TestWebAuthnBackupEligible(t *testing.T) {
	svc, pool, p := newMFATest(t)
	for _, be := range []bool{true, false} {
		a := webauthntest.New(t, testOrigin)
		a.BackupEligible = be
		f := registerPasskey(t, svc, p, a)
		var stored bool
		require.NoError(t, pool.QueryRow(context.Background(), "SELECT backup_eligible FROM webauthn_credential WHERE id = $1", f.ID).Scan(&stored))
		assert.Equal(t, be, stored)
		_, err := finishLogin(svc, p, "login", "", a.Get(beginLogin(t, svc, p, "login", "")))
		require.NoError(t, err, "BE=%v", be)

		a.BackupEligible = !be
		_, err = finishLogin(svc, p, "login", "", a.Get(beginLogin(t, svc, p, "login", "")))
		require.ErrorIs(t, err, ErrInvalidCode, "BE changed to %v", !be)
	}
}
