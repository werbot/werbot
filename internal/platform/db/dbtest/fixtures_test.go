package dbtest_test

import (
	"bytes"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/platform/db/dbtest"
	"github.com/werbot/werbot/internal/platform/principal"
	"github.com/werbot/werbot/internal/platform/secretbox"
)

func TestSeed(t *testing.T) {
	pool, _ := dbtest.New(t)
	f := dbtest.Seed(t, pool)
	ctx := t.Context()

	var resGW uuid.UUID
	require.NoError(t, pool.QueryRow(ctx, "SELECT gateway_id FROM resource WHERE id = $1 AND org_id = $2", f.ResourceID, f.OrgID).Scan(&resGW))
	assert.Equal(t, f.GatewayID, resGW)
	for inst, gw := range map[uuid.UUID]uuid.UUID{f.InstanceID: f.GatewayID, f.OtherInstanceID: f.OtherGatewayID} {
		var got uuid.UUID
		require.NoError(t, pool.QueryRow(ctx, "SELECT gateway_id FROM gateway_instance WHERE id = $1 AND org_id = $2", inst, f.OrgID).Scan(&got))
		assert.Equal(t, gw, got)
	}

	sid := f.StartSession(t, pool, f.MemberID)
	var user, inst uuid.UUID
	require.NoError(t, pool.QueryRow(ctx,
		"SELECT user_id, gateway_instance_id FROM session WHERE id = $1 AND org_id = $2 AND gateway_id = $3 AND resource_id = $4 AND ended_at IS NULL",
		sid, f.OrgID, f.GatewayID, f.ResourceID).Scan(&user, &inst))
	assert.Equal(t, f.MemberID, user)
	assert.Equal(t, f.InstanceID, inst)

	for id, role := range map[uuid.UUID]string{
		f.OwnerID: principal.RoleOwner, f.AdminID: principal.RoleAdmin,
		f.MemberID: principal.RoleMember, f.OtherMemberID: principal.RoleMember,
	} {
		assert.Equal(t, principal.Principal{UserID: id, OrgID: f.OrgID, Role: role}, f.Principal(id))
		var dbRole string
		require.NoError(t, pool.QueryRow(ctx, "SELECT role FROM membership WHERE org_id = $1 AND user_id = $2", f.OrgID, id).Scan(&dbRole))
		assert.Equal(t, role, dbRole)
	}
}

func TestTOTPFixture(t *testing.T) {
	pool, _ := dbtest.New(t)
	f := dbtest.Seed(t, pool)
	box, err := secretbox.New(bytes.Repeat([]byte{1}, 32), nil)
	require.NoError(t, err)

	secret := f.TOTP(t, pool, box, f.MemberID)
	assert.Regexp(t, `^[A-Z2-7]{32}$`, secret)
	var ciphertext []byte
	var keyID string
	var confirmed bool
	require.NoError(t, pool.QueryRow(t.Context(), "SELECT ciphertext, key_id, confirmed_at IS NOT NULL FROM totp_secret WHERE user_id = $1",
		f.MemberID).Scan(&ciphertext, &keyID, &confirmed))
	assert.True(t, confirmed)
	assert.NotContains(t, string(ciphertext), secret, "the secret is stored sealed")
	plain, err := box.Open(ciphertext, keyID)
	require.NoError(t, err)
	assert.Equal(t, secret, string(plain))
}

func TestMFARuleFixture(t *testing.T) {
	pool, _ := dbtest.New(t)
	f := dbtest.Seed(t, pool)
	id := f.MFARule(t, pool, "prod", `{"users":[]}`, `{"all":true}`, 90*time.Second, "webauthn")
	var org uuid.UUID
	var maxAge, strength, subjects string
	require.NoError(t, pool.QueryRow(t.Context(), "SELECT org_id, max_age::text, strength, subjects::text FROM mfa_rule WHERE id = $1 AND name = 'prod'",
		id).Scan(&org, &maxAge, &strength, &subjects))
	assert.Equal(t, f.OrgID, org)
	assert.Equal(t, "00:01:30", maxAge)
	assert.Equal(t, "webauthn", strength)
	assert.JSONEq(t, `{"users":[]}`, subjects)
}
