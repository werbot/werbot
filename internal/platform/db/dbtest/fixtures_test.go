package dbtest_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/platform/db/dbtest"
	"github.com/werbot/werbot/internal/platform/principal"
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
