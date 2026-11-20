package access_test

import (
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/access"
	"github.com/werbot/werbot/internal/platform/jobs"
)

// age moves the deadline of approval id into the past.
func age(t *testing.T, pool *pgxpool.Pool, id uuid.UUID) {
	t.Helper()
	_, err := pool.Exec(bg, "UPDATE approval SET expires_at = now() - interval '1 minute' WHERE id = $1", id)
	require.NoError(t, err)
}

func TestExpire(t *testing.T) {
	svc, pool, f, q, _ := newTestService(t)
	f.ApprovalPolicy(t, pool, "prod", access.ModeManual, onResource(f.ResourceID), allUsers, noOne)
	stale := pending(t, svc, f, f.MemberID)
	fresh := pending(t, svc, f, f.MemberID)
	done := pending(t, svc, f, f.MemberID)
	_, err := svc.Decide(bg, f.Principal(f.OwnerID), done.ID, true, "")
	require.NoError(t, err)
	cmd, err := svc.RequestCommandApproval(bg, commandInput(f))
	require.NoError(t, err)
	age(t, pool, stale.ID)
	age(t, pool, done.ID)
	age(t, pool, cmd.ID)
	q.Reset()

	n, err := svc.Expire(bg)
	require.NoError(t, err)
	assert.EqualValues(t, 2, n)
	assert.Equal(t, access.StatusExpired, status(t, pool, stale.ID))
	assert.Equal(t, access.StatusExpired, status(t, pool, cmd.ID))
	assert.Equal(t, access.StatusPending, status(t, pool, fresh.ID))
	assert.Equal(t, access.StatusApproved, status(t, pool, done.ID))

	var events, system int
	require.NoError(t, pool.QueryRow(bg, `SELECT count(*), count(*) FILTER (WHERE actor_kind = 'system' AND actor_user_id IS NULL)
		FROM audit_event WHERE action = 'approval.expire'`).Scan(&events, &system))
	assert.Equal(t, 2, events)
	assert.Equal(t, 2, system)
	assert.Equal(t, []string{emailOf(t, pool, f.MemberID)}, q.To("access_decision"), "mail for the access request only")
	assert.Equal(t, map[string]string{"url": "https://werbot.test/requests?id=" + stale.ID.String(), "resource": "web",
		"status": "expired", "comment": ""}, q.Of("access_decision")[0].Data)

	n, err = svc.Expire(bg)
	require.NoError(t, err)
	assert.Zero(t, n, "second run finds nothing")
	assert.Len(t, q.Of("access_decision"), 1)
}

func TestExpireNeverApproves(t *testing.T) {
	svc, pool, f, _, _ := newTestService(t)
	id := f.ApprovalPolicy(t, pool, "prod", access.ModeManual, onResource(f.ResourceID), allUsers, noOne)
	a := pending(t, svc, f, f.MemberID)
	setPolicy(t, pool, id, "mode = 'auto'")
	age(t, pool, a.ID)

	n, err := svc.Expire(bg)
	require.NoError(t, err)
	assert.EqualValues(t, 1, n)
	assert.Equal(t, access.StatusExpired, status(t, pool, a.ID))
	assert.Empty(t, grants(t, pool, a.ID))
}

func TestExpireRacesApprove(t *testing.T) {
	svc, pool, f, _, _ := newTestService(t)
	f.ApprovalPolicy(t, pool, "prod", access.ModeManual, onResource(f.ResourceID), allUsers, noOne)
	for range 100 {
		a := pending(t, svc, f, f.MemberID)
		var deadline time.Time
		require.NoError(t, pool.QueryRow(bg,
			"UPDATE approval SET expires_at = now() + interval '50 milliseconds' WHERE id = $1 RETURNING expires_at", a.ID).Scan(&deadline))
		time.Sleep(time.Until(deadline) - 2*time.Millisecond)
		var wg sync.WaitGroup
		wg.Go(func() { _, _ = svc.Decide(bg, f.Principal(f.OwnerID), a.ID, true, "") })
		wg.Go(func() { _, _ = svc.Expire(bg) })
		wg.Wait()
		_, err := svc.Expire(bg) // Expire may have run before the deadline
		require.NoError(t, err)
		switch status(t, pool, a.ID) {
		case access.StatusApproved:
			assert.Len(t, grants(t, pool, a.ID), 1)
		case access.StatusExpired:
			assert.Empty(t, grants(t, pool, a.ID))
		default:
			t.Fatalf("status %s", status(t, pool, a.ID))
		}
	}
}

func TestRegisterApprovalJobs(t *testing.T) {
	periodic := access.RegisterApprovalJobs(river.NewWorkers(), access.New(nil, nil, nil, access.Config{}))
	assert.Len(t, periodic, 1)
	assert.Equal(t, "approval.expire", access.ExpireArgs{}.Kind())
	assert.Equal(t, jobs.QueueMaintenance, access.ExpireArgs{}.InsertOpts().Queue)
}
