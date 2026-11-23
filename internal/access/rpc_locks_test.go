package access_test

import (
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/durationpb"

	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
	"github.com/werbot/werbot/internal/access"
)

func TestRPCLockService(t *testing.T) {
	svc, _, f, _, _ := newTestService(t)
	h := access.NewLockHandler(svc)
	admin, member := as(f, f.AdminID), as(f, f.MemberID)
	create := &werbotv1.CreateLockRequest{TargetType: "user", TargetId: f.MemberID.String(), Reason: "incident",
		ExpiresIn: durationpb.New(time.Hour)}

	_, err := h.CreateLock(member, connect.NewRequest(create))
	assert.Equal(t, connect.CodePermissionDenied, connect.CodeOf(err))
	_, err = h.ListLocks(member, connect.NewRequest(&werbotv1.ListLocksRequest{}))
	assert.Equal(t, connect.CodePermissionDenied, connect.CodeOf(err))
	_, err = h.DeleteLock(member, connect.NewRequest(&werbotv1.DeleteLockRequest{Id: f.MemberID.String()}))
	assert.Equal(t, connect.CodePermissionDenied, connect.CodeOf(err))

	_, err = h.CreateLock(admin, connect.NewRequest(&werbotv1.CreateLockRequest{TargetType: "mfa_device", TargetId: f.MemberID.String(), Reason: "x"}))
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err), "a user id is no passkey")
	_, err = h.CreateLock(admin, connect.NewRequest(&werbotv1.CreateLockRequest{TargetType: "user", TargetId: "x", Reason: "x"}))
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err), "bad target id")

	res, err := h.CreateLock(admin, connect.NewRequest(create))
	require.NoError(t, err)
	l := res.Msg.GetLock()
	assert.Equal(t, "user", l.GetTargetType())
	assert.Equal(t, f.MemberID.String(), l.GetTargetId())
	assert.Equal(t, "incident", l.GetReason())
	assert.WithinDuration(t, time.Now().Add(time.Hour), l.GetExpiresAt().AsTime(), 5*time.Second)
	assert.Equal(t, f.AdminID.String(), l.GetCreatedBy())

	list, err := h.ListLocks(admin, connect.NewRequest(&werbotv1.ListLocksRequest{}))
	require.NoError(t, err)
	require.Len(t, list.Msg.GetLocks(), 1)
	assert.Equal(t, l.GetId(), list.Msg.GetLocks()[0].GetId())
	assert.Equal(t, f.AdminID.String(), list.Msg.GetLocks()[0].GetCreatedBy())
	assert.NotNil(t, list.Msg.GetLocks()[0].GetCreatedAt())

	_, err = h.DeleteLock(admin, connect.NewRequest(&werbotv1.DeleteLockRequest{Id: l.GetId()}))
	require.NoError(t, err)
	list, err = h.ListLocks(admin, connect.NewRequest(&werbotv1.ListLocksRequest{}))
	require.NoError(t, err)
	assert.Empty(t, list.Msg.GetLocks())
	_, err = h.DeleteLock(admin, connect.NewRequest(&werbotv1.DeleteLockRequest{Id: l.GetId()}))
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err))

	perm, err := h.CreateLock(admin, connect.NewRequest(&werbotv1.CreateLockRequest{TargetType: "resource", TargetId: f.ResourceID.String(), Reason: "x"}))
	require.NoError(t, err)
	assert.Nil(t, perm.Msg.GetLock().GetExpiresAt(), "no expires_in: until deleted")
}
