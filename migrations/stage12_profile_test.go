package migrations_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/platform/db/dbtest"
	"github.com/werbot/werbot/migrations"
)

// TestCLISessionAgentFK: an agent profile's cli_session names an agent of its
// own organization; deleting the agent deletes the row (never a session of a
// person); 00164 goes down and up.
func TestCLISessionAgentFK(t *testing.T) {
	pool, _ := dbtest.New(t)
	ctx := context.Background()
	f := dbtest.Seed(t, pool)
	other := dbtest.Seed(t, pool)
	agent := dbtest.Agent(t, pool, f.OrgID, f.MemberID, "ops")
	foreign := dbtest.Agent(t, pool, other.OrgID, other.MemberID, "ops")

	const q = `INSERT INTO cli_session (org_id, user_id, client_id, refresh_hash, agent_id, expires_at, absolute_expires_at)
		VALUES ($1, $2, 'werbot-cli', $3, $4, now() + interval '1 hour', now() + interval '1 hour') RETURNING id`
	_, err := pool.Exec(ctx, q, f.OrgID, f.MemberID, []byte(uuid.NewString()), foreign)
	pgCode(t, err, "23503")

	var id uuid.UUID
	require.NoError(t, pool.QueryRow(ctx, q, f.OrgID, f.MemberID, []byte(uuid.NewString()), agent).Scan(&id))
	_, err = pool.Exec(ctx, "DELETE FROM agent WHERE id = $1", agent)
	require.NoError(t, err)
	var n int
	require.NoError(t, pool.QueryRow(ctx, "SELECT count(*) FROM cli_session WHERE id = $1", id).Scan(&n))
	require.Zero(t, n, "the profile goes with its agent")

	sqlDB := stdlib.OpenDBFromPool(pool)
	defer func() { _ = sqlDB.Close() }()
	p, err := goose.NewProvider(goose.DialectPostgres, sqlDB, migrations.FS)
	require.NoError(t, err)
	_, err = p.DownTo(ctx, 163)
	require.NoError(t, err)
	_, err = p.Up(ctx)
	require.NoError(t, err)
}
