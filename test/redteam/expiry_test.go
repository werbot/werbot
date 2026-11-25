//go:build redteam

package redteam

import (
	"context"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/durationpb"

	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
	"github.com/werbot/werbot/test/cluster"
)

// Values of the grant scenarios (12 §12 item 3).
const (
	grantedResource = "target2"        // not bound to alice: a grant is her only access
	grantEndMax     = 2 * time.Second  // a session ends this long after its grant expires or is revoked, always
	grantDuration   = time.Minute      // grant length of the expiry scenario (plan Task 2)
	endWait         = 15 * time.Second // GetSession polling and the session end report
)

// autoGrants lets every person request grantedResource and approves at once; Cleanup deletes the policy.
func autoGrants(t *testing.T) {
	t.Helper()
	res, err := st.Edge.ApprovalPolicies.CreateApprovalPolicy(t.Context(), connect.NewRequest(&werbotv1.CreateApprovalPolicyRequest{
		Name: "rt-auto-target2", Mode: "auto", MaxDuration: durationpb.New(time.Hour),
		ResourceSelector: &werbotv1.ResourceSelector{ResourceIds: []string{st.Resources[grantedResource].String()}},
		Requesters:       &werbotv1.Subjects{AllUsers: true},
	}))
	require.NoError(t, err)
	t.Cleanup(func() {
		_, err := st.Edge.ApprovalPolicies.DeleteApprovalPolicy(context.Background(),
			connect.NewRequest(&werbotv1.DeleteApprovalPolicyRequest{Id: res.Msg.GetApprovalPolicy().GetId()}))
		assert.NoError(t, err, "delete approval policy")
	})
}

// requestGrant asks for grantedResource as alice for d, expects the auto
// approval and returns the grant id; Cleanup revokes a grant still active.
func requestGrant(t *testing.T, d time.Duration) uuid.UUID {
	t.Helper()
	req, err := st.Edge.AccessRequests.CreateRequest(t.Context(), connect.NewRequest(&werbotv1.CreateRequestRequest{
		ResourceId: st.Resources[grantedResource].String(), Duration: durationpb.New(d), Reason: "redteam",
	}))
	require.NoError(t, err)
	require.Equal(t, "approved", req.Msg.GetRequest().GetStatus(), "the auto policy approves at once")
	var id uuid.UUID
	require.NoError(t, st.DB.QueryRow(context.Background(), "SELECT id FROM access_grant WHERE approval_id = $1",
		req.Msg.GetRequest().GetId()).Scan(&id))
	t.Cleanup(func() {
		_, _ = st.Edge.AccessRequests.RevokeGrant(context.Background(), connect.NewRequest(&werbotv1.RevokeGrantRequest{Id: id.String(), Reason: "cleanup"}))
	})
	assert.Equal(t, 1, audits(t, "access.grant.create", id))
	return id
}

// grantedShell opens a shell on grantedResource once the grant reached the pool and returns its session.
func grantedShell(t *testing.T) uuid.UUID {
	t.Helper()
	c := st.Dial(t, st.Addr(t, "nginx", 2222), "alice+"+grantedResource)
	require.Eventually(t, func() bool {
		sess, err := c.NewSession()
		if err != nil {
			return false
		}
		defer sess.Close()
		out, err := sess.Output("hostname")
		return err == nil && string(out) == "target\n"
	}, cluster.BindingPropagation, 50*time.Millisecond, "the grant reaches the pool")
	sh := cluster.OpenShell(t, c)
	sh.Mark(t, "granted")
	var id uuid.UUID
	require.NoError(t, st.DB.QueryRow(context.Background(), `SELECT id FROM session WHERE resource_id = $1 AND ended_at IS NULL
		ORDER BY started_at DESC LIMIT 1`, st.Resources[grantedResource]).Scan(&id))
	return id
}

// endLag is how long after the grant's end (expires_at, or revoked_at once revoked) session ended.
func endLag(t *testing.T, session, grant uuid.UUID) time.Duration {
	t.Helper()
	var secs float64
	require.NoError(t, st.DB.QueryRow(context.Background(), `SELECT extract(epoch FROM s.ended_at - coalesce(g.revoked_at, g.expires_at))::float8
		FROM session s, access_grant g WHERE s.id = $1 AND g.id = $2`, session, grant).Scan(&secs))
	return time.Duration(secs * float64(time.Second))
}

// requireNoAccess requires the plain refusal for alice on grantedResource.
func requireNoAccess(t *testing.T) {
	t.Helper()
	out, code := cluster.Exec(t, st.Dial(t, st.Addr(t, "nginx", 2222), "alice+"+grantedResource), "hostname")
	require.Equal(t, "Access denied", strings.TrimSpace(out))
	require.Equal(t, 1, code)
}

// TestRedteamGrantExpiryBreaksSession (12 §12 item 3): a shell opened under a
// grant ends with access_revoked within grantEndMax of the grant's expiry, the
// end is audited, and the next connection is refused.
func TestRedteamGrantExpiryBreaksSession(t *testing.T) {
	begin(t)
	requireNoAccess(t)
	autoGrants(t)
	grant := requestGrant(t, grantDuration)
	id := grantedShell(t)
	var expires time.Time
	require.NoError(t, st.DB.QueryRow(context.Background(), "SELECT expires_at FROM access_grant WHERE id = $1", grant).Scan(&expires))

	st.WaitEnded(t, id, "access_revoked", time.Until(expires)+grantEndMax+endWait)
	lag := endLag(t, id, grant)
	t.Logf("grant expiry broke session in %s", lag)
	assert.LessOrEqual(t, lag, grantEndMax)
	var ends int
	require.NoError(t, st.DB.QueryRow(context.Background(), `SELECT count(*) FROM audit_event WHERE action = 'session.end'
		AND data->>'session_id' = $1 AND data->>'reason' = 'access_revoked'`, id.String()).Scan(&ends))
	assert.Equal(t, 1, ends, "session.end audit of the broken session")
	requireNoAccess(t)
}

// TestRedteamGrantRevokeBreaksSession: revoking a grant ends its shell with
// access_revoked within grantEndMax.
func TestRedteamGrantRevokeBreaksSession(t *testing.T) {
	begin(t)
	autoGrants(t)
	grant := requestGrant(t, time.Hour)
	id := grantedShell(t)

	_, err := st.Edge.AccessRequests.RevokeGrant(t.Context(), connect.NewRequest(&werbotv1.RevokeGrantRequest{Id: grant.String(), Reason: "redteam"}))
	require.NoError(t, err)
	st.WaitEnded(t, id, "access_revoked", grantEndMax+endWait)
	lag := endLag(t, id, grant)
	t.Logf("grant revoke broke session in %s", lag)
	assert.LessOrEqual(t, lag, grantEndMax)
	assert.Equal(t, 1, audits(t, "access.grant.revoke", grant))
	requireNoAccess(t)
}
