package inventory_test

import (
	"context"
	"strings"
	"testing"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	connectorv1 "github.com/werbot/werbot/gen/werbot/connector/v1"
	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
	"github.com/werbot/werbot/internal/connector"
	_ "github.com/werbot/werbot/internal/connector/tcp" // base types of presets
	_ "github.com/werbot/werbot/internal/connector/udp"
	"github.com/werbot/werbot/internal/inventory"
	"github.com/werbot/werbot/internal/platform/apperr"
	"github.com/werbot/werbot/internal/platform/db/dbtest"
	"github.com/werbot/werbot/internal/platform/principal"
)

type presetEnv struct {
	env
	ph *inventory.PresetHandler
	rh *inventory.Handler
}

func presetSetup(t *testing.T) presetEnv {
	t.Helper()
	e := setup(t)
	return presetEnv{e, inventory.NewPresetHandler(e.svc),
		inventory.NewHandler(e.svc, fakeCreds{map[uuid.UUID]connector.CredentialKind{}}, nil, nil, nil)}
}

func (e presetEnv) as(role string) context.Context {
	return principal.With(context.Background(), principal.Principal{UserID: e.owner, OrgID: e.org, Role: role})
}

func pgCreate(name string) *werbotv1.CreateResourcePresetRequest {
	return &werbotv1.CreateResourcePresetRequest{Name: name, BaseType: "tcp", DisplayName: "PostgreSQL", Icon: "database",
		DefaultPorts: []*connectorv1.PortRange{{Port: 5432}}, Category: "databases", LaunchHint: "psql -h localhost"}
}

func (e presetEnv) create(t *testing.T, req *werbotv1.CreateResourcePresetRequest) *werbotv1.ResourcePreset {
	t.Helper()
	res, err := e.ph.CreateResourcePreset(e.ctx, connect.NewRequest(req))
	require.NoError(t, err)
	return res.Msg.GetPreset()
}

func (e presetEnv) auditCount(t *testing.T, action string) (n int) {
	t.Helper()
	require.NoError(t, e.pool.QueryRow(context.Background(),
		"SELECT count(*) FROM audit_event WHERE action = $1 AND target_type = 'resource_preset' AND actor_user_id = $2", action, e.owner).Scan(&n))
	return n
}

func TestPresetCRUD(t *testing.T) {
	e := presetSetup(t)
	p := e.create(t, pgCreate("postgres"))
	assert.Equal(t, "postgres", p.GetName())
	assert.Equal(t, "tcp", p.GetBaseType())
	assert.EqualValues(t, 5432, p.GetDefaultPorts()[0].GetPort())

	upd := pgCreate("postgres")
	upd.DisplayName = "Postgres"
	ures, err := e.ph.UpdateResourcePreset(e.ctx, connect.NewRequest(&werbotv1.UpdateResourcePresetRequest{Id: p.GetId(), Name: upd.Name,
		BaseType: upd.BaseType, DisplayName: upd.DisplayName, Icon: upd.Icon, DefaultPorts: upd.DefaultPorts, Category: upd.Category}))
	require.NoError(t, err)
	assert.Equal(t, "Postgres", ures.Msg.GetPreset().GetDisplayName())
	assert.Empty(t, ures.Msg.GetPreset().GetLaunchHint())

	list, err := e.ph.ListResourcePresets(e.ctx, connect.NewRequest(&werbotv1.ListResourcePresetsRequest{}))
	require.NoError(t, err)
	require.Len(t, list.Msg.GetPresets(), 1)

	otherOrg := principal.With(context.Background(), principal.Principal{UserID: e.owner, OrgID: dbtest.Org(t, e.pool), Role: "owner"})
	list, err = e.ph.ListResourcePresets(otherOrg, connect.NewRequest(&werbotv1.ListResourcePresetsRequest{}))
	require.NoError(t, err)
	assert.Empty(t, list.Msg.GetPresets(), "another organization sees no presets")
	_, err = e.ph.DeleteResourcePreset(otherOrg, connect.NewRequest(&werbotv1.DeleteResourcePresetRequest{Id: p.GetId()}))
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))

	_, err = e.ph.DeleteResourcePreset(e.ctx, connect.NewRequest(&werbotv1.DeleteResourcePresetRequest{Id: p.GetId()}))
	require.NoError(t, err)
	for _, a := range []string{"resource_preset.create", "resource_preset.update", "resource_preset.delete"} {
		assert.Equal(t, 1, e.auditCount(t, a), a)
	}

	member := e.as("member")
	_, err = e.ph.CreateResourcePreset(member, connect.NewRequest(pgCreate("redis")))
	assert.Equal(t, connect.CodePermissionDenied, connect.CodeOf(err))
	_, err = e.ph.ListResourcePresets(member, connect.NewRequest(&werbotv1.ListResourcePresetsRequest{}))
	assert.Equal(t, connect.CodePermissionDenied, connect.CodeOf(err))
}

func TestPresetValidation(t *testing.T) {
	e := presetSetup(t)
	mod := func(f func(*werbotv1.CreateResourcePresetRequest)) *werbotv1.CreateResourcePresetRequest {
		r := pgCreate("postgres")
		f(r)
		return r
	}
	for name, req := range map[string]*werbotv1.CreateResourcePresetRequest{
		"builtin tcp":       pgCreate("tcp"),
		"builtin ssh":       pgCreate("ssh"),
		"upper-case name":   pgCreate("Postgres"),
		"base type ssh":     mod(func(r *werbotv1.CreateResourcePresetRequest) { r.BaseType = "ssh" }),
		"no ports":          mod(func(r *werbotv1.CreateResourcePresetRequest) { r.DefaultPorts = nil }),
		"port zero":         mod(func(r *werbotv1.CreateResourcePresetRequest) { r.DefaultPorts = []*connectorv1.PortRange{{}} }),
		"unknown category":  mod(func(r *werbotv1.CreateResourcePresetRequest) { r.Category = "bogus" }),
		"long hint":         mod(func(r *werbotv1.CreateResourcePresetRequest) { r.LaunchHint = strings.Repeat("h", 513) }),
		"empty display":     mod(func(r *werbotv1.CreateResourcePresetRequest) { r.DisplayName = "" }),
		"bad icon":          mod(func(r *werbotv1.CreateResourcePresetRequest) { r.Icon = "Data Base" }),
		"long display name": mod(func(r *werbotv1.CreateResourcePresetRequest) { r.DisplayName = strings.Repeat("d", 65) }),
	} {
		_, err := e.ph.CreateResourcePreset(e.ctx, connect.NewRequest(req))
		assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err), name)
	}

	p := e.create(t, pgCreate("postgres"))
	_, err := e.ph.CreateResourcePreset(e.ctx, connect.NewRequest(pgCreate("postgres")))
	assert.Equal(t, connect.CodeAlreadyExists, connect.CodeOf(err))
	for name, f := range map[string]func(*werbotv1.UpdateResourcePresetRequest){
		"other base type": func(r *werbotv1.UpdateResourcePresetRequest) { r.BaseType = "udp" },
		"other name":      func(r *werbotv1.UpdateResourcePresetRequest) { r.Name = "pg" },
	} {
		req := &werbotv1.UpdateResourcePresetRequest{Id: p.GetId(), Name: "postgres", BaseType: "tcp", DisplayName: "PostgreSQL",
			DefaultPorts: []*connectorv1.PortRange{{Port: 5432}}, Category: "databases"}
		f(req)
		_, err := e.ph.UpdateResourcePreset(e.ctx, connect.NewRequest(req))
		assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err), name)
	}
}

func TestListResourceTypesIncludesPresets(t *testing.T) {
	e := presetSetup(t)
	p := e.create(t, pgCreate("postgres"))
	otherOrg := principal.With(context.Background(), principal.Principal{UserID: e.owner, OrgID: dbtest.Org(t, e.pool), Role: "member"})
	_, err := e.svc.CreatePreset(principal.With(context.Background(), principal.Principal{UserID: e.owner, OrgID: e.org, Role: "owner"}),
		e.org, inventory.PresetInput{Name: "redis", BaseType: "tcp", DisplayName: "Redis", Category: "databases",
			DefaultPorts: []connector.PortRange{{Port: 6379}}})
	require.NoError(t, err)

	res, err := e.rh.ListResourceTypes(e.as("member"), connect.NewRequest(&werbotv1.ListResourceTypesRequest{}))
	require.NoError(t, err)
	var names []string
	for _, ty := range res.Msg.GetTypes() {
		names = append(names, ty.GetName())
		if ty.GetName() == "tcp" {
			assert.Empty(t, ty.GetPresetId(), "a builtin entry carries no preset fields")
			assert.Empty(t, ty.GetBaseType())
		}
		if ty.GetName() != "postgres" {
			continue
		}
		assert.Equal(t, p.GetId(), ty.GetPresetId())
		assert.Equal(t, "tcp", ty.GetBaseType())
		assert.Equal(t, []string{"stream"}, ty.GetKinds())
		assert.Equal(t, "databases", ty.GetCategory())
		assert.Equal(t, "PostgreSQL", ty.GetDisplayName())
		assert.Equal(t, "database", ty.GetIcon())
		assert.Equal(t, "psql -h localhost", ty.GetLaunchHint())
		assert.EqualValues(t, 5432, ty.GetDefaultPorts()[0].GetPort())
		assert.Equal(t, []string{"tcp.connect"}, ty.GetActions())
	}
	assert.Contains(t, names, "tcp")
	assert.Less(t, indexOf(names, "postgres"), indexOf(names, "redis"), "presets by name")
	assert.Less(t, indexOf(names, "udp"), indexOf(names, "postgres"), "builtin types first")

	res, err = e.rh.ListResourceTypes(otherOrg, connect.NewRequest(&werbotv1.ListResourceTypesRequest{}))
	require.NoError(t, err)
	for _, ty := range res.Msg.GetTypes() {
		assert.Empty(t, ty.GetPresetId(), "a preset of another organization is invisible")
	}
}

func indexOf(s []string, v string) int {
	for i, x := range s {
		if x == v {
			return i
		}
	}
	return -1
}

func presetResource(name, typ, presetID string) *connect.Request[werbotv1.CreateResourceRequest] {
	return connect.NewRequest(&werbotv1.CreateResourceRequest{Name: name, Type: typ, PresetId: presetID,
		ConfigJson: `{"host":"10.0.0.5","ports":[{"port":5432}]}`})
}

func TestCreateResourceFromPreset(t *testing.T) {
	e := presetSetup(t)
	p := e.create(t, pgCreate("postgres"))

	res, err := e.rh.CreateResource(e.ctx, presetResource("db", "tcp", p.GetId()))
	require.NoError(t, err)
	assert.Equal(t, "tcp", res.Msg.GetResource().GetType())
	assert.Equal(t, p.GetId(), res.Msg.GetResource().GetPresetId())
	got, err := e.rh.GetResource(e.ctx, connect.NewRequest(&werbotv1.GetResourceRequest{Id: res.Msg.GetResource().GetId()}))
	require.NoError(t, err)
	assert.Equal(t, p.GetId(), got.Msg.GetResource().GetPresetId())
	upd, err := e.rh.UpdateResource(e.ctx, connect.NewRequest(&werbotv1.UpdateResourceRequest{Id: res.Msg.GetResource().GetId(), Name: "db"}))
	require.NoError(t, err)
	assert.Equal(t, p.GetId(), upd.Msg.GetResource().GetPresetId(), "update keeps the preset")

	_, err = e.rh.CreateResource(e.ctx, presetResource("db-udp", "udp", p.GetId()))
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err), "type other than base_type")

	otherOrg := dbtest.Org(t, e.pool)
	foreign, err := e.svc.CreatePreset(principal.With(context.Background(), principal.Principal{UserID: e.owner, OrgID: otherOrg, Role: "owner"}),
		otherOrg, inventory.PresetInput{Name: "postgres", BaseType: "tcp", DisplayName: "PG", Category: "databases",
			DefaultPorts: []connector.PortRange{{Port: 5432}}})
	require.NoError(t, err)
	_, err = e.rh.CreateResource(e.ctx, presetResource("db-foreign", "tcp", foreign.ID.String()))
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err), "preset of another organization")
	_, err = e.rh.CreateResource(e.ctx, presetResource("db-none", "tcp", uuid.NewString()))
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err), "unknown preset")
}

func TestDeletePresetInUse(t *testing.T) {
	e := presetSetup(t)
	p := e.create(t, pgCreate("postgres"))
	res, err := e.rh.CreateResource(e.ctx, presetResource("db", "tcp", p.GetId()))
	require.NoError(t, err)

	del := connect.NewRequest(&werbotv1.DeleteResourcePresetRequest{Id: p.GetId()})
	_, err = e.ph.DeleteResourcePreset(e.ctx, del)
	assert.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err))
	assert.Equal(t, 0, e.auditCount(t, "resource_preset.delete"))

	_, err = e.rh.DeleteResource(e.ctx, connect.NewRequest(&werbotv1.DeleteResourceRequest{Id: res.Msg.GetResource().GetId()}))
	require.NoError(t, err)
	_, err = e.ph.DeleteResourcePreset(e.ctx, del)
	require.NoError(t, err)
}

// A preset deleted between its check and the insert of the resource
// (resource_preset_fk) is an invalid argument, not a missing resource.
func TestPresetForeignKeyIsInvalidArgument(t *testing.T) {
	err := inventory.NotFound(&pgconn.PgError{Code: "23503", ConstraintName: "resource_preset_fk"})
	assert.ErrorIs(t, err, apperr.ErrInvalidArgument)
	assert.ErrorContains(t, err, "preset not found")
}
