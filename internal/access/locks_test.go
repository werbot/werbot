package access_test

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/access"
	"github.com/werbot/werbot/internal/platform/apperr"
	"github.com/werbot/werbot/internal/platform/db/dbtest"
)

// cliSession inserts a live CLI session of user in org.
func cliSession(t *testing.T, pool *pgxpool.Pool, org, user uuid.UUID) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	require.NoError(t, pool.QueryRow(bg, `INSERT INTO cli_session (org_id, user_id, client_id, refresh_hash, expires_at, absolute_expires_at)
		VALUES ($1, $2, 'werbot-cli', uuid_send(gen_random_uuid()), now() + interval '1 hour', now() + interval '1 day') RETURNING id`,
		org, user).Scan(&id))
	return id
}

// lockTargets returns one target of each supported type in the organization of f.
func lockTargets(t *testing.T, pool *pgxpool.Pool, f dbtest.Fixture) map[string]uuid.UUID {
	t.Helper()
	return map[string]uuid.UUID{
		"user":        f.MemberID,
		"team":        dbtest.Team(t, pool, f.OrgID, f.MemberID),
		"resource":    f.ResourceID,
		"grant":       f.Grant(t, pool, f.MemberID, f.ResourceID, time.Hour),
		"cli_session": cliSession(t, pool, f.OrgID, f.MemberID),
	}
}

func TestCreateLockTargets(t *testing.T) {
	svc, pool, f, _, _ := newTestService(t)
	admin := f.Principal(f.AdminID)
	for typ, id := range lockTargets(t, pool, f) {
		l, err := svc.CreateLock(bg, admin, access.LockInput{TargetType: typ, TargetID: &id, Reason: " incident "})
		require.NoError(t, err, typ)
		assert.Equal(t, f.OrgID, l.OrgID, typ)
		assert.Equal(t, typ, l.TargetType)
		assert.Equal(t, &id, l.TargetID, typ)
		assert.Equal(t, "incident", l.Reason, typ)
		assert.Nil(t, l.ExpiresAt, typ)
	}

	other := dbtest.Seed(t, pool)
	for typ, id := range lockTargets(t, pool, other) {
		_, err := svc.CreateLock(bg, admin, access.LockInput{TargetType: typ, TargetID: &id, Reason: "x"})
		require.ErrorIs(t, err, apperr.ErrNotFound, "target of another org: "+typ)
	}
	unknown := uuid.New()
	_, err := svc.CreateLock(bg, admin, access.LockInput{TargetType: "user", TargetID: &unknown, Reason: "x"})
	require.ErrorIs(t, err, apperr.ErrNotFound)

	_, err = svc.CreateLock(bg, admin, access.LockInput{TargetType: "mfa_device", TargetID: &unknown, Reason: "x"})
	require.ErrorIs(t, err, apperr.ErrInvalidArgument)
	assert.ErrorContains(t, err, "target type mfa_device is not supported yet")
	for name, in := range map[string]access.LockInput{
		"empty reason": {TargetType: "user", TargetID: &f.MemberID, Reason: "  "},
		"long reason":  {TargetType: "user", TargetID: &f.MemberID, Reason: strings.Repeat("a", 1025)},
		"no target":    {TargetType: "user", Reason: "x"},
		"unknown type": {TargetType: "bogus", TargetID: &f.MemberID, Reason: "x"},
		"negative ttl": {TargetType: "user", TargetID: &f.MemberID, Reason: "x", ExpiresIn: -time.Second},
	} {
		_, err := svc.CreateLock(bg, admin, in)
		require.ErrorIs(t, err, apperr.ErrInvalidArgument, name)
	}
	_, err = svc.CreateLock(bg, f.Principal(f.MemberID), access.LockInput{TargetType: "user", TargetID: &f.OtherMemberID, Reason: "x"})
	require.ErrorIs(t, err, apperr.ErrPermissionDenied, "a member never locks")

	var n int
	require.NoError(t, pool.QueryRow(bg, "SELECT count(*) FROM lock").Scan(&n))
	assert.Equal(t, 5, n, "only the five valid locks are stored")
}

func TestLockExpiry(t *testing.T) {
	svc, pool, f, _, _ := newTestService(t)
	admin := f.Principal(f.AdminID)
	timed, err := svc.CreateLock(bg, admin, access.LockInput{TargetType: "user", TargetID: &f.MemberID, Reason: "x", ExpiresIn: time.Minute})
	require.NoError(t, err)
	require.NotNil(t, timed.ExpiresAt)
	assert.WithinDuration(t, time.Now().Add(time.Minute), *timed.ExpiresAt, 5*time.Second)
	forever, err := svc.CreateLock(bg, admin, access.LockInput{TargetType: "resource", TargetID: &f.ResourceID, Reason: "x"})
	require.NoError(t, err)
	assert.Nil(t, forever.ExpiresAt)
	other := dbtest.Seed(t, pool)
	other.Lock(t, pool, "user", &other.MemberID)
	ids := func(ls []access.Lock) (out []uuid.UUID) {
		for _, l := range ls {
			out = append(out, l.ID)
		}
		return out
	}

	got, err := svc.ActiveLocks(bg, f.OrgID)
	require.NoError(t, err)
	assert.Equal(t, []uuid.UUID{timed.ID, forever.ID}, ids(got), "by id; only the organization's")

	_, err = pool.Exec(bg, "UPDATE lock SET expires_at = now() - interval '1 second' WHERE id = $1", timed.ID)
	require.NoError(t, err)
	got, err = svc.ActiveLocks(bg, f.OrgID)
	require.NoError(t, err)
	assert.Equal(t, []uuid.UUID{forever.ID}, ids(got))
	views, err := svc.ListLocks(bg, admin)
	require.NoError(t, err)
	require.Len(t, views, 1, "ListLocks lists active locks only")
	assert.Equal(t, forever.ID, views[0].ID)
}

func TestDeleteLockAudited(t *testing.T) {
	svc, pool, f, _, _ := newTestService(t)
	admin := f.Principal(f.AdminID)
	first, err := svc.CreateLock(bg, admin, access.LockInput{TargetType: "user", TargetID: &f.MemberID, Reason: "x"})
	require.NoError(t, err)
	l, err := svc.CreateLock(bg, admin, access.LockInput{TargetType: "resource", TargetID: &f.ResourceID, Reason: "x"})
	require.NoError(t, err)
	want := map[string]any{"target_type": "resource", "target_id": f.ResourceID.String()}
	assert.Equal(t, []map[string]any{want}, eventData(t, pool, "lock.create", l.ID))

	views, err := svc.ListLocks(bg, admin)
	require.NoError(t, err)
	require.Len(t, views, 2)
	assert.Equal(t, []uuid.UUID{l.ID, first.ID}, []uuid.UUID{views[0].ID, views[1].ID}, "newest first")
	assert.Equal(t, &f.AdminID, views[0].CreatedBy)
	assert.False(t, views[0].CreatedAt.IsZero())

	require.ErrorIs(t, svc.DeleteLock(bg, f.Principal(f.MemberID), l.ID), apperr.ErrPermissionDenied, "a member never unlocks")
	changed := listenOnce(t, pool, "werbot_org_changed")
	require.NoError(t, svc.DeleteLock(bg, admin, l.ID))
	assert.Equal(t, f.OrgID.String(), <-changed)
	assert.Equal(t, []map[string]any{want}, eventData(t, pool, "lock.delete", l.ID))
	require.ErrorIs(t, svc.DeleteLock(bg, admin, l.ID), apperr.ErrNotFound, "deleted once")
	other := dbtest.Seed(t, pool)
	require.ErrorIs(t, svc.DeleteLock(bg, admin, other.Lock(t, pool, "user", &other.MemberID)), apperr.ErrNotFound, "other org")
	_, err = svc.ListLocks(bg, f.Principal(f.MemberID))
	require.ErrorIs(t, err, apperr.ErrPermissionDenied)
}
