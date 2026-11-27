package inventory_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/inventory"
)

// readonlySetup is healthSetup with an agent_readonly credential on the
// resource, its status pending.
func readonlySetup(t *testing.T) healthEnv {
	t.Helper()
	h := healthSetup(t, 1)
	_, err := h.pool.Exec(context.Background(), `INSERT INTO credential (org_id, resource_id, kind, ciphertext, key_id, slot)
		VALUES ($1, $2, 'password', '\x00', 'test', 'agent_readonly')`, h.org, h.res.ID)
	require.NoError(t, err)
	_, err = h.pool.Exec(context.Background(), "UPDATE resource SET agent_readonly_status = 'pending' WHERE id = $1", h.res.ID)
	require.NoError(t, err)
	return h
}

// version is the current check version: the later of the credential's and
// the resource's updated_at, in microseconds.
func (h healthEnv) version(t *testing.T) int64 {
	t.Helper()
	var cred, res time.Time
	require.NoError(t, h.pool.QueryRow(context.Background(), `SELECT c.updated_at, r.updated_at FROM resource r
		JOIN credential c ON c.resource_id = r.id AND c.slot = 'agent_readonly' WHERE r.id = $1`, h.res.ID).Scan(&cred, &res))
	return max(cred.UnixMicro(), res.UnixMicro())
}

func (h healthEnv) check(t *testing.T, status, detail string, version int64) {
	t.Helper()
	require.NoError(t, h.svc.ReportHealth(h.ctx, h.org, h.inst[0], []inventory.Health{{ResourceID: h.res.ID, OK: true,
		CheckedAt: time.Now(), AgentReadonlyStatus: status, AgentReadonlyDetail: detail, AgentReadonlyVersion: version}}))
}

func (h healthEnv) readonly(t *testing.T) (status, detail string) {
	t.Helper()
	r, err := h.svc.Get(h.ctx, h.org, h.res.ID)
	require.NoError(t, err)
	return r.AgentReadonlyStatus, r.AgentReadonlyDetail
}

// checkAudits are the data of the resource.agent_readonly.check events, oldest first.
func (h healthEnv) checkAudits(t *testing.T) []map[string]any {
	t.Helper()
	rows, err := h.pool.Query(context.Background(), `SELECT actor_kind, target_id, data FROM audit_event
		WHERE org_id = $1 AND action = 'resource.agent_readonly.check' ORDER BY id`, h.org)
	require.NoError(t, err)
	defer rows.Close()
	var out []map[string]any
	for rows.Next() {
		var kind, target string
		var raw []byte
		require.NoError(t, rows.Scan(&kind, &target, &raw))
		assert.Equal(t, "gateway", kind)
		assert.Equal(t, h.res.ID.String(), target)
		var data map[string]any
		require.NoError(t, json.Unmarshal(raw, &data))
		out = append(out, data)
	}
	require.NoError(t, rows.Err())
	return out
}

// A check result of the current version sets the status and audits the change
// with the gateway as the actor; the same result again changes nothing; an
// empty status carries no check.
func TestReportHealthAgentReadonly(t *testing.T) {
	h := readonlySetup(t)
	v := h.version(t)

	h.check(t, "", "", v)
	st, _ := h.readonly(t)
	assert.Equal(t, "pending", st, "a probe without a check")
	assert.Empty(t, h.checkAudits(t))

	h.check(t, "ok", "", v)
	st, detail := h.readonly(t)
	assert.Equal(t, "ok", st)
	assert.Empty(t, detail)
	assert.Equal(t, []map[string]any{{"status": "ok", "detail": ""}}, h.checkAudits(t))
	assert.Equal(t, v, h.version(t), "a status write keeps the check version")

	h.check(t, "ok", "", v)
	assert.Len(t, h.checkAudits(t), 1, "an unchanged result is not audited")

	h.check(t, "has_write", "INSERT on db.public.t", v)
	st, detail = h.readonly(t)
	assert.Equal(t, "has_write", st)
	assert.Equal(t, "INSERT on db.public.t", detail)
	assert.Equal(t, map[string]any{"status": "has_write", "detail": "INSERT on db.public.t"}, h.checkAudits(t)[1])
}

// A result is ignored unless it is of the current version: after the
// credential is set again or the resource changes, an older check never
// counts; an instance of another pool, a resource without the credential and
// a forged version change nothing.
func TestReportHealthIgnoresStaleCredentialVersion(t *testing.T) {
	h := readonlySetup(t)
	old := h.version(t)
	ctx := context.Background()

	_, err := h.pool.Exec(ctx, "UPDATE credential SET updated_at = now() + interval '1 second' WHERE resource_id = $1", h.res.ID)
	require.NoError(t, err)
	h.check(t, "ok", "", old)
	st, _ := h.readonly(t)
	assert.Equal(t, "pending", st, "a check of the replaced credential")

	_, err = h.svc.Update(h.ctx, h.org, inventory.UpdateInput{ID: h.res.ID, Name: "web", GatewayID: &h.gwPool.ID, Config: sshCfg("h2")})
	require.NoError(t, err)
	cur := h.version(t)
	require.Greater(t, cur, old)
	h.check(t, "ok", "", cur-1)
	st, _ = h.readonly(t)
	assert.Equal(t, "pending", st, "a check of the old configuration")

	other := h.newPool(t, "other")
	foreign := h.enroll(t, h.token(t, other.ID, 0), "gw-x")
	require.NoError(t, h.svc.ReportHealth(h.ctx, h.org, foreign.InstanceID, []inventory.Health{{ResourceID: h.res.ID, OK: true,
		CheckedAt: time.Now(), AgentReadonlyStatus: "ok", AgentReadonlyVersion: cur}}))
	st, _ = h.readonly(t)
	assert.Equal(t, "pending", st, "an instance of another pool")

	_, err = h.pool.Exec(ctx, "DELETE FROM credential WHERE resource_id = $1", h.res.ID)
	require.NoError(t, err)
	h.check(t, "ok", "", cur)
	st, _ = h.readonly(t)
	assert.Equal(t, "pending", st, "no agent_readonly credential")
	assert.Empty(t, h.checkAudits(t))
}
