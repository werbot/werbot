package app

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
	"github.com/werbot/werbot/gen/werbot/v1/werbotv1connect"
	"github.com/werbot/werbot/internal/access"
	"github.com/werbot/werbot/internal/platform/db/dbtest"
	"github.com/werbot/werbot/internal/policy"
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

// The policy adapters put the active grants and locks of access into the policy snapshot.
func TestPolicySnapshotGrantsAndLocks(t *testing.T) {
	s := newStage4(t, slog.New(slog.DiscardHandler))
	f := dbtest.Seed(t, s.pool)
	ctx := context.Background()
	g := f.Grant(t, s.pool, f.MemberID, f.ResourceID, time.Hour)
	f.Grant(t, s.pool, f.MemberID, f.ResourceID, -time.Hour) // expired: not in the snapshot
	l := f.Lock(t, s.pool, "user", &f.MemberID)
	var exp time.Time
	require.NoError(t, s.pool.QueryRow(ctx, "SELECT expires_at FROM access_grant WHERE id = $1", g).Scan(&exp))

	snap, err := s.d.Policy.Snapshot(ctx, f.OrgID)
	require.NoError(t, err)
	assert.Equal(t, []policy.SnapshotGrant{{ID: g, UserID: f.MemberID, ResourceID: f.ResourceID, ExpiresUnix: exp.Unix()}}, snap.Grants)
	assert.Equal(t, []policy.SnapshotLock{{ID: l, TargetType: "user", TargetID: &f.MemberID}}, snap.Locks)

	ps, text, err := policy.Compile(snap)
	require.NoError(t, err)
	assert.Contains(t, text, `@id("lock:`+l.String()+`")`)
	d := ps.Authorize(policy.Request{UserID: f.MemberID, ResourceID: f.ResourceID, Action: "ssh.connect", IP: netip.MustParseAddr("10.0.0.1")})
	assert.Equal(t, []string{"lock:" + l.String()}, d.Reasons)
}

// mount serves the stage-10 services; without a principal each answers Unauthenticated, not Unimplemented.
func TestMountJITServices(t *testing.T) {
	s := newStage4(t, slog.New(slog.DiscardHandler))
	mux := http.NewServeMux()
	s.d.mount(mux)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	ctx := context.Background()

	_, err := werbotv1connect.NewApprovalPolicyServiceClient(srv.Client(), srv.URL).
		ListApprovalPolicies(ctx, connect.NewRequest(&werbotv1.ListApprovalPoliciesRequest{}))
	assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
	_, err = werbotv1connect.NewAccessRequestServiceClient(srv.Client(), srv.URL).
		ListRequests(ctx, connect.NewRequest(&werbotv1.ListRequestsRequest{}))
	assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
	_, err = werbotv1connect.NewLockServiceClient(srv.Client(), srv.URL).ListLocks(ctx, connect.NewRequest(&werbotv1.ListLocksRequest{}))
	assert.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}
