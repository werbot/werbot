package access_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/access"
	"github.com/werbot/werbot/internal/platform/apperr"
	"github.com/werbot/werbot/internal/platform/db/dbtest"
)

func ids(vs []access.RequestView) []uuid.UUID {
	out := make([]uuid.UUID, len(vs))
	for i, v := range vs {
		out[i] = v.ID
	}
	return out
}

func TestListRequestsScopes(t *testing.T) {
	svc, pool, f, _, _ := newTestService(t)
	team := dbtest.Team(t, pool, f.OrgID, f.OtherMemberID)
	f.ApprovalPolicy(t, pool, "prod", access.ModeManual, onResource(f.ResourceID), allUsers, `{"teams": ["`+team.String()+`"]}`)
	mine := pending(t, svc, f, f.MemberID)
	theirs := pending(t, svc, f, f.AdminID)

	got, next, err := svc.ListRequests(bg, f.Principal(f.MemberID), access.RequestFilter{Scope: "mine"})
	require.NoError(t, err)
	assert.Zero(t, next)
	require.Equal(t, []uuid.UUID{mine.ID}, ids(got))
	assert.Equal(t, "web", got[0].ResourceName)
	assert.NotEmpty(t, got[0].RequesterLogin)
	assert.False(t, got[0].CanDecide)

	got, _, err = svc.ListRequests(bg, f.Principal(f.OtherMemberID), access.RequestFilter{Scope: "to_approve"})
	require.NoError(t, err)
	assert.Equal(t, []uuid.UUID{theirs.ID, mine.ID}, ids(got), "newest first")
	assert.True(t, got[0].CanDecide)

	got, _, err = svc.ListRequests(bg, f.Principal(f.MemberID), access.RequestFilter{Scope: "to_approve"})
	require.NoError(t, err)
	assert.Empty(t, got)

	got, next, err = svc.ListRequests(bg, f.Principal(f.OtherMemberID), access.RequestFilter{Scope: "to_approve", PageSize: 1})
	require.NoError(t, err)
	assert.Equal(t, []uuid.UUID{theirs.ID}, ids(got))
	assert.Equal(t, 2, next)
	got, next, err = svc.ListRequests(bg, f.Principal(f.OtherMemberID), access.RequestFilter{Scope: "to_approve", Page: 2, PageSize: 1})
	require.NoError(t, err)
	assert.Equal(t, []uuid.UUID{mine.ID}, ids(got))
	assert.Zero(t, next)

	_, _, err = svc.ListRequests(bg, f.Principal(f.MemberID), access.RequestFilter{Scope: "all"})
	assert.ErrorIs(t, err, apperr.ErrPermissionDenied)
	got, _, err = svc.ListRequests(bg, f.Principal(f.OwnerID), access.RequestFilter{Scope: "all"})
	require.NoError(t, err)
	assert.Len(t, got, 2)
	assert.False(t, got[0].CanDecide, "owner is not a named approver")

	_, err = svc.Decide(bg, f.Principal(f.OtherMemberID), mine.ID, false, "")
	require.NoError(t, err)
	got, _, err = svc.ListRequests(bg, f.Principal(f.OwnerID), access.RequestFilter{Scope: "all", Status: access.StatusDenied})
	require.NoError(t, err)
	assert.Equal(t, []uuid.UUID{mine.ID}, ids(got))

	for _, bad := range []access.RequestFilter{{Scope: "x"}, {Scope: "mine", Status: "x"}, {Scope: "mine", Page: -1}} {
		_, _, err = svc.ListRequests(bg, f.Principal(f.OwnerID), bad)
		assert.ErrorIs(t, err, apperr.ErrInvalidArgument)
	}
}

func TestGetRequestVisibility(t *testing.T) {
	svc, pool, f, _, _ := newTestService(t)
	team := dbtest.Team(t, pool, f.OrgID, f.OtherMemberID)
	f.ApprovalPolicy(t, pool, "prod", access.ModeManual, onResource(f.ResourceID), allUsers, `{"teams": ["`+team.String()+`"]}`)
	a := pending(t, svc, f, f.MemberID)
	for who, can := range map[uuid.UUID]bool{f.MemberID: false, f.OtherMemberID: true, f.OwnerID: false, f.AdminID: false} {
		v, err := svc.GetRequest(bg, f.Principal(who), a.ID)
		require.NoError(t, err)
		assert.Equal(t, a.ID, v.ID)
		assert.Equal(t, can, v.CanDecide)
	}
	stranger := dbtest.User(t, pool, f.OrgID, "member")
	_, err := svc.GetRequest(bg, f.Principal(stranger), a.ID)
	assert.ErrorIs(t, err, apperr.ErrNotFound)
	_, err = svc.GetRequest(bg, f.Principal(f.OwnerID), uuid.New())
	assert.ErrorIs(t, err, apperr.ErrNotFound)
}

func TestRequestable(t *testing.T) {
	svc, pool, f, _, _ := newTestService(t)
	team := dbtest.Team(t, pool, f.OrgID, f.MemberID)
	r2 := dbtest.Resource(t, pool, f.OrgID, "db", nil)
	f.ApprovalPolicy(t, pool, "team", access.ModeManual, onResource(f.ResourceID), `{"teams": ["`+team.String()+`"]}`, noOne)
	bgPol := f.ApprovalPolicy(t, pool, "bg", access.ModeManual, onResource(r2), allUsers, noOne)
	setPolicy(t, pool, bgPol, "break_glass = true")

	got, err := svc.Requestable(bg, f.Principal(f.MemberID), []uuid.UUID{f.ResourceID, r2})
	require.NoError(t, err)
	assert.Equal(t, map[uuid.UUID]bool{f.ResourceID: true, r2: true}, got)
	got, err = svc.Requestable(bg, f.Principal(f.OtherMemberID), []uuid.UUID{f.ResourceID})
	require.NoError(t, err)
	assert.Equal(t, map[uuid.UUID]bool{f.ResourceID: false}, got)
}

func TestCanDecideOnlyPendingBeforeDeadline(t *testing.T) {
	svc, pool, f, _, _ := newTestService(t)
	f.ApprovalPolicy(t, pool, "prod", access.ModeManual, onResource(f.ResourceID), allUsers, noOne)
	decided := pending(t, svc, f, f.MemberID)
	overdue := pending(t, svc, f, f.MemberID)
	open := pending(t, svc, f, f.MemberID)
	_, err := svc.Decide(bg, f.Principal(f.OwnerID), decided.ID, true, "")
	require.NoError(t, err)
	_, err = pool.Exec(bg, "UPDATE approval SET expires_at = now() - interval '1 second' WHERE id = $1", overdue.ID)
	require.NoError(t, err)
	for id, can := range map[uuid.UUID]bool{decided.ID: false, overdue.ID: false, open.ID: true} {
		v, err := svc.GetRequest(bg, f.Principal(f.AdminID), id)
		require.NoError(t, err)
		assert.Equal(t, can, v.CanDecide, id)
	}
	got, _, err := svc.ListRequests(bg, f.Principal(f.AdminID), access.RequestFilter{Scope: "to_approve"})
	require.NoError(t, err)
	assert.Equal(t, []uuid.UUID{open.ID}, ids(got))
}
