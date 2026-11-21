package identity

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/platform/apperr"
	"github.com/werbot/werbot/internal/platform/principal"
)

func TestUpdateSettingsMaskOutput(t *testing.T) {
	svc, pool, _ := newTestService(t)
	seedUser(t, pool, seedOrg(t, pool, "Acme"), "owner", principal.RoleOwner)
	ctx, _ := loginCtx(t, svc, "owner")
	p, _ := principal.Require(ctx)

	got, err := svc.Settings(context.Background(), p.OrgID)
	require.NoError(t, err)
	assert.Equal(t, "agents", got.MaskOutput, "new organization")

	got, err = svc.UpdateSettings(ctx, p, OrgSettings{MaskOutput: "all"}, []string{"mask_output"})
	require.NoError(t, err)
	assert.Equal(t, "all", got.MaskOutput)
	got, err = svc.Settings(context.Background(), p.OrgID)
	require.NoError(t, err)
	assert.Equal(t, "all", got.MaskOutput)

	for _, v := range []string{"some", ""} {
		_, err = svc.UpdateSettings(ctx, p, OrgSettings{MaskOutput: v}, []string{"mask_output"})
		assert.ErrorIs(t, err, apperr.ErrInvalidArgument, v)
	}
	got, err = svc.Settings(context.Background(), p.OrgID)
	require.NoError(t, err)
	assert.Equal(t, "all", got.MaskOutput, "failed updates change nothing")
}
