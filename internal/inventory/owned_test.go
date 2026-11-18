package inventory_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/inventory"
	"github.com/werbot/werbot/internal/platform/db/dbtest"
)

func TestOwnedBy(t *testing.T) {
	e := gwSetup(t)
	poolA, poolA2 := e.newPool(t, "a"), e.newPool(t, "a2")
	res := func(ctx context.Context, org uuid.UUID, name string, pool uuid.UUID) uuid.UUID {
		r, err := e.svc.Create(ctx, org, inventory.CreateInput{Name: name, Type: "ssh", Config: sshCfg("h"), GatewayID: &pool})
		require.NoError(t, err)
		return r.ID
	}
	own := res(e.ctx, e.org, "own", poolA.ID)
	other := res(e.ctx, e.org, "other", poolA2.ID)
	ctxB, orgB := e.otherOrg(t)
	poolB := dbtest.Gateway(t, e.pool, orgB, "b")
	foreign := res(ctxB, orgB, "foreign", poolB)

	for _, c := range []struct {
		name                string
		org, pool, resource uuid.UUID
		want                bool
	}{
		{"owned", e.org, poolA.ID, own, true},
		{"another pool's resource", e.org, poolA.ID, other, false},
		{"another org's resource", e.org, poolA.ID, foreign, false},
		{"another org's resource in its pool", e.org, poolB, foreign, false},
		{"unknown id", e.org, poolA.ID, uuid.New(), false},
	} {
		got, err := e.svc.OwnedBy(e.ctx, c.org, c.pool, c.resource)
		require.NoError(t, err, c.name)
		assert.Equal(t, c.want, got, c.name)
	}
}
