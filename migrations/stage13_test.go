package migrations_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/platform/db/dbtest"
	"github.com/werbot/werbot/migrations"
)

// insertTicket inserts a stream ticket of org and returns the insert error.
func insertTicket(ctx context.Context, pool *pgxpool.Pool, org uuid.UUID, mode string, sessionID, resourceID *uuid.UUID, userID, instanceID uuid.UUID) error {
	_, err := pool.Exec(ctx, `INSERT INTO stream_ticket (org_id, mode, session_id, resource_id, user_id, gateway_instance_id, target_server_id, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, now() + interval '30 seconds')`, org, mode, sessionID, resourceID, userID, instanceID, uuid.New())
	return err
}

// TestMigrations13: session.front takes only ssh or web; a stream ticket names
// a session exactly when it observes and a resource exactly when it opens a
// web terminal; 00180–00182 go down and up.
func TestMigrations13(t *testing.T) {
	pool, _ := dbtest.New(t)
	ctx := context.Background()
	f := dbtest.Seed(t, pool)
	sid := f.StartSession(t, pool, f.MemberID)

	var front string
	require.NoError(t, pool.QueryRow(ctx, "SELECT front FROM session WHERE id = $1", sid).Scan(&front))
	assert.Equal(t, "ssh", front, "an existing session is an ssh one")
	_, err := pool.Exec(ctx, "UPDATE session SET front = 'bogus' WHERE id = $1", sid)
	pgCode(t, err, "23514")
	_, err = pool.Exec(ctx, "UPDATE session SET front = 'web' WHERE id = $1", sid)
	require.NoError(t, err)

	pgCode(t, insertTicket(ctx, pool, f.OrgID, "web", nil, nil, f.MemberID, f.InstanceID), "23514")
	pgCode(t, insertTicket(ctx, pool, f.OrgID, "observe", &sid, &f.ResourceID, f.MemberID, f.InstanceID), "23514")
	pgCode(t, insertTicket(ctx, pool, f.OrgID, "observe", nil, nil, f.MemberID, f.InstanceID), "23514")
	pgCode(t, insertTicket(ctx, pool, f.OrgID, "bogus", nil, &f.ResourceID, f.MemberID, f.InstanceID), "23514")
	require.NoError(t, insertTicket(ctx, pool, f.OrgID, "web", nil, &f.ResourceID, f.MemberID, f.InstanceID))
	require.NoError(t, insertTicket(ctx, pool, f.OrgID, "observe", &sid, nil, f.OwnerID, f.InstanceID))

	_, err = pool.Exec(ctx, `INSERT INTO server_instance (id, hostname, internal_url) VALUES ($1, 'core-1', 'https://core-1:8444')`, uuid.New())
	require.NoError(t, err)

	sqlDB := stdlib.OpenDBFromPool(pool)
	defer func() { _ = sqlDB.Close() }()
	p, err := goose.NewProvider(goose.DialectPostgres, sqlDB, migrations.FS)
	require.NoError(t, err)
	exists := func(table string) (ok bool) {
		require.NoError(t, pool.QueryRow(ctx, "SELECT to_regclass($1) IS NOT NULL", table).Scan(&ok))
		return ok
	}
	_, err = p.DownTo(ctx, 164)
	require.NoError(t, err)
	assert.False(t, exists("server_instance") || exists("stream_ticket"))
	_, err = pool.Exec(ctx, "SELECT front FROM session")
	pgCode(t, err, "42703")
	_, err = p.Up(ctx)
	require.NoError(t, err)
	assert.True(t, exists("server_instance") && exists("stream_ticket"))
}

// TestStreamTicketCrossOrgRejected: every reference of a stream ticket stays
// inside the ticket's organization — a session, gateway instance or resource
// of another organization, or a user who is not a member, is a foreign key
// violation (23503); deleting the session or instance deletes the ticket.
func TestStreamTicketCrossOrgRejected(t *testing.T) {
	pool, _ := dbtest.New(t)
	ctx := context.Background()
	a, b := dbtest.Seed(t, pool), dbtest.Seed(t, pool)
	sidA, sidB := a.StartSession(t, pool, a.MemberID), b.StartSession(t, pool, b.MemberID)

	pgCode(t, insertTicket(ctx, pool, a.OrgID, "observe", &sidB, nil, a.OwnerID, a.InstanceID), "23503")
	pgCode(t, insertTicket(ctx, pool, a.OrgID, "observe", &sidA, nil, a.OwnerID, b.InstanceID), "23503")
	pgCode(t, insertTicket(ctx, pool, a.OrgID, "web", nil, &b.ResourceID, a.MemberID, a.InstanceID), "23503")
	pgCode(t, insertTicket(ctx, pool, a.OrgID, "web", nil, &a.ResourceID, b.MemberID, a.InstanceID), "23503")

	require.NoError(t, insertTicket(ctx, pool, a.OrgID, "observe", &sidA, nil, a.OwnerID, a.InstanceID))
	require.NoError(t, insertTicket(ctx, pool, a.OrgID, "web", nil, &a.ResourceID, a.MemberID, a.OtherInstanceID))
	count := func() (n int) {
		require.NoError(t, pool.QueryRow(ctx, "SELECT count(*) FROM stream_ticket WHERE org_id = $1", a.OrgID).Scan(&n))
		return n
	}
	_, err := pool.Exec(ctx, "DELETE FROM session WHERE id = $1", sidA)
	require.NoError(t, err)
	assert.Equal(t, 1, count(), "the observe ticket goes with its session")
	_, err = pool.Exec(ctx, "DELETE FROM gateway_instance WHERE id = $1", a.OtherInstanceID)
	require.NoError(t, err)
	assert.Zero(t, count(), "the web ticket goes with its instance")
}
