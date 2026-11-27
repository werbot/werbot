package inventory_test

import (
	"context"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
	"github.com/werbot/werbot/internal/connector"
	"github.com/werbot/werbot/internal/inventory"
)

// createPG creates a postgres resource over the handler and returns its id.
func createPG(t *testing.T, h *inventory.Handler, e env, name string) string {
	t.Helper()
	res, err := h.CreateResource(e.ctx, connect.NewRequest(&werbotv1.CreateResourceRequest{Name: name, Type: "postgres",
		ConfigJson: `{"host":"db","username":"app"}`}))
	require.NoError(t, err)
	return res.Msg.GetResource().GetId()
}

// TestSetCredentialSlotRPC: slot "" is primary; an unknown slot, and
// agent_readonly at a type without a query action, are InvalidArgument; the
// kind must be one of the type's; agent_readonly at postgres goes to its own
// slot, and GetResource and ListResources show its kind and the status.
func TestSetCredentialSlotRPC(t *testing.T) {
	e := setup(t)
	creds := newFakeCreds()
	h := inventory.NewHandler(e.svc, creds, nil, nil, nil)
	pg := createPG(t, h, e, "pg")
	sshRes, err := h.CreateResource(e.ctx, createReq("web", sshCfg("h")))
	require.NoError(t, err)
	ssh := sshRes.Msg.GetResource().GetId()

	set := func(id, slot, kind string) error {
		_, err := h.SetCredential(e.ctx, connect.NewRequest(&werbotv1.SetCredentialRequest{ResourceId: id, Slot: slot, Kind: kind, Value: []byte("x")}))
		return err
	}
	clearSlot := func(id, slot string) error {
		_, err := h.ClearCredential(e.ctx, connect.NewRequest(&werbotv1.ClearCredentialRequest{ResourceId: id, Slot: slot}))
		return err
	}
	invalid := func(err error, msg string) {
		t.Helper()
		assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err), msg)
		assert.ErrorContains(t, err, msg)
	}
	invalid(set(pg, "x", "password"), "unknown credential slot")
	invalid(clearSlot(pg, "x"), "unknown credential slot")
	invalid(set(ssh, "agent_readonly", "password"), "resource type has no agent credential slot")
	invalid(clearSlot(ssh, "agent_readonly"), "resource type has no agent credential slot")
	invalid(set(pg, "agent_readonly", "private_key"), `credential kind "private_key" is not accepted by type "postgres"`)
	assert.Empty(t, creds.m, "a refused request stores nothing")

	require.NoError(t, set(pg, "", "password"))
	require.NoError(t, set(pg, "agent_readonly", "client_cert"))
	pgID := uuid.MustParse(pg)
	assert.Equal(t, map[credKey]connector.CredentialKind{
		{pgID, "primary"}: connector.CredentialPassword, {pgID, "agent_readonly"}: connector.CredentialClientCert}, creds.m)

	// credential.Service.Set marks the status in its transaction (internal/credential); here the column is set directly.
	_, err = e.pool.Exec(context.Background(), "UPDATE resource SET agent_readonly_status = 'has_write', agent_readonly_detail = 'INSERT on db.public.t' WHERE id = $1", pgID)
	require.NoError(t, err)
	get, err := h.GetResource(e.ctx, connect.NewRequest(&werbotv1.GetResourceRequest{Id: pg}))
	require.NoError(t, err)
	r := get.Msg.GetResource()
	assert.Equal(t, "password", r.GetCredentialKind())
	assert.Equal(t, "client_cert", r.GetAgentCredentialKind())
	assert.Equal(t, "has_write", r.GetAgentReadonlyStatus())
	assert.Equal(t, "INSERT on db.public.t", r.GetAgentReadonlyDetail())
	list, err := h.ListResources(e.ctx, connect.NewRequest(&werbotv1.ListResourcesRequest{}))
	require.NoError(t, err)
	for _, r := range list.Msg.GetResources() {
		want := map[string]string{pg: "has_write", ssh: "none"}[r.GetId()]
		assert.Equal(t, want, r.GetAgentReadonlyStatus(), r.GetName())
	}

	require.NoError(t, clearSlot(pg, "agent_readonly"))
	assert.Equal(t, map[credKey]connector.CredentialKind{{pgID, "primary"}: connector.CredentialPassword}, creds.m,
		"clearing the agent slot keeps the primary one")
	require.NoError(t, clearSlot(pg, ""))
	assert.Empty(t, creds.m)
}

// TestUpdateResourceResetsAgentReadonly: an update of a resource whose
// agent_readonly status is not none sets pending with an empty detail and
// moves updated_at (part of the check version), so a check of the old
// settings never counts; a resource at none stays none.
func TestUpdateResourceResetsAgentReadonly(t *testing.T) {
	e := setup(t)
	h := inventory.NewHandler(e.svc, newFakeCreds(), nil, nil, nil)
	pg, plain := createPG(t, h, e, "pg"), createPG(t, h, e, "plain")
	update := func(id, name string) *werbotv1.Resource {
		t.Helper()
		res, err := h.UpdateResource(e.ctx, connect.NewRequest(&werbotv1.UpdateResourceRequest{Id: id, Name: name,
			ConfigJson: `{"host":"db2","username":"app"}`}))
		require.NoError(t, err)
		return res.Msg.GetResource()
	}
	for _, status := range []string{"ok", "has_write", "error", "pending"} {
		_, err := e.pool.Exec(context.Background(),
			"UPDATE resource SET agent_readonly_status = $2, agent_readonly_detail = 'finding', updated_at = now() - interval '1 minute' WHERE id = $1",
			pg, status)
		require.NoError(t, err)
		var before time.Time
		require.NoError(t, e.pool.QueryRow(context.Background(), "SELECT updated_at FROM resource WHERE id = $1", pg).Scan(&before))

		r := update(pg, "pg")
		assert.Equal(t, "pending", r.GetAgentReadonlyStatus(), status)
		assert.Empty(t, r.GetAgentReadonlyDetail(), status)
		assert.True(t, r.GetUpdatedAt().AsTime().After(before), "%s: the check version moves", status)
	}
	assert.Equal(t, "none", update(plain, "plain").GetAgentReadonlyStatus(), "no agent credential: none stays")
}
