package access_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/access"
	"github.com/werbot/werbot/internal/platform/apperr"
	"github.com/werbot/werbot/internal/platform/db/dbtest"
	"github.com/werbot/werbot/internal/platform/principal"
)

var bg = context.Background()

func TestCreateRequestManualPending(t *testing.T) {
	svc, pool, f, q, _ := newTestService(t)
	pol := f.ApprovalPolicy(t, pool, "prod", access.ModeManual, onResource(f.ResourceID), allUsers, noOne)
	a, err := svc.CreateRequest(bg, f.Principal(f.MemberID),
		access.CreateRequestInput{ResourceID: &f.ResourceID, Duration: 2 * time.Hour, Reason: " deploy "})
	require.NoError(t, err)
	assert.Equal(t, access.StatusPending, a.Status)
	assert.Equal(t, access.KindAccess, a.Kind)
	assert.Equal(t, &pol, a.PolicyID)
	assert.Equal(t, "deploy", a.Reason)
	assert.Equal(t, 2*time.Hour, a.Duration)
	assert.WithinDuration(t, time.Now().Add(time.Hour), a.ExpiresAt, 5*time.Second)
	assert.Nil(t, a.DecidedAt)
	assert.ElementsMatch(t, []string{emailOf(t, pool, f.OwnerID), emailOf(t, pool, f.AdminID)}, q.to("access_request"))
	m := q.of("access_request")[0]
	assert.Equal(t, "https://werbot.test/requests?id="+a.ID.String(), m.Data["url"])
	assert.Equal(t, "web", m.Data["resource"])
	assert.Equal(t, "2h", m.Data["duration"])
	assert.Equal(t, "deploy", m.Data["reason"])
	assert.Empty(t, m.Data["command"])
	assert.Equal(t, []string{"access.request.create"}, auditActions(t, pool))
	assert.Empty(t, grants(t, pool, a.ID))
}

func TestCreateRequestAuto(t *testing.T) {
	svc, pool, f, q, _ := newTestService(t)
	f.ApprovalPolicy(t, pool, "a", access.ModeAuto, onResource(f.ResourceID), allUsers, noOne)
	f.ApprovalPolicy(t, pool, "b", access.ModeAuto, onResource(f.ResourceID), allUsers, noOne)
	a, err := svc.CreateRequest(bg, f.Principal(f.MemberID),
		access.CreateRequestInput{ResourceID: &f.ResourceID, Duration: 2 * time.Hour, Reason: "x"})
	require.NoError(t, err)
	assert.Equal(t, access.StatusApproved, a.Status)
	assert.Nil(t, a.DecidedBy)
	require.NotNil(t, a.DecidedAt)
	g := grants(t, pool, a.ID)
	require.Len(t, g, 1)
	assert.WithinDuration(t, time.Now().Add(2*time.Hour), g[f.ResourceID], 5*time.Second)
	assert.Equal(t, []string{"access.request.create", "access.request.approve", "access.grant.create"}, auditActions(t, pool))
	var kind string
	var auto bool
	require.NoError(t, pool.QueryRow(bg, "SELECT actor_kind, (data->>'auto')::bool FROM audit_event WHERE action = 'access.request.approve'").Scan(&kind, &auto))
	assert.Equal(t, "system", kind)
	assert.True(t, auto)
	assert.Empty(t, q.sent)
}

func TestAutoBlockedByManual(t *testing.T) {
	svc, pool, f, _, _ := newTestService(t)
	f.ApprovalPolicy(t, pool, "a", access.ModeAuto, onResource(f.ResourceID), allUsers, noOne)
	f.ApprovalPolicy(t, pool, "b", access.ModeManual, onResource(f.ResourceID), allUsers, noOne)
	a, err := svc.CreateRequest(bg, f.Principal(f.MemberID), access.CreateRequestInput{ResourceID: &f.ResourceID, Reason: "x"})
	require.NoError(t, err)
	assert.Equal(t, access.StatusPending, a.Status)
	assert.Empty(t, grants(t, pool, a.ID))
}

func TestCreateRequestNoPolicy(t *testing.T) {
	svc, pool, f, _, _ := newTestService(t)
	team := dbtest.Team(t, pool, f.OrgID)
	f.ApprovalPolicy(t, pool, "team only", access.ModeManual, onResource(f.ResourceID), `{"teams": ["`+team.String()+`"]}`, noOne)
	_, err := svc.CreateRequest(bg, f.Principal(f.MemberID), access.CreateRequestInput{ResourceID: &f.ResourceID, Reason: "x"})
	require.ErrorIs(t, err, apperr.ErrPermissionDenied)
	assert.ErrorContains(t, err, "no approval policy matches this request")
}

func TestCreateRequestInput(t *testing.T) {
	svc, pool, f, _, _ := newTestService(t)
	f.ApprovalPolicy(t, pool, "prod", access.ModeManual, onResource(f.ResourceID), allUsers, noOne)
	p := f.Principal(f.MemberID)
	missing := uuid.New()
	long := make([]byte, 1025)
	for i := range long {
		long[i] = 'a'
	}
	for name, tc := range map[string]struct {
		in   access.CreateRequestInput
		want error
	}{
		"neither":          {access.CreateRequestInput{Reason: "x"}, apperr.ErrInvalidArgument},
		"both":             {access.CreateRequestInput{ResourceID: &f.ResourceID, Selector: map[string]string{"env": "prod"}, Reason: "x"}, apperr.ErrInvalidArgument},
		"bad label":        {access.CreateRequestInput{Selector: map[string]string{"Bad Key": "x"}, Reason: "x"}, apperr.ErrInvalidArgument},
		"long reason":      {access.CreateRequestInput{ResourceID: &f.ResourceID, Reason: string(long)}, apperr.ErrInvalidArgument},
		"unknown resource": {access.CreateRequestInput{ResourceID: &missing, Reason: "x"}, apperr.ErrNotFound},
		"empty selector":   {access.CreateRequestInput{Selector: map[string]string{"env": "none"}, Reason: "x"}, apperr.ErrInvalidArgument},
	} {
		_, err := svc.CreateRequest(bg, p, tc.in)
		assert.ErrorIs(t, err, tc.want, name)
	}
	_, err := svc.CreateRequest(bg, p, access.CreateRequestInput{Selector: map[string]string{"env": "none"}, Reason: "x"})
	assert.ErrorContains(t, err, "no resources match selector")
	other := dbtest.Seed(t, pool)
	_, err = svc.CreateRequest(bg, p, access.CreateRequestInput{ResourceID: &other.ResourceID, Reason: "x"})
	assert.ErrorIs(t, err, apperr.ErrNotFound, "a resource of another organization")
}

func TestCreateRequestReasonAndDuration(t *testing.T) {
	svc, pool, f, _, set := newTestService(t)
	f.ApprovalPolicy(t, pool, "prod", access.ModeAuto, onResource(f.ResourceID), allUsers, noOne)
	p := f.Principal(f.MemberID)
	_, err := svc.CreateRequest(bg, p, access.CreateRequestInput{ResourceID: &f.ResourceID, Reason: "  "})
	require.ErrorIs(t, err, apperr.ErrInvalidArgument)
	assert.ErrorContains(t, err, "reason is required")
	_, err = svc.CreateRequest(bg, p, access.CreateRequestInput{ResourceID: &f.ResourceID, Reason: "x", Duration: 9 * time.Hour})
	require.ErrorIs(t, err, apperr.ErrInvalidArgument)
	assert.ErrorContains(t, err, "duration exceeds 8h0m0s")

	set.S.GrantDefaultTTL = 4 * time.Hour
	a, err := svc.CreateRequest(bg, p, access.CreateRequestInput{ResourceID: &f.ResourceID, Reason: "x"})
	require.NoError(t, err)
	assert.Equal(t, 4*time.Hour, a.Duration)
	assert.WithinDuration(t, time.Now().Add(4*time.Hour), grants(t, pool, a.ID)[f.ResourceID], 5*time.Second)
}

func TestSoleOwnerCannotRequest(t *testing.T) {
	svc, pool, _, q, _ := newTestService(t)
	org := dbtest.Org(t, pool)
	owner := dbtest.User(t, pool, org, principal.RoleOwner)
	r := dbtest.Resource(t, pool, org, "db", nil)
	f := dbtest.Fixture{OrgID: org}
	f.ApprovalPolicy(t, pool, "all", access.ModeManual, onResource(r), allUsers, noOne)
	_, err := svc.CreateRequest(bg, principal.Principal{UserID: owner, OrgID: org, Role: principal.RoleOwner},
		access.CreateRequestInput{ResourceID: &r, Reason: "x"})
	require.ErrorIs(t, err, apperr.ErrFailedPrecondition)
	assert.ErrorContains(t, err, "no one can approve this request")
	var n int
	require.NoError(t, pool.QueryRow(bg, "SELECT count(*) FROM approval").Scan(&n))
	assert.Zero(t, n)
	assert.Empty(t, q.sent)
}

func TestBreakGlass(t *testing.T) {
	svc, pool, f, q, _ := newTestService(t)
	pol := f.ApprovalPolicy(t, pool, "bg", access.ModeManual, onResource(f.ResourceID), allUsers, noOne)
	setPolicy(t, pool, pol, "break_glass = true, require_reason = false")
	p := f.Principal(f.MemberID)
	in := access.CreateRequestInput{ResourceID: &f.ResourceID, BreakGlass: true}
	_, err := svc.CreateRequest(bg, p, in)
	assert.ErrorContains(t, err, "reason is required")
	in.Reason, in.Duration = "incident", 2*time.Hour
	_, err = svc.CreateRequest(bg, p, in)
	assert.ErrorContains(t, err, "duration exceeds 1h0m0s")

	in.Duration = 0
	a, err := svc.CreateRequest(bg, p, in)
	require.NoError(t, err)
	assert.Equal(t, access.StatusApproved, a.Status)
	assert.True(t, a.BreakGlass)
	var bgFlag, review bool
	var createdBy uuid.UUID
	require.NoError(t, pool.QueryRow(bg, "SELECT break_glass, needs_review, created_by FROM access_grant WHERE approval_id = $1", a.ID).
		Scan(&bgFlag, &review, &createdBy))
	assert.True(t, bgFlag)
	assert.True(t, review)
	assert.Equal(t, f.MemberID, createdBy)
	assert.Equal(t, []string{"access.request.create", "access.break_glass", "access.grant.create"}, auditActions(t, pool))
	assert.ElementsMatch(t, []string{emailOf(t, pool, f.OwnerID), emailOf(t, pool, f.AdminID)}, q.to("break_glass"))
	assert.Equal(t, "incident", q.of("break_glass")[0].Data["reason"])

	other := dbtest.Resource(t, pool, f.OrgID, "other", nil)
	f.ApprovalPolicy(t, pool, "plain", access.ModeManual, onResource(other), allUsers, noOne)
	_, err = svc.CreateRequest(bg, p, access.CreateRequestInput{ResourceID: &other, BreakGlass: true, Reason: "x"})
	require.ErrorIs(t, err, apperr.ErrPermissionDenied)
	assert.ErrorContains(t, err, "break-glass is not allowed for this resource")
}

func TestCancel(t *testing.T) {
	svc, pool, f, _, _ := newTestService(t)
	f.ApprovalPolicy(t, pool, "prod", access.ModeManual, onResource(f.ResourceID), allUsers, noOne)
	p := f.Principal(f.MemberID)
	a, err := svc.CreateRequest(bg, p, access.CreateRequestInput{ResourceID: &f.ResourceID, Reason: "x"})
	require.NoError(t, err)
	assert.ErrorIs(t, svc.Cancel(bg, f.Principal(f.OtherMemberID), a.ID), apperr.ErrNotFound)
	assert.ErrorIs(t, svc.Cancel(bg, f.Principal(f.OwnerID), a.ID), apperr.ErrNotFound, "only the requester cancels")
	assert.ErrorIs(t, svc.Cancel(bg, p, uuid.New()), apperr.ErrNotFound)
	require.NoError(t, svc.Cancel(bg, p, a.ID))
	assert.Equal(t, access.StatusCanceled, status(t, pool, a.ID))
	assert.Contains(t, auditActions(t, pool), "access.request.cancel")

	b, err := svc.CreateRequest(bg, p, access.CreateRequestInput{ResourceID: &f.ResourceID, Reason: "x"})
	require.NoError(t, err)
	_, err = svc.Decide(bg, f.Principal(f.AdminID), b.ID, true, "")
	require.NoError(t, err)
	err = svc.Cancel(bg, p, b.ID)
	require.ErrorIs(t, err, apperr.ErrFailedPrecondition)
	assert.ErrorContains(t, err, "request is not pending")
}
