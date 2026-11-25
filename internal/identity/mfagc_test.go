package identity

import (
	"context"
	"testing"

	"github.com/riverqueue/river"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/platform/db/dbtest"
	"github.com/werbot/werbot/internal/platform/jobs"
)

// TestMFAGC: challenges and verifications older than 24h are deleted, fresh
// ones stay.
func TestMFAGC(t *testing.T) {
	pool, _ := dbtest.New(t)
	ctx := context.Background()
	f := dbtest.Seed(t, pool)
	for _, age := range []string{"25 hours", "23 hours"} {
		_, err := pool.Exec(ctx, `INSERT INTO mfa_challenge (org_id, user_id, code, binding, strength, expires_at, created_at)
			VALUES ($1, $2, 'BCDF-GHJK', 'key:SHA256:x', 'any', now(), now() - $3::interval)`, f.OrgID, f.MemberID, age)
		require.NoError(t, err)
		_, err = pool.Exec(ctx, `INSERT INTO mfa_verification (org_id, user_id, binding, method, verified_at)
			VALUES ($1, $2, 'key:SHA256:x', 'totp', now() - $3::interval)`, f.OrgID, f.MemberID, age)
		require.NoError(t, err)
	}

	n, err := MFAGC(ctx, pool, MFAGCOlderThan)
	require.NoError(t, err)
	assert.Equal(t, int64(2), n)
	for _, q := range []string{
		"SELECT count(*), min(created_at) > now() - interval '24 hours' FROM mfa_challenge",
		"SELECT count(*), min(verified_at) > now() - interval '24 hours' FROM mfa_verification",
	} {
		var left int
		var fresh bool
		require.NoError(t, pool.QueryRow(ctx, q).Scan(&left, &fresh))
		assert.Equal(t, 1, left, q)
		assert.True(t, fresh, q)
	}

	assert.Equal(t, "mfa.gc", MFAGCArgs{}.Kind())
	assert.Equal(t, jobs.QueueMaintenance, MFAGCArgs{}.InsertOpts().Queue)
	assert.Len(t, RegisterMFAJobs(river.NewWorkers(), pool), 1)
}
