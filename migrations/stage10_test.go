package migrations_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/platform/db/dbtest"
	"github.com/werbot/werbot/migrations"
)

const insertApproval = `INSERT INTO approval (org_id, kind, requester_user_id, resource_id, status, expires_at)
	VALUES ($1, 'access', $2, $3, %s, now() + interval '1 hour')`

func TestMigrations10(t *testing.T) {
	pool, _ := dbtest.New(t)
	ctx := context.Background()
	sqlDB := stdlib.OpenDBFromPool(pool)
	defer func() { _ = sqlDB.Close() }()
	p, err := goose.NewProvider(goose.DialectPostgres, sqlDB, migrations.FS)
	require.NoError(t, err)

	_, err = p.DownTo(ctx, 119)
	require.NoError(t, err)
	old := dbtest.Org(t, pool)
	_, err = p.Up(ctx)
	require.NoError(t, err)

	var ttls [5]string
	require.NoError(t, pool.QueryRow(ctx, `SELECT grant_default_ttl::text, grant_max_ttl::text, request_pending_ttl::text,
		command_approval_ttl::text, lock_mode FROM org_setting WHERE org_id = $1`, old).
		Scan(&ttls[0], &ttls[1], &ttls[2], &ttls[3], &ttls[4]))
	assert.Equal(t, [5]string{"08:00:00", "14 days", "01:00:00", "00:05:00", "best_effort"}, ttls)

	f := dbtest.Seed(t, pool)
	pid := f.ApprovalPolicy(t, pool, "p", "manual", `{}`, `{}`, `{}`)
	assert.NotEqual(t, uuid.Nil, pid)
	rid := f.Lock(t, pool, "all_agents", nil)
	assert.NotEqual(t, uuid.Nil, rid)

	for name, q := range map[string]string{
		"policy mode":         `INSERT INTO approval_policy (org_id, name, mode, resource_selector, requesters) VALUES ($1, 'x', 'bogus', '{}', '{}')`,
		"policy max_duration": `INSERT INTO approval_policy (org_id, name, mode, resource_selector, requesters, max_duration) VALUES ($1, 'x', 'auto', '{}', '{}', '15 days')`,
		"approval status":     `INSERT INTO approval (org_id, kind, requester_user_id, status, expires_at) VALUES ($1, 'access', gen_random_uuid(), 'bogus', now())`,
		"approval kind":       `INSERT INTO approval (org_id, kind, requester_user_id, status, expires_at) VALUES ($1, 'bogus', gen_random_uuid(), 'pending', now())`,
		"all_agents target":   `INSERT INTO lock (org_id, target_type, target_id, reason) VALUES ($1, 'all_agents', gen_random_uuid(), 'r')`,
		"user without target": `INSERT INTO lock (org_id, target_type, reason) VALUES ($1, 'user', 'r')`,
		"lock empty reason":   `INSERT INTO lock (org_id, target_type, target_id, reason) VALUES ($1, 'user', gen_random_uuid(), '')`,
		"lock target type":    `INSERT INTO lock (org_id, target_type, target_id, reason) VALUES ($1, 'bogus', gen_random_uuid(), 'r')`,
	} {
		_, err := pool.Exec(ctx, q, f.OrgID)
		require.Error(t, err, name)
		pgCode(t, err, "23514")
	}
	for name, q := range map[string]string{
		"lock_mode":         "UPDATE org_setting SET lock_mode = 'paranoid'",
		"grant_default_ttl": "UPDATE org_setting SET grant_default_ttl = '15 days'",
		"default above max": "UPDATE org_setting SET grant_default_ttl = '2 days', grant_max_ttl = '1 day'",
		"zero default":      "UPDATE org_setting SET grant_default_ttl = '0'",
		"pending ttl":       "UPDATE org_setting SET request_pending_ttl = '25 hours'",
		"command ttl":       "UPDATE org_setting SET command_approval_ttl = '10 seconds'",
	} {
		_, err := pool.Exec(ctx, q)
		require.Error(t, err, name)
		pgCode(t, err, "23514")
	}

	_, err = pool.Exec(ctx, "UPDATE org_setting SET lock_mode = 'strict' WHERE org_id = $1", f.OrgID)
	require.NoError(t, err)
	_, err = p.DownTo(ctx, 119)
	require.NoError(t, err)
	_, err = p.Up(ctx)
	require.NoError(t, err)
}

func TestAccessGrantCrossOrgRejected(t *testing.T) {
	pool, _ := dbtest.New(t)
	ctx := context.Background()
	a, b := dbtest.Seed(t, pool), dbtest.Seed(t, pool)

	_, err := pool.Exec(ctx, "INSERT INTO access_grant (org_id, user_id, resource_id, expires_at) VALUES ($1, $2, $3, now())",
		a.OrgID, a.MemberID, b.ResourceID)
	pgCode(t, err, "23503")
	_, err = pool.Exec(ctx, "INSERT INTO access_grant (org_id, user_id, resource_id, expires_at) VALUES ($1, $2, $3, now())",
		a.OrgID, b.MemberID, a.ResourceID)
	pgCode(t, err, "23503")

	a.Grant(t, pool, a.MemberID, a.ResourceID, time.Hour)
	_, err = pool.Exec(ctx, fmtApproval("'pending'"), a.OrgID, a.MemberID, a.ResourceID)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, "DELETE FROM membership WHERE org_id = $1 AND user_id = $2", a.OrgID, a.MemberID)
	require.NoError(t, err)
	assertCount(t, pool, "access_grant", a.OrgID, 0)
	assertCount(t, pool, "approval", a.OrgID, 0)
}

func TestApprovalKeepsHistoryOnResourceDelete(t *testing.T) {
	pool, _ := dbtest.New(t)
	ctx := context.Background()
	f := dbtest.Seed(t, pool)
	f.Grant(t, pool, f.MemberID, f.ResourceID, time.Hour)
	_, err := pool.Exec(ctx, fmtApproval("'approved'"), f.OrgID, f.MemberID, f.ResourceID)
	require.NoError(t, err)

	_, err = pool.Exec(ctx, "DELETE FROM resource WHERE id = $1", f.ResourceID)
	require.NoError(t, err)

	var resourceID *uuid.UUID
	var org uuid.UUID
	require.NoError(t, pool.QueryRow(ctx, "SELECT resource_id, org_id FROM approval WHERE org_id = $1", f.OrgID).Scan(&resourceID, &org))
	assert.Nil(t, resourceID)
	assert.Equal(t, f.OrgID, org)
	assertCount(t, pool, "access_grant", f.OrgID, 0)
}

func TestJITTablesNotifyOrgChanged(t *testing.T) {
	pool, _ := dbtest.New(t)
	ctx := context.Background()
	f := dbtest.Seed(t, pool)
	conn, err := pool.Acquire(ctx)
	require.NoError(t, err)
	defer conn.Release()
	_, err = conn.Exec(ctx, "LISTEN werbot_org_changed")
	require.NoError(t, err)

	// no notification: approval_policy and approval are not part of the snapshot
	f.ApprovalPolicy(t, pool, "p", "manual", `{}`, `{}`, `{}`)
	_, err = pool.Exec(ctx, fmtApproval("'pending'"), f.OrgID, f.MemberID, f.ResourceID)
	require.NoError(t, err)

	grant := f.Grant(t, pool, f.MemberID, f.ResourceID, time.Hour)
	_, err = pool.Exec(ctx, "UPDATE access_grant SET revoked_at = now() WHERE id = $1", grant)
	require.NoError(t, err)
	lock := f.Lock(t, pool, "all_agents", nil)
	_, err = pool.Exec(ctx, "DELETE FROM lock WHERE id = $1", lock)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, "UPDATE org_setting SET lock_mode = 'strict' WHERE org_id = $1", f.OrgID)
	require.NoError(t, err)

	// a second organization marks the end: notifications arrive in commit order
	other := dbtest.Org(t, pool)
	got := 0
	for {
		wctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		n, err := conn.Conn().WaitForNotification(wctx)
		cancel()
		require.NoError(t, err)
		if n.Payload == other.String() {
			break
		}
		assert.Equal(t, f.OrgID.String(), n.Payload)
		got++
	}
	assert.Equal(t, 5, got, "grant insert+update, lock insert+delete, org_setting update")
}

func fmtApproval(status string) string { return fmt.Sprintf(insertApproval, status) }

func assertCount(t *testing.T, pool *pgxpool.Pool, table string, org uuid.UUID, want int) {
	t.Helper()
	var n int
	require.NoError(t, pool.QueryRow(context.Background(), "SELECT count(*) FROM "+table+" WHERE org_id = $1", org).Scan(&n))
	assert.Equal(t, want, n, table)
}
