package inventory_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/inventory"
)

// healthEnv is a pool with one resource and n online instances.
type healthEnv struct {
	gwEnv
	gwPool inventory.Gateway
	res    inventory.Resource
	inst   []uuid.UUID
}

func healthSetup(t *testing.T, n int) healthEnv {
	t.Helper()
	e := gwSetup(t)
	g := e.newPool(t, "edge")
	h := healthEnv{gwEnv: e, gwPool: g}
	r, err := e.svc.Create(e.ctx, e.org, inventory.CreateInput{Name: "web", Type: "ssh", Config: sshCfg("h"), GatewayID: &g.ID})
	require.NoError(t, err)
	h.res = r
	for i := range n {
		en := e.enroll(t, e.token(t, g.ID, 0), "gw-"+string(rune('a'+i)))
		require.NoError(t, e.gw.Heartbeat(context.Background(), en.InstanceID, "v", nil))
		h.inst = append(h.inst, en.InstanceID)
	}
	return h
}

func (h healthEnv) report(t *testing.T, inst uuid.UUID, ok bool, msg string, age time.Duration) {
	t.Helper()
	require.NoError(t, h.svc.ReportHealth(h.ctx, h.org, inst, []inventory.Health{
		{ResourceID: h.res.ID, OK: ok, Error: msg, CheckedAt: time.Now().Add(-age)}}))
}

func (h healthEnv) state(t *testing.T) inventory.HealthStatus {
	t.Helper()
	r, err := h.svc.Get(h.ctx, h.org, h.res.ID)
	require.NoError(t, err)
	list, err := h.svc.List(h.ctx, h.org, nil)
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, r.Health, list[0].Health, "Get and List agree")
	return r.Health
}

func TestReportHealthAndState(t *testing.T) {
	h := healthSetup(t, 2)
	assert.Equal(t, inventory.HealthUnknown, h.state(t).State)
	assert.Nil(t, h.state(t).CheckedAt)

	h.report(t, h.inst[0], true, "", 0)
	h.report(t, h.inst[1], false, "dial: refused", 0)
	assert.Equal(t, inventory.HealthHealthy, h.state(t).State, "one ok is enough")

	h.report(t, h.inst[0], false, "timeout", time.Second)
	got := h.state(t)
	assert.Equal(t, inventory.HealthUnhealthy, got.State)
	assert.Equal(t, "dial: refused", got.Error, "the error of the freshest failure")
	assert.NotNil(t, got.CheckedAt)

	h.report(t, h.inst[0], true, "", 4*time.Minute)
	h.report(t, h.inst[1], false, "x", 4*time.Minute)
	assert.Equal(t, inventory.HealthUnknown, h.state(t).State, "older than 3 min")

	h.report(t, h.inst[0], true, "", 0)
	_, err := h.pool.Exec(context.Background(), "UPDATE gateway_instance SET last_seen_at = now() - interval '2 minutes' WHERE id = $1", h.inst[0])
	require.NoError(t, err)
	h.report(t, h.inst[1], false, "down", 0)
	assert.Equal(t, inventory.HealthUnhealthy, h.state(t).State, "an offline instance does not count")
}

func TestReportHealthSkipsForeign(t *testing.T) {
	h := healthSetup(t, 1)
	other := h.newPool(t, "other")
	enOther := h.enroll(t, h.token(t, other.ID, 0), "gw-x")
	_, org2 := h.otherOrg(t)
	count := func() int { return h.count(t, "SELECT count(*) FROM resource_health") }

	h.report(t, enOther.InstanceID, true, "", 0) // instance of another pool
	assert.Equal(t, 0, count())
	require.NoError(t, h.svc.ReportHealth(h.ctx, org2, h.inst[0], []inventory.Health{{ResourceID: h.res.ID, OK: true, CheckedAt: time.Now()}}))
	assert.Equal(t, 0, count(), "another organization")

	now := time.Now()
	require.NoError(t, h.svc.ReportHealth(h.ctx, h.org, h.inst[0], []inventory.Health{
		{ResourceID: h.res.ID, OK: false, Error: "old", CheckedAt: now.Add(-time.Minute)},
		{ResourceID: uuid.New(), OK: true, CheckedAt: now},
		{ResourceID: h.res.ID, OK: true, CheckedAt: now}}))
	assert.Equal(t, 1, count(), "unknown resource skipped, repeated id collapsed")
	assert.Equal(t, inventory.HealthHealthy, h.state(t).State, "the newest wins")
}

func TestOnlineInstances(t *testing.T) {
	h := healthSetup(t, 2)
	ctx := context.Background()
	ids, err := h.gw.OnlineInstances(ctx, h.gwPool.ID)
	require.NoError(t, err)
	assert.ElementsMatch(t, h.inst, ids)

	_, err = h.pool.Exec(ctx, "UPDATE gateway_instance SET last_seen_at = now() - interval '46 seconds' WHERE id = $1", h.inst[0])
	require.NoError(t, err)
	ids, err = h.gw.OnlineInstances(ctx, h.gwPool.ID)
	require.NoError(t, err)
	assert.Equal(t, []uuid.UUID{h.inst[1]}, ids)

	ids, err = h.gw.OnlineInstances(ctx, uuid.New())
	require.NoError(t, err)
	assert.Empty(t, ids)
}
