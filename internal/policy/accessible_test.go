package policy

import (
	"context"
	"log/slog"
	"net/netip"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type teamOf map[uuid.UUID][]uuid.UUID

func (t teamOf) UserTeams(_ context.Context, _, user uuid.UUID) ([]uuid.UUID, error) {
	return t[user], nil
}

func TestAccessibleResources(t *testing.T) {
	e := setup(t)
	alice, team := uuid.New(), uuid.New()
	e.svc = New(Deps{Pool: e.pool, Resources: e.res, Bindings: e.binds, Teams: teamOf{alice: {team}}, Logger: slog.New(slog.DiscardHandler)})
	r1, r2, r3 := e.resource(t, e.org), e.resource(t, e.org), e.resource(t, e.org)
	e.bind(e.org, alice, r1)
	e.binds[e.org] = append(e.binds[e.org], Binding{ID: uuid.New(), SubjectType: "team", SubjectID: team, ResourceID: &r2})
	at := time.Date(2026, 11, 18, 10, 0, 0, 0, time.UTC) // Wednesday

	got, err := e.svc.AccessibleResources(e.ctx, e.org, alice, ip10, at)
	require.NoError(t, err)
	assert.Equal(t, []uuid.UUID{r1, r2}, got)
	assert.NotContains(t, got, r3)

	require.NoError(t, e.svc.SetRules(e.ctx, e.org, ResourceRules{ResourceID: r2,
		Networks: []NetworkRule{{CIDR: netip.MustParsePrefix("10.0.0.0/8"), Effect: EffectDeny}}}))
	got, err = e.svc.AccessibleResources(e.ctx, e.org, alice, ip10, at)
	require.NoError(t, err)
	assert.Equal(t, []uuid.UUID{r1}, got, "network deny")

	require.NoError(t, e.svc.SetRules(e.ctx, e.org, ResourceRules{ResourceID: r1,
		Schedule: []Slot{{Weekday: int(time.Friday), Hour: 10}}}))
	got, err = e.svc.AccessibleResources(e.ctx, e.org, alice, ip10, at)
	require.NoError(t, err)
	assert.Empty(t, got, "outside the schedule")

	_, err = e.pool.Exec(context.Background(), "INSERT INTO ip_blocklist (cidr, source) VALUES ('203.0.113.0/24', 'test')")
	require.NoError(t, err)
	got, err = e.svc.AccessibleResources(e.ctx, e.org, alice, netip.MustParseAddr("203.0.113.7"), at)
	require.NoError(t, err)
	assert.Empty(t, got, "blocklist")
}

func TestAccessibleResourcesCompileErrorDenies(t *testing.T) {
	e := setup(t)
	r := e.resource(t, e.org)
	e.binds[e.org] = []Binding{{ID: uuid.New(), SubjectType: "robot", SubjectID: e.owner, ResourceID: &r}}
	got, err := e.svc.AccessibleResources(e.ctx, e.org, e.owner, ip10, time.Time{})
	require.NoError(t, err)
	assert.Empty(t, got)
}
