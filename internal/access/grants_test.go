package access_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/access"
	"github.com/werbot/werbot/internal/platform/apperr"
	"github.com/werbot/werbot/internal/platform/db/dbtest"
)

// eventData returns the data of every audit event with action of target id, oldest first.
func eventData(t *testing.T, pool *pgxpool.Pool, action string, id uuid.UUID) []map[string]any {
	t.Helper()
	rows, err := pool.Query(bg, "SELECT data FROM audit_event WHERE action = $1 AND target_id = $2 ORDER BY id", action, id.String())
	require.NoError(t, err)
	out, err := pgx.CollectRows(rows, pgx.RowTo[map[string]any])
	require.NoError(t, err)
	return out
}

// breakGlassGrant inserts a break-glass grant of user that needs a review.
func breakGlassGrant(t *testing.T, pool *pgxpool.Pool, f dbtest.Fixture, user uuid.UUID) uuid.UUID {
	t.Helper()
	id := f.Grant(t, pool, user, f.ResourceID, time.Hour)
	_, err := pool.Exec(bg, "UPDATE access_grant SET break_glass = true, needs_review = true WHERE id = $1", id)
	require.NoError(t, err)
	return id
}

func TestActiveGrantsFilters(t *testing.T) {
	svc, pool, f, _, _ := newTestService(t)
	active := f.Grant(t, pool, f.MemberID, f.ResourceID, time.Hour)
	f.Grant(t, pool, f.MemberID, f.ResourceID, -time.Second)
	revoked := f.Grant(t, pool, f.MemberID, f.ResourceID, time.Hour)
	_, err := pool.Exec(bg, "UPDATE access_grant SET revoked_at = now() WHERE id = $1", revoked)
	require.NoError(t, err)
	other := dbtest.Seed(t, pool)
	other.Grant(t, pool, other.MemberID, other.ResourceID, time.Hour)

	got, err := svc.ActiveGrants(bg, f.OrgID)
	require.NoError(t, err)
	require.Len(t, got, 1)
	g := got[0]
	assert.Equal(t, active, g.ID)
	assert.Equal(t, f.OrgID, g.OrgID)
	assert.Equal(t, f.MemberID, g.UserID)
	assert.Equal(t, f.ResourceID, g.ResourceID)
	assert.WithinDuration(t, time.Now().Add(time.Hour), g.ExpiresAt, 5*time.Second)
}

func TestRevokeGrant(t *testing.T) {
	svc, pool, f, _, _ := newTestService(t)
	g := f.Grant(t, pool, f.MemberID, f.ResourceID, time.Hour)
	changed := listenOnce(t, pool, "werbot_org_changed")

	require.NoError(t, svc.RevokeGrant(bg, f.Principal(f.AdminID), g, " leaked key "))
	assert.Equal(t, f.OrgID.String(), <-changed, "the snapshot of the organization is rebuilt")
	var revoked *time.Time
	require.NoError(t, pool.QueryRow(bg, "SELECT revoked_at FROM access_grant WHERE id = $1", g).Scan(&revoked))
	require.NotNil(t, revoked)
	data := eventData(t, pool, "access.grant.revoke", g)
	require.Len(t, data, 1)
	assert.Equal(t, "leaked key", data[0]["reason"])
	assert.Equal(t, f.MemberID.String(), data[0]["user_id"])
	assert.Equal(t, f.ResourceID.String(), data[0]["resource_id"])

	require.NoError(t, svc.RevokeGrant(bg, f.Principal(f.AdminID), g, "again"), "a revoked grant stays revoked")
	assert.Len(t, eventData(t, pool, "access.grant.revoke", g), 1, "no second event")

	foreign := f.Grant(t, pool, f.OtherMemberID, f.ResourceID, time.Hour)
	require.ErrorIs(t, svc.RevokeGrant(bg, f.Principal(f.MemberID), foreign, ""), apperr.ErrPermissionDenied)
	assert.Empty(t, eventData(t, pool, "access.grant.revoke", foreign))

	own := f.Grant(t, pool, f.MemberID, f.ResourceID, time.Hour)
	require.NoError(t, svc.RevokeGrant(bg, f.Principal(f.MemberID), own, ""))
	assert.Len(t, eventData(t, pool, "access.grant.revoke", own), 1)

	other := dbtest.Seed(t, pool)
	otherGrant := other.Grant(t, pool, other.MemberID, other.ResourceID, time.Hour)
	for name, id := range map[string]uuid.UUID{"unknown": uuid.New(), "other org": otherGrant} {
		require.ErrorIs(t, svc.RevokeGrant(bg, f.Principal(f.OwnerID), id, ""), apperr.ErrNotFound, name)
	}
}

func TestReviewBreakGlass(t *testing.T) {
	svc, pool, f, _, _ := newTestService(t)
	g := breakGlassGrant(t, pool, f, f.AdminID)

	require.ErrorIs(t, svc.ReviewBreakGlass(bg, f.Principal(f.AdminID), g, "mine"), apperr.ErrPermissionDenied, "the grant owner never reviews")
	require.ErrorIs(t, svc.ReviewBreakGlass(bg, f.Principal(f.MemberID), g, "ok"), apperr.ErrPermissionDenied, "a member never reviews")
	assert.Empty(t, eventData(t, pool, "access.break_glass.review", g))

	require.NoError(t, svc.ReviewBreakGlass(bg, f.Principal(f.OwnerID), g, " justified "))
	var needs bool
	require.NoError(t, pool.QueryRow(bg, "SELECT needs_review FROM access_grant WHERE id = $1", g).Scan(&needs))
	assert.False(t, needs)
	data := eventData(t, pool, "access.break_glass.review", g)
	require.Len(t, data, 1)
	assert.Equal(t, "justified", data[0]["comment"])
	assert.Equal(t, f.AdminID.String(), data[0]["user_id"])

	require.ErrorIs(t, svc.ReviewBreakGlass(bg, f.Principal(f.OwnerID), g, ""), apperr.ErrFailedPrecondition, "reviewed once")
	plain := f.Grant(t, pool, f.MemberID, f.ResourceID, time.Hour)
	require.ErrorIs(t, svc.ReviewBreakGlass(bg, f.Principal(f.OwnerID), plain, ""), apperr.ErrFailedPrecondition, "not break-glass")
	other := dbtest.Seed(t, pool)
	require.ErrorIs(t, svc.ReviewBreakGlass(bg, f.Principal(f.OwnerID), breakGlassGrant(t, pool, other, other.MemberID), ""),
		apperr.ErrNotFound, "other org")
}

func TestListGrantsMemberOwnOnly(t *testing.T) {
	svc, pool, f, _, _ := newTestService(t)
	mine := f.Grant(t, pool, f.MemberID, f.ResourceID, time.Hour)
	expired := f.Grant(t, pool, f.MemberID, f.ResourceID, -time.Second)
	theirs := f.Grant(t, pool, f.OtherMemberID, f.ResourceID, time.Hour)
	other := dbtest.Seed(t, pool)
	other.Grant(t, pool, other.MemberID, other.ResourceID, time.Hour)
	ids := func(vs []access.GrantView) (out []uuid.UUID) {
		for _, v := range vs {
			out = append(out, v.ID)
		}
		return out
	}

	_, err := svc.ListGrants(bg, f.Principal(f.MemberID), access.GrantFilter{UserID: &f.OtherMemberID})
	require.ErrorIs(t, err, apperr.ErrPermissionDenied)

	got, err := svc.ListGrants(bg, f.Principal(f.MemberID), access.GrantFilter{})
	require.NoError(t, err)
	assert.ElementsMatch(t, []uuid.UUID{mine, expired}, ids(got))
	got, err = svc.ListGrants(bg, f.Principal(f.MemberID), access.GrantFilter{UserID: &f.MemberID, Active: true})
	require.NoError(t, err)
	require.Equal(t, []uuid.UUID{mine}, ids(got))
	assert.Equal(t, "web", got[0].ResourceName)
	assert.NotEmpty(t, got[0].UserLogin)
	assert.False(t, got[0].CreatedAt.IsZero())

	got, err = svc.ListGrants(bg, f.Principal(f.AdminID), access.GrantFilter{Active: true})
	require.NoError(t, err)
	assert.ElementsMatch(t, []uuid.UUID{mine, theirs}, ids(got), "owner and admin see every grant of the organization")
	unknown := uuid.New()
	got, err = svc.ListGrants(bg, f.Principal(f.AdminID), access.GrantFilter{ResourceID: &unknown})
	require.NoError(t, err)
	assert.Empty(t, got)
}
