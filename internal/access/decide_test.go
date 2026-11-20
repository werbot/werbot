package access_test

import (
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/access"
	"github.com/werbot/werbot/internal/platform/apperr"
	"github.com/werbot/werbot/internal/platform/db/dbtest"
)

// pending creates a pending 2h request of the member for the fixture resource under a manual policy.
func pending(t *testing.T, svc *access.Service, f dbtest.Fixture, requester uuid.UUID) access.Approval {
	t.Helper()
	a, err := svc.CreateRequest(bg, f.Principal(requester),
		access.CreateRequestInput{ResourceID: &f.ResourceID, Duration: 2 * time.Hour, Reason: "x"})
	require.NoError(t, err)
	require.Equal(t, access.StatusPending, a.Status)
	return a
}

func TestDecideApproveCreatesGrant(t *testing.T) {
	svc, pool, f, q, _ := newTestService(t)
	f.ApprovalPolicy(t, pool, "prod", access.ModeManual, onResource(f.ResourceID), allUsers, noOne)
	a := pending(t, svc, f, f.MemberID)
	q.reset()
	got, err := svc.Decide(bg, f.Principal(f.AdminID), a.ID, true, " ok ")
	require.NoError(t, err)
	assert.Equal(t, access.StatusApproved, got.Status)
	assert.Equal(t, &f.AdminID, got.DecidedBy)
	assert.Equal(t, "ok", got.Comment)
	require.NotNil(t, got.DecidedAt)
	g := grants(t, pool, a.ID)
	require.Len(t, g, 1)
	assert.WithinDuration(t, time.Now().Add(2*time.Hour), g[f.ResourceID], 5*time.Second)
	assert.Equal(t, []string{emailOf(t, pool, f.MemberID)}, q.to("access_decision"))
	assert.Equal(t, map[string]string{"url": "https://werbot.test/requests?id=" + a.ID.String(), "resource": "web",
		"status": "approved", "comment": "ok"}, q.of("access_decision")[0].Data)
	assert.Equal(t, []string{"access.request.create", "access.request.approve", "access.grant.create"}, auditActions(t, pool))
}

func TestDecideDeny(t *testing.T) {
	svc, pool, f, q, _ := newTestService(t)
	f.ApprovalPolicy(t, pool, "prod", access.ModeManual, onResource(f.ResourceID), allUsers, noOne)
	a := pending(t, svc, f, f.MemberID)
	got, err := svc.Decide(bg, f.Principal(f.OwnerID), a.ID, false, "not now")
	require.NoError(t, err)
	assert.Equal(t, access.StatusDenied, got.Status)
	assert.Empty(t, grants(t, pool, a.ID))
	m := q.of("access_decision")
	require.Len(t, m, 1)
	assert.Equal(t, "denied", m[0].Data["status"])
	assert.Equal(t, "not now", m[0].Data["comment"])
	assert.Contains(t, auditActions(t, pool), "access.request.deny")
}

func TestDecideVisibility(t *testing.T) {
	svc, pool, f, _, _ := newTestService(t)
	f.ApprovalPolicy(t, pool, "prod", access.ModeManual, onResource(f.ResourceID), allUsers, noOne)
	a := pending(t, svc, f, f.MemberID)
	_, err := svc.Decide(bg, f.Principal(f.OtherMemberID), a.ID, true, "")
	assert.ErrorIs(t, err, apperr.ErrNotFound, "a member who cannot decide does not see the request")
	_, err = svc.Decide(bg, f.Principal(f.MemberID), a.ID, true, "")
	assert.ErrorIs(t, err, apperr.ErrPermissionDenied, "the requester")
	_, err = svc.Decide(bg, f.Principal(f.AdminID), uuid.New(), true, "")
	assert.ErrorIs(t, err, apperr.ErrNotFound)
	other := dbtest.Seed(t, pool)
	_, err = svc.Decide(bg, other.Principal(other.OwnerID), a.ID, false, "")
	assert.ErrorIs(t, err, apperr.ErrNotFound, "an owner of another organization")
	assert.Equal(t, access.StatusPending, status(t, pool, a.ID))
}

func TestOwnerDeniesWithoutPolicy(t *testing.T) {
	svc, pool, f, _, _ := newTestService(t)
	pol := f.ApprovalPolicy(t, pool, "prod", access.ModeManual, onResource(f.ResourceID), allUsers, noOne)
	a := pending(t, svc, f, f.MemberID)
	_, err := pool.Exec(bg, "DELETE FROM approval_policy WHERE id = $1", pol)
	require.NoError(t, err)
	_, err = svc.Decide(bg, f.Principal(f.AdminID), a.ID, true, "")
	require.ErrorIs(t, err, apperr.ErrPermissionDenied)
	assert.ErrorContains(t, err, "you cannot decide this request")
	got, err := svc.Decide(bg, f.Principal(f.AdminID), a.ID, false, "")
	require.NoError(t, err, "owner and admin may deny without a matching policy")
	assert.Equal(t, access.StatusDenied, got.Status)
}

func TestSelfApprovalForbidden(t *testing.T) {
	svc, pool, f, _, _ := newTestService(t)
	f.ApprovalPolicy(t, pool, "prod", access.ModeManual, onResource(f.ResourceID), allUsers, noOne)
	a := pending(t, svc, f, f.AdminID)
	_, err := svc.Decide(bg, f.Principal(f.AdminID), a.ID, true, "")
	require.ErrorIs(t, err, apperr.ErrPermissionDenied)
	assert.ErrorContains(t, err, "you cannot decide this request")
	_, err = svc.Decide(bg, f.Principal(f.AdminID), a.ID, false, "")
	assert.ErrorContains(t, err, "you cannot decide this request", "nor deny own request")
	got, err := svc.Decide(bg, f.Principal(f.OwnerID), a.ID, true, "")
	require.NoError(t, err)
	assert.Equal(t, access.StatusApproved, got.Status)
}

func TestDecideNamedApprovers(t *testing.T) {
	svc, pool, f, q, _ := newTestService(t)
	team := dbtest.Team(t, pool, f.OrgID, f.OtherMemberID)
	f.ApprovalPolicy(t, pool, "prod", access.ModeManual, onResource(f.ResourceID), allUsers, `{"teams": ["`+team.String()+`"]}`)
	a := pending(t, svc, f, f.MemberID)
	assert.Equal(t, []string{emailOf(t, pool, f.OtherMemberID)}, q.to("access_request"), "only the named team")
	_, err := svc.Decide(bg, f.Principal(f.AdminID), a.ID, true, "")
	assert.ErrorIs(t, err, apperr.ErrPermissionDenied, "admin is not a named approver")
	got, err := svc.Decide(bg, f.Principal(f.OtherMemberID), a.ID, true, "")
	require.NoError(t, err)
	assert.Equal(t, access.StatusApproved, got.Status)
}

func TestDecideAfterExpiryFails(t *testing.T) {
	svc, pool, f, _, _ := newTestService(t)
	f.ApprovalPolicy(t, pool, "prod", access.ModeManual, onResource(f.ResourceID), allUsers, noOne)
	a := pending(t, svc, f, f.MemberID)
	_, err := pool.Exec(bg, "UPDATE approval SET expires_at = now() - interval '1 second' WHERE id = $1", a.ID)
	require.NoError(t, err)
	_, err = svc.Decide(bg, f.Principal(f.AdminID), a.ID, true, "")
	require.ErrorIs(t, err, apperr.ErrFailedPrecondition)
	assert.ErrorContains(t, err, "request expired")
	assert.Equal(t, access.StatusPending, status(t, pool, a.ID))
	assert.Empty(t, grants(t, pool, a.ID))
}

func TestDecideTwice(t *testing.T) {
	svc, pool, f, _, _ := newTestService(t)
	f.ApprovalPolicy(t, pool, "prod", access.ModeManual, onResource(f.ResourceID), allUsers, noOne)
	a := pending(t, svc, f, f.MemberID)
	_, err := svc.Decide(bg, f.Principal(f.AdminID), a.ID, false, "")
	require.NoError(t, err)
	_, err = svc.Decide(bg, f.Principal(f.OwnerID), a.ID, true, "")
	require.ErrorIs(t, err, apperr.ErrFailedPrecondition)
	assert.ErrorContains(t, err, "request is not pending")
	assert.Empty(t, grants(t, pool, a.ID))
}

func TestDecideConcurrentOneWinner(t *testing.T) {
	svc, pool, f, _, _ := newTestService(t)
	f.ApprovalPolicy(t, pool, "prod", access.ModeManual, onResource(f.ResourceID), allUsers, noOne)
	a := pending(t, svc, f, f.MemberID)
	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i, who := range []uuid.UUID{f.OwnerID, f.AdminID} {
		wg.Go(func() { _, errs[i] = svc.Decide(bg, f.Principal(who), a.ID, true, "") })
	}
	wg.Wait()
	ok := 0
	for _, err := range errs {
		if err == nil {
			ok++
		} else {
			assert.ErrorContains(t, err, "request is not pending")
		}
	}
	assert.Equal(t, 1, ok)
	assert.Len(t, grants(t, pool, a.ID), 1)
}

func TestApproveSelectorExpandsAtDecision(t *testing.T) {
	svc, pool, f, _, _ := newTestService(t)
	r1 := dbtest.Resource(t, pool, f.OrgID, "db1", map[string]string{"env": "prod"})
	r2 := dbtest.Resource(t, pool, f.OrgID, "db2", nil)
	f.ApprovalPolicy(t, pool, "prod", access.ModeManual, `{"labels": {"env": "prod"}}`, allUsers, noOne)
	p := f.Principal(f.MemberID)
	_, err := svc.CreateRequest(bg, p, access.CreateRequestInput{Selector: map[string]string{"env": "stage"}, Reason: "x"})
	require.ErrorIs(t, err, apperr.ErrInvalidArgument)
	assert.ErrorContains(t, err, "no resources match selector")

	a, err := svc.CreateRequest(bg, p, access.CreateRequestInput{Selector: map[string]string{"env": "prod"}, Reason: "x"})
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"env": "prod"}, a.Selector)
	assert.Nil(t, a.ResourceID)
	_, err = pool.Exec(bg, "INSERT INTO resource_label (resource_id, org_id, key, value) VALUES ($1, $2, 'env', 'prod')", r2, f.OrgID)
	require.NoError(t, err)
	_, err = svc.Decide(bg, f.Principal(f.AdminID), a.ID, true, "")
	require.NoError(t, err)
	g := grants(t, pool, a.ID)
	assert.Len(t, g, 2)
	assert.Contains(t, g, r1)
	assert.Contains(t, g, r2)

	b, err := svc.CreateRequest(bg, p, access.CreateRequestInput{Selector: map[string]string{"env": "prod"}, Reason: "x"})
	require.NoError(t, err)
	_, err = pool.Exec(bg, "DELETE FROM resource_label WHERE key = 'env'")
	require.NoError(t, err)
	_, err = svc.Decide(bg, f.Principal(f.AdminID), b.ID, true, "")
	require.ErrorIs(t, err, apperr.ErrInvalidArgument)
	assert.ErrorContains(t, err, "no resources match selector")
	assert.Equal(t, access.StatusPending, status(t, pool, b.ID))
}
