package app

import (
	"context"
	"log/slog"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	sshv1 "github.com/werbot/werbot/gen/werbot/connector/ssh/v1"
	"github.com/werbot/werbot/internal/inventory"
	"github.com/werbot/werbot/internal/platform/db/dbtest"
	"github.com/werbot/werbot/internal/platform/principal"
)

// The policy adapter gives each resource its endpoint host (the resource host of the egress class).
func TestPolicyAdaptersHost(t *testing.T) {
	s := newStage4(t, slog.New(slog.DiscardHandler))
	f := dbtest.Seed(t, s.pool)
	ctx := principal.With(context.Background(), f.Principal(f.OwnerID))
	r, err := s.d.Inventory.Create(ctx, f.OrgID, inventory.CreateInput{Name: "db", Type: "ssh", GatewayID: &f.GatewayID,
		Config: &sshv1.Config{Host: "db.internal", Login: "root"}})
	require.NoError(t, err)

	rs, err := policyResources{s.d.Inventory}.PolicyResources(ctx, f.OrgID)
	require.NoError(t, err)
	hosts := map[uuid.UUID]string{}
	for _, x := range rs {
		hosts[x.ID] = x.Host
	}
	assert.Equal(t, map[uuid.UUID]string{r.ID: "db.internal", f.ResourceID: ""}, hosts)
}
