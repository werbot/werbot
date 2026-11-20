package access_test

import (
	"context"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/durationpb"

	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
	"github.com/werbot/werbot/internal/access"
	"github.com/werbot/werbot/internal/platform/db/dbtest"
	"github.com/werbot/werbot/internal/platform/principal"
)

// as returns a context carrying userID of f with its seeded role.
func as(f dbtest.Fixture, userID uuid.UUID) context.Context {
	return principal.With(bg, f.Principal(userID))
}

func TestRPCCreateApproveFlow(t *testing.T) {
	svc, pool, f, _, _ := newTestService(t)
	pol := f.ApprovalPolicy(t, pool, "prod", access.ModeManual, onResource(f.ResourceID), allUsers, noOne)
	h := access.NewAccessRequestHandler(svc)
	member, owner := as(f, f.MemberID), as(f, f.OwnerID)
	create := func() *werbotv1.AccessRequest {
		res, err := h.CreateRequest(member, connect.NewRequest(&werbotv1.CreateRequestRequest{ResourceId: f.ResourceID.String(),
			Duration: durationpb.New(2 * time.Hour), Reason: "deploy"}))
		require.NoError(t, err)
		return res.Msg.GetRequest()
	}

	req := create()
	assert.Equal(t, "pending", req.GetStatus())
	assert.Equal(t, "access", req.GetKind())
	assert.Equal(t, "https://werbot.test/requests?id="+req.GetId(), req.GetUrl())
	assert.Equal(t, f.MemberID.String(), req.GetRequesterUserId())
	assert.Equal(t, f.ResourceID.String(), req.GetResourceId())
	assert.Equal(t, pol.String(), req.GetPolicyId())
	assert.Equal(t, 2*time.Hour, req.GetDuration().AsDuration())
	assert.Equal(t, "deploy", req.GetReason())
	assert.NotNil(t, req.GetExpiresAt())
	assert.NotNil(t, req.GetCreatedAt())

	list, err := h.ListRequests(owner, connect.NewRequest(&werbotv1.ListRequestsRequest{Scope: werbotv1.RequestScope_REQUEST_SCOPE_TO_APPROVE}))
	require.NoError(t, err)
	require.Len(t, list.Msg.GetRequests(), 1)
	got := list.Msg.GetRequests()[0]
	assert.Equal(t, req.GetId(), got.GetId())
	assert.True(t, got.GetCanDecide())
	assert.Equal(t, "web", got.GetResourceName())
	assert.NotEmpty(t, got.GetRequesterLogin())
	assert.Equal(t, got.GetUrl(), req.GetUrl())
	assert.Zero(t, list.Msg.GetNextPage())
	mine, err := h.ListRequests(member, connect.NewRequest(&werbotv1.ListRequestsRequest{}))
	require.NoError(t, err)
	require.Len(t, mine.Msg.GetRequests(), 1, "unspecified scope lists own requests")
	assert.False(t, mine.Msg.GetRequests()[0].GetCanDecide())

	ok, err := h.Approve(owner, connect.NewRequest(&werbotv1.ApproveRequest{Id: req.GetId(), Comment: "go"}))
	require.NoError(t, err)
	assert.Equal(t, "approved", ok.Msg.GetRequest().GetStatus())
	assert.Equal(t, f.OwnerID.String(), ok.Msg.GetRequest().GetDecidedBy())
	assert.NotNil(t, ok.Msg.GetRequest().GetDecidedAt())
	assert.Equal(t, "go", ok.Msg.GetRequest().GetComment())
	seen, err := h.GetRequest(member, connect.NewRequest(&werbotv1.GetRequestRequest{Id: req.GetId()}))
	require.NoError(t, err)
	assert.Equal(t, "approved", seen.Msg.GetRequest().GetStatus())

	gs, err := h.ListGrants(member, connect.NewRequest(&werbotv1.ListGrantsRequest{Active: true}))
	require.NoError(t, err)
	require.Len(t, gs.Msg.GetGrants(), 1)
	g := gs.Msg.GetGrants()[0]
	assert.Equal(t, f.MemberID.String(), g.GetUserId())
	assert.Equal(t, f.ResourceID.String(), g.GetResourceId())
	assert.Equal(t, "web", g.GetResourceName())
	assert.Equal(t, req.GetId(), g.GetApprovalId())
	assert.WithinDuration(t, time.Now().Add(2*time.Hour), g.GetExpiresAt().AsTime(), 5*time.Second)
	assert.Nil(t, g.GetRevokedAt())
	_, err = h.RevokeGrant(member, connect.NewRequest(&werbotv1.RevokeGrantRequest{Id: g.GetId(), Reason: "done"}))
	require.NoError(t, err)
	gs, err = h.ListGrants(member, connect.NewRequest(&werbotv1.ListGrantsRequest{Active: true}))
	require.NoError(t, err)
	assert.Empty(t, gs.Msg.GetGrants(), "a revoked grant is not active")

	denied := create()
	res, err := h.Deny(owner, connect.NewRequest(&werbotv1.DenyRequest{Id: denied.GetId(), Comment: "no"}))
	require.NoError(t, err)
	assert.Equal(t, "denied", res.Msg.GetRequest().GetStatus())

	canceled := create()
	_, err = h.Cancel(member, connect.NewRequest(&werbotv1.CancelRequest{Id: canceled.GetId()}))
	require.NoError(t, err)
	seen, err = h.GetRequest(member, connect.NewRequest(&werbotv1.GetRequestRequest{Id: canceled.GetId()}))
	require.NoError(t, err)
	assert.Equal(t, "canceled", seen.Msg.GetRequest().GetStatus())
}

func TestRPCRequestErrors(t *testing.T) {
	svc, _, f, _, _ := newTestService(t)
	h := access.NewAccessRequestHandler(svc)
	member := as(f, f.MemberID)

	_, err := h.CreateRequest(member, connect.NewRequest(&werbotv1.CreateRequestRequest{ResourceId: f.ResourceID.String(),
		LabelSelector: map[string]string{"env": "prod"}, Reason: "x"}))
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err), "both resource and selector")
	_, err = h.CreateRequest(member, connect.NewRequest(&werbotv1.CreateRequestRequest{ResourceId: f.ResourceID.String(), Reason: "x"}))
	assert.Equal(t, connect.CodePermissionDenied, connect.CodeOf(err), "no policy")
	assert.ErrorContains(t, err, "no approval policy matches this request")
	_, err = h.CreateRequest(member, connect.NewRequest(&werbotv1.CreateRequestRequest{ResourceId: "x", Reason: "x"}))
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err), "bad resource id")
	_, err = h.GetRequest(member, connect.NewRequest(&werbotv1.GetRequestRequest{Id: "x"}))
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err), "bad id")
	_, err = h.GetRequest(member, connect.NewRequest(&werbotv1.GetRequestRequest{Id: uuid.NewString()}))
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))
	_, err = h.ListRequests(member, connect.NewRequest(&werbotv1.ListRequestsRequest{Scope: werbotv1.RequestScope(99)}))
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err), "unknown scope")
	bad := "x"
	_, err = h.ListGrants(member, connect.NewRequest(&werbotv1.ListGrantsRequest{ResourceId: &bad}))
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err), "bad grant filter")
	_, err = h.CreateRequest(context.Background(), connect.NewRequest(&werbotv1.CreateRequestRequest{}))
	assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}

func TestRPCScopeAllMemberDenied(t *testing.T) {
	svc, pool, f, _, _ := newTestService(t)
	h := access.NewAccessRequestHandler(svc)
	member, admin := as(f, f.MemberID), as(f, f.AdminID)
	grant := f.Grant(t, pool, f.MemberID, f.ResourceID, time.Hour)
	_, err := pool.Exec(bg, "UPDATE access_grant SET break_glass = true, needs_review = true WHERE id = $1", grant)
	require.NoError(t, err)

	_, err = h.ListRequests(member, connect.NewRequest(&werbotv1.ListRequestsRequest{Scope: werbotv1.RequestScope_REQUEST_SCOPE_ALL}))
	assert.Equal(t, connect.CodePermissionDenied, connect.CodeOf(err))
	_, err = h.ReviewBreakGlass(member, connect.NewRequest(&werbotv1.ReviewBreakGlassRequest{GrantId: grant.String()}))
	assert.Equal(t, connect.CodePermissionDenied, connect.CodeOf(err))
	other := f.OtherMemberID.String()
	_, err = h.ListGrants(member, connect.NewRequest(&werbotv1.ListGrantsRequest{UserId: &other}))
	assert.Equal(t, connect.CodePermissionDenied, connect.CodeOf(err), "member lists only own grants")

	_, err = h.ListRequests(admin, connect.NewRequest(&werbotv1.ListRequestsRequest{Scope: werbotv1.RequestScope_REQUEST_SCOPE_ALL}))
	require.NoError(t, err)
	_, err = h.ReviewBreakGlass(admin, connect.NewRequest(&werbotv1.ReviewBreakGlassRequest{GrantId: grant.String(), Comment: "ok"}))
	require.NoError(t, err)
	gs, err := h.ListGrants(admin, connect.NewRequest(&werbotv1.ListGrantsRequest{UserId: new(f.MemberID.String())}))
	require.NoError(t, err)
	require.Len(t, gs.Msg.GetGrants(), 1)
	assert.True(t, gs.Msg.GetGrants()[0].GetBreakGlass())
	assert.False(t, gs.Msg.GetGrants()[0].GetNeedsReview(), "reviewed")
}
