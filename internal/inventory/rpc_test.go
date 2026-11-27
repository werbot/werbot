package inventory_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"connectrpc.com/connect"
	"connectrpc.com/validate"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/wrapperspb"

	sshv1 "github.com/werbot/werbot/gen/werbot/connector/ssh/v1"
	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
	"github.com/werbot/werbot/gen/werbot/v1/werbotv1connect"
	"github.com/werbot/werbot/internal/connector"
	_ "github.com/werbot/werbot/internal/connector/postgres"
	"github.com/werbot/werbot/internal/inventory"
	"github.com/werbot/werbot/internal/platform/db/dbtest"
	"github.com/werbot/werbot/internal/platform/principal"
)

// fakeCreds keeps credentials in memory.
type fakeCreds struct {
	m map[uuid.UUID]connector.CredentialKind
}

func (f fakeCreds) Set(_ context.Context, _, id uuid.UUID, k connector.CredentialKind, _ []byte) error {
	f.m[id] = k
	return nil
}

func (f fakeCreds) Clear(_ context.Context, _, id uuid.UUID) error {
	delete(f.m, id)
	return nil
}

func (f fakeCreds) Kinds(_ context.Context, _ uuid.UUID, ids []uuid.UUID) (map[uuid.UUID]connector.CredentialKind, error) {
	out := map[uuid.UUID]connector.CredentialKind{}
	for _, id := range ids {
		if k, ok := f.m[id]; ok {
			out[id] = k
		}
	}
	return out, nil
}

func newHandler(t *testing.T) (*inventory.Handler, env) {
	t.Helper()
	e := setup(t)
	return inventory.NewHandler(e.svc, fakeCreds{map[uuid.UUID]connector.CredentialKind{}}, nil, nil, nil), e
}

func createReq(name string, cfg *sshv1.Config) *connect.Request[werbotv1.CreateResourceRequest] {
	a, _ := anypb.New(cfg)
	return connect.NewRequest(&werbotv1.CreateResourceRequest{Name: name, Type: "ssh", Config: a})
}

func TestResponsesNeverContainCredential(t *testing.T) {
	h, e := newHandler(t)
	res, err := h.CreateResource(e.ctx, createReq("one", sshCfg("h")))
	require.NoError(t, err)
	id := res.Msg.GetResource().GetId()
	_, err = h.SetCredential(e.ctx, connect.NewRequest(&werbotv1.SetCredentialRequest{ResourceId: id, Kind: "password", Value: []byte("s3cr3t-value")}))
	require.NoError(t, err)

	get, err := h.GetResource(e.ctx, connect.NewRequest(&werbotv1.GetResourceRequest{Id: id}))
	require.NoError(t, err)
	list, err := h.ListResources(e.ctx, connect.NewRequest(&werbotv1.ListResourcesRequest{}))
	require.NoError(t, err)
	upd, err := h.UpdateResource(e.ctx, connect.NewRequest(&werbotv1.UpdateResourceRequest{Id: id, Name: "one"}))
	require.NoError(t, err)
	created, err := h.CreateResource(e.ctx, createReq("two", sshCfg("h2")))
	require.NoError(t, err)

	for _, m := range []proto.Message{get.Msg, list.Msg, upd.Msg, created.Msg} {
		b, _ := proto.Marshal(m)
		j, _ := protojson.Marshal(m)
		assert.NotContains(t, string(b), "s3cr3t-value")
		assert.NotContains(t, string(j), "s3cr3t-value")
	}
	assert.Equal(t, "password", get.Msg.GetResource().GetCredentialKind())
	assert.Equal(t, "password", upd.Msg.GetResource().GetCredentialKind())
	assert.Empty(t, created.Msg.GetResource().GetCredentialKind())
}

func TestCreateInvalidConfigIsInvalidArgument(t *testing.T) {
	h, e := newHandler(t)
	wrong, _ := anypb.New(wrapperspb.String("x"))
	good, _ := anypb.New(sshCfg("h"))
	for name, req := range map[string]*werbotv1.CreateResourceRequest{
		"port out of range": createReq("a", &sshv1.Config{Host: "h", Port: 70000, Login: "root"}).Msg,
		"foreign any":       {Name: "a", Type: "ssh", Config: wrong},
		"bad json":          {Name: "a", Type: "ssh", ConfigJson: `{"port":"x"}`},
		"both":              {Name: "a", Type: "ssh", Config: good, ConfigJson: `{"host":"h","login":"root"}`},
		"none":              {Name: "a", Type: "ssh"},
		"unknown type":      {Name: "a", Type: "nope", Config: good},
		"bad gateway id":    {Name: "a", Type: "ssh", Config: good, GatewayId: "x"},
	} {
		_, err := h.CreateResource(e.ctx, connect.NewRequest(req))
		assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err), name)
	}
}

func TestCreateFromConfigJSON(t *testing.T) {
	h, e := newHandler(t)
	res, err := h.CreateResource(e.ctx, connect.NewRequest(&werbotv1.CreateResourceRequest{
		Name: "j", Type: "ssh", ConfigJson: `{"host":"h","login":"root"}`}))
	require.NoError(t, err)
	get, err := h.GetResource(e.ctx, connect.NewRequest(&werbotv1.GetResourceRequest{Id: res.Msg.GetResource().GetId()}))
	require.NoError(t, err)
	var cfg sshv1.Config
	require.NoError(t, get.Msg.GetResource().GetConfig().UnmarshalTo(&cfg))
	assert.Equal(t, "h", cfg.Host)
	assert.EqualValues(t, 22, cfg.Port)
	assert.Equal(t, "root", cfg.Login)
	assert.JSONEq(t, `{"host":"h","port":22,"login":"root"}`, get.Msg.GetResource().GetConfigJson())
}

func TestUpdateKeepsConfigWhenNoneGiven(t *testing.T) {
	h, e := newHandler(t)
	res, err := h.CreateResource(e.ctx, createReq("k", sshCfg("keep")))
	require.NoError(t, err)
	upd, err := h.UpdateResource(e.ctx, connect.NewRequest(&werbotv1.UpdateResourceRequest{Id: res.Msg.GetResource().GetId(), Name: "k2"}))
	require.NoError(t, err)
	assert.Equal(t, "k2", upd.Msg.GetResource().GetName())
	assert.Contains(t, upd.Msg.GetResource().GetConfigJson(), `"keep"`)
}

func TestSetCredentialKindMustMatchType(t *testing.T) {
	h, e := newHandler(t)
	res, err := h.CreateResource(e.ctx, createReq("c", sshCfg("h")))
	require.NoError(t, err)
	_, err = h.SetCredential(e.ctx, connect.NewRequest(&werbotv1.SetCredentialRequest{ResourceId: res.Msg.GetResource().GetId(), Kind: "token", Value: []byte("x")}))
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))
}

func TestMemberIsDenied(t *testing.T) {
	h, e := newHandler(t)
	ctx := principal.With(context.Background(), principal.Principal{UserID: e.owner, OrgID: e.org, Role: "member"})
	_, err := h.CreateResource(ctx, createReq("m", sshCfg("h")))
	assert.Equal(t, connect.CodePermissionDenied, connect.CodeOf(err))
	types, err := h.ListResourceTypes(ctx, connect.NewRequest(&werbotv1.ListResourceTypesRequest{}))
	require.NoError(t, err)
	names := []string{}
	for _, ty := range types.Msg.GetTypes() {
		names = append(names, ty.GetName())
	}
	assert.Contains(t, names, "ssh")
}

func TestListResourceTypesExposesKinds(t *testing.T) {
	h, e := newHandler(t)
	types, err := h.ListResourceTypes(e.ctx, connect.NewRequest(&werbotv1.ListResourceTypesRequest{}))
	require.NoError(t, err)
	for _, ty := range types.Msg.GetTypes() {
		if ty.GetName() != "ssh" {
			continue
		}
		assert.Equal(t, []string{"terminal", "exec", "stream"}, ty.GetKinds())
		assert.Equal(t, []string{"ssh.connect", "ssh.exec", "ssh.shell", "ssh.remote_forward"}, ty.GetActions())
		assert.Equal(t, []string{"password", "private_key"}, ty.GetCredentialKinds())
		assert.Equal(t, "L2", ty.GetLevel())
		assert.Equal(t, "type.googleapis.com/werbot.connector.ssh.v1.Config", ty.GetConfigTypeUrl())
		return
	}
	t.Fatal("ssh type not listed")
}

func TestOtherOrgIsNotFound(t *testing.T) {
	h, e := newHandler(t)
	orgB := dbtest.Org(t, e.pool)
	rb, err := e.svc.Create(context.Background(), orgB, inventory.CreateInput{Name: "b", Type: "ssh", Config: sshCfg("h")})
	require.NoError(t, err)
	id := rb.ID.String()

	_, err = h.GetResource(e.ctx, connect.NewRequest(&werbotv1.GetResourceRequest{Id: id}))
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
	_, err = h.DeleteResource(e.ctx, connect.NewRequest(&werbotv1.DeleteResourceRequest{Id: id}))
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
	_, err = h.SetCredential(e.ctx, connect.NewRequest(&werbotv1.SetCredentialRequest{ResourceId: id, Kind: "password", Value: []byte("x")}))
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
	_, err = h.ClearCredential(e.ctx, connect.NewRequest(&werbotv1.ClearCredentialRequest{ResourceId: id}))
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
	_, err = h.ClearCredential(e.ctx, connect.NewRequest(&werbotv1.ClearCredentialRequest{ResourceId: uuid.NewString()}))
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}

func TestResourceTimeoutsAndHealthOverRPC(t *testing.T) {
	h, e := newHandler(t)
	mux := http.NewServeMux()
	mux.Handle(werbotv1connect.NewResourceServiceHandler(h, connect.WithInterceptors(validate.NewInterceptor())))
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mux.ServeHTTP(w, r.WithContext(principal.With(r.Context(), principal.Principal{UserID: e.owner, OrgID: e.org, Role: "owner"})))
	}))
	t.Cleanup(srv.Close)
	c := werbotv1connect.NewResourceServiceClient(srv.Client(), srv.URL)

	req := createReq("one", sshCfg("h")).Msg
	req.IdleTimeout, req.MaxDuration = durationpb.New(10*time.Minute), durationpb.New(2*time.Hour)
	res, err := c.CreateResource(context.Background(), connect.NewRequest(req))
	require.NoError(t, err)
	got := res.Msg.GetResource()
	assert.Equal(t, 10*time.Minute, got.GetIdleTimeout().AsDuration())
	assert.Equal(t, 2*time.Hour, got.GetMaxDuration().AsDuration())
	assert.Equal(t, werbotv1.ResourceHealth_STATE_UNKNOWN, got.GetHealth().GetState())
	assert.Nil(t, got.GetHealth().GetCheckedAt())

	upd := &werbotv1.UpdateResourceRequest{Id: got.GetId(), Name: "one", IdleTimeout: durationpb.New(24 * time.Hour), MaxDuration: durationpb.New(168 * time.Hour)}
	ur, err := c.UpdateResource(context.Background(), connect.NewRequest(upd))
	require.NoError(t, err)
	assert.Equal(t, 24*time.Hour, ur.Msg.GetResource().GetIdleTimeout().AsDuration())
	assert.Equal(t, 168*time.Hour, ur.Msg.GetResource().GetMaxDuration().AsDuration())
	upd.MaxDuration = durationpb.New(169 * time.Hour)
	_, err = c.UpdateResource(context.Background(), connect.NewRequest(upd))
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err))

	for _, d := range []time.Duration{30 * time.Second, 25 * time.Hour} {
		bad := createReq("two", sshCfg("h")).Msg
		bad.IdleTimeout = durationpb.New(d)
		_, err = c.CreateResource(context.Background(), connect.NewRequest(bad))
		assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err), d.String())
	}
}

func TestSetCredentialPostgresKinds(t *testing.T) {
	h, e := newHandler(t)
	res, err := h.CreateResource(e.ctx, connect.NewRequest(&werbotv1.CreateResourceRequest{Name: "pg", Type: "postgres",
		ConfigJson: `{"host":"db","username":"app"}`}))
	require.NoError(t, err)
	id := res.Msg.GetResource().GetId()
	set := func(kind string) error {
		_, err := h.SetCredential(e.ctx, connect.NewRequest(&werbotv1.SetCredentialRequest{ResourceId: id, Kind: kind, Value: []byte("x")}))
		return err
	}
	require.NoError(t, set("password"))
	require.NoError(t, set("client_cert"))
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(set("private_key")))
}
