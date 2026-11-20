package app

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/access"
	"github.com/werbot/werbot/internal/platform/db/dbtest"
)

// The access adapters read the organization settings and queue emails on the River client of newServices.
func TestAccessAdapters(t *testing.T) {
	s := newStage4(t, slog.New(slog.DiscardHandler))
	f := dbtest.Seed(t, s.pool)
	ctx := context.Background()

	set, err := accessSettings{s.d}.AccessSettings(ctx, f.OrgID)
	require.NoError(t, err)
	assert.Equal(t, access.Settings{GrantDefaultTTL: 8 * time.Hour, GrantMaxTTL: 14 * 24 * time.Hour,
		RequestPendingTTL: time.Hour, CommandApprovalTTL: 5 * time.Minute}, set)

	f.ApprovalPolicy(t, s.pool, "prod", access.ModeManual, `{"resource_ids": ["`+f.ResourceID.String()+`"]}`, `{"all_users": true}`, `{}`)
	a, err := s.d.Access.CreateRequest(ctx, f.Principal(f.MemberID), access.CreateRequestInput{ResourceID: &f.ResourceID, Reason: "x"})
	require.NoError(t, err)
	assert.Equal(t, 8*time.Hour, a.Duration, "grant_default_ttl of the organization")
	var n int
	require.NoError(t, s.pool.QueryRow(ctx,
		"SELECT count(*) FROM river_job WHERE kind = 'send_email' AND args->>'template' = 'access_request'").Scan(&n))
	assert.Equal(t, 2, n, "owner and admin")
}
