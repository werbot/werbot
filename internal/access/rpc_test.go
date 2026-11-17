package access_test

import (
	"context"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	connectorv1 "github.com/werbot/werbot/gen/werbot/connector/v1"
	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
	"github.com/werbot/werbot/internal/access"
	"github.com/werbot/werbot/internal/platform/db/dbtest"
	"github.com/werbot/werbot/internal/platform/principal"
)

func TestRPCMemberDenied(t *testing.T) {
	e := setup(t)
	h := access.NewHandler(e.svc)
	member := dbtest.User(t, e.pool, e.org, "member")
	ctx := principal.With(context.Background(), principal.Principal{UserID: member, OrgID: e.org, Role: "member"})

	_, err := h.ListBindings(ctx, connect.NewRequest(&werbotv1.ListBindingsRequest{}))
	assert.Equal(t, connect.CodePermissionDenied, connect.CodeOf(err))
	_, err = h.CreateBinding(ctx, connect.NewRequest(&werbotv1.CreateBindingRequest{}))
	assert.Equal(t, connect.CodePermissionDenied, connect.CodeOf(err))
	_, err = h.DeleteBinding(ctx, connect.NewRequest(&werbotv1.DeleteBindingRequest{}))
	assert.Equal(t, connect.CodePermissionDenied, connect.CodeOf(err))
}

func TestRPCRoundTrip(t *testing.T) {
	e := setup(t)
	h := access.NewHandler(e.svc)
	r := dbtest.Resource(t, e.pool, e.org, "web", nil)

	_, err := h.CreateBinding(e.ctx, connect.NewRequest(&werbotv1.CreateBindingRequest{
		SubjectType: "user", SubjectId: "", ResourceId: r.String()}))
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err), "bad subject id")
	created, err := h.CreateBinding(e.ctx, connect.NewRequest(&werbotv1.CreateBindingRequest{
		SubjectType: "user", SubjectId: e.owner.String(), ResourceId: r.String()}))
	require.NoError(t, err)
	assert.Equal(t, r.String(), created.Msg.GetBinding().GetResourceId())

	_, err = h.CreateBinding(e.ctx, connect.NewRequest(&werbotv1.CreateBindingRequest{SubjectType: "user",
		SubjectId: e.owner.String(), ResourceId: r.String(), Ports: []*connectorv1.PortRange{{Port: 70000}}}))
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err), "port above 65535")
	_, err = h.CreateBinding(e.ctx, connect.NewRequest(&werbotv1.CreateBindingRequest{SubjectType: "user",
		SubjectId: e.owner.String(), ResourceId: r.String(), Ports: []*connectorv1.PortRange{{Port: 0}}}))
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err), "port 0")
	ported, err := h.CreateBinding(e.ctx, connect.NewRequest(&werbotv1.CreateBindingRequest{SubjectType: "user",
		SubjectId: e.owner.String(), ResourceId: r.String(), Ports: []*connectorv1.PortRange{{Port: 5432}, {Port: 8000, EndPort: 8100}}}))
	require.NoError(t, err)
	assert.True(t, proto.Equal(&werbotv1.Binding{Ports: []*connectorv1.PortRange{{Port: 5432}, {Port: 8000, EndPort: 8100}}},
		&werbotv1.Binding{Ports: ported.Msg.GetBinding().GetPorts()}))
	_, err = h.DeleteBinding(e.ctx, connect.NewRequest(&werbotv1.DeleteBindingRequest{Id: ported.Msg.GetBinding().GetId()}))
	require.NoError(t, err)

	id := r.String()
	list, err := h.ListBindings(e.ctx, connect.NewRequest(&werbotv1.ListBindingsRequest{ResourceId: &id}))
	require.NoError(t, err)
	require.Len(t, list.Msg.GetBindings(), 1)

	_, err = h.DeleteBinding(e.ctx, connect.NewRequest(&werbotv1.DeleteBindingRequest{Id: created.Msg.GetBinding().GetId()}))
	require.NoError(t, err)
}
