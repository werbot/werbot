package access_test

import (
	"context"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/durationpb"

	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
	"github.com/werbot/werbot/internal/access"
	"github.com/werbot/werbot/internal/platform/db/dbtest"
	"github.com/werbot/werbot/internal/platform/principal"
)

func TestRPCApprovalPolicyMemberDenied(t *testing.T) {
	e := setup(t)
	h := access.NewApprovalPolicyHandler(e.svc)
	member := dbtest.User(t, e.pool, e.org, "member")
	ctx := principal.With(context.Background(), principal.Principal{UserID: member, OrgID: e.org, Role: "member"})

	_, err := h.ListApprovalPolicies(ctx, connect.NewRequest(&werbotv1.ListApprovalPoliciesRequest{}))
	assert.Equal(t, connect.CodePermissionDenied, connect.CodeOf(err))
	_, err = h.CreateApprovalPolicy(ctx, connect.NewRequest(&werbotv1.CreateApprovalPolicyRequest{}))
	assert.Equal(t, connect.CodePermissionDenied, connect.CodeOf(err))
	_, err = h.UpdateApprovalPolicy(ctx, connect.NewRequest(&werbotv1.UpdateApprovalPolicyRequest{}))
	assert.Equal(t, connect.CodePermissionDenied, connect.CodeOf(err))
	_, err = h.DeleteApprovalPolicy(ctx, connect.NewRequest(&werbotv1.DeleteApprovalPolicyRequest{}))
	assert.Equal(t, connect.CodePermissionDenied, connect.CodeOf(err))
}

func TestRPCApprovalPolicyRoundTrip(t *testing.T) {
	e := setup(t)
	h := access.NewApprovalPolicyHandler(e.svc)
	create := &werbotv1.CreateApprovalPolicyRequest{Name: "prod", Mode: "manual", RequireReason: true,
		ResourceSelector: &werbotv1.ResourceSelector{Labels: map[string]string{"env": "prod"}},
		Requesters:       &werbotv1.Subjects{AllUsers: true}, Approvers: &werbotv1.Subjects{Users: []string{e.owner.String()}},
		MaxDuration: durationpb.New(2 * time.Hour)}

	_, err := h.CreateApprovalPolicy(e.ctx, connect.NewRequest(&werbotv1.CreateApprovalPolicyRequest{Name: "x", Mode: "manual",
		Requesters: &werbotv1.Subjects{Users: []string{"not-a-uuid"}}}))
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err), "bad user id")
	_, err = h.CreateApprovalPolicy(e.ctx, connect.NewRequest(&werbotv1.CreateApprovalPolicyRequest{Name: "x", Mode: "manual",
		ResourceSelector: &werbotv1.ResourceSelector{}, Requesters: &werbotv1.Subjects{AllUsers: true}}))
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err), "empty selector")

	created, err := h.CreateApprovalPolicy(e.ctx, connect.NewRequest(create))
	require.NoError(t, err)
	got := created.Msg.GetApprovalPolicy()
	assert.Equal(t, "prod", got.GetName())
	assert.Equal(t, 2*time.Hour, got.GetMaxDuration().AsDuration())
	assert.Equal(t, map[string]string{"env": "prod"}, got.GetResourceSelector().GetLabels())
	assert.Equal(t, []string{e.owner.String()}, got.GetApprovers().GetUsers())
	assert.Equal(t, e.owner.String(), got.GetCreatedBy())
	assert.NotNil(t, got.GetCreatedAt())

	_, err = h.CreateApprovalPolicy(e.ctx, connect.NewRequest(create))
	assert.Equal(t, connect.CodeAlreadyExists, connect.CodeOf(err))

	upd, err := h.UpdateApprovalPolicy(e.ctx, connect.NewRequest(&werbotv1.UpdateApprovalPolicyRequest{Id: got.GetId(), Name: "prod",
		Mode: "auto", ResourceSelector: create.ResourceSelector, Requesters: create.Requesters}))
	require.NoError(t, err)
	assert.Equal(t, "auto", upd.Msg.GetApprovalPolicy().GetMode())
	assert.Equal(t, 8*time.Hour, upd.Msg.GetApprovalPolicy().GetMaxDuration().AsDuration(), "full replace: default duration")

	list, err := h.ListApprovalPolicies(e.ctx, connect.NewRequest(&werbotv1.ListApprovalPoliciesRequest{}))
	require.NoError(t, err)
	require.Len(t, list.Msg.GetApprovalPolicies(), 1)

	_, err = h.DeleteApprovalPolicy(e.ctx, connect.NewRequest(&werbotv1.DeleteApprovalPolicyRequest{Id: got.GetId()}))
	require.NoError(t, err)
	_, err = h.DeleteApprovalPolicy(e.ctx, connect.NewRequest(&werbotv1.DeleteApprovalPolicyRequest{Id: got.GetId()}))
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
}
