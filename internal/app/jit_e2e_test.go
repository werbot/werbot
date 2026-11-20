package app

import (
	"context"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	gossh "golang.org/x/crypto/ssh"
	"google.golang.org/protobuf/types/known/durationpb"

	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
)

// grantFor has bob ask for srv for d and alice approve it; it waits until the gateway lets bob in.
func (s *stack) grantFor(t *testing.T, alice, bob jitClients, key gossh.Signer, srv uuid.UUID, d time.Duration) {
	t.Helper()
	ctx := context.Background()
	res, err := bob.req.CreateRequest(ctx, connect.NewRequest(&werbotv1.CreateRequestRequest{
		ResourceId: srv.String(), Duration: durationpb.New(d), Reason: "deploy"}))
	require.NoError(t, err)
	require.Equal(t, "pending", res.Msg.GetRequest().GetStatus())
	assert.True(t, s.denied(t, key, "bob+srv"), "a pending request gives no access")
	_, err = alice.req.Approve(ctx, connect.NewRequest(&werbotv1.ApproveRequest{Id: res.Msg.GetRequest().GetId()}))
	require.NoError(t, err)
	require.Eventually(t, func() bool { return s.allowed(t, key, "bob+srv") }, 3*time.Second, 50*time.Millisecond)
}

// jitStack is a stack with resource srv (bound to alice), a manual approval
// policy on it and the member bob, whom the gateway already refuses.
func jitStack(t *testing.T) (s *stack, srv uuid.UUID, alice, bob jitClients, key gossh.Signer, bobID uuid.UUID) {
	t.Helper()
	s = startStack(t)
	srv = s.addResource(t, "srv", "ssh")
	alice = signIn(t, s.url, "alice@example.com")
	requestPolicy(t, alice, "srv-access", false, srv)
	key, bob, bobID = s.addMember(t, "bob")
	require.Eventually(t, func() bool { return s.denied(t, key, "bob+srv") }, settle, 100*time.Millisecond)
	return s, srv, alice, bob, key, bobID
}

func TestE2ERequestApproveSSHExpiry(t *testing.T) {
	s, srv, alice, bob, key, bobID := jitStack(t)
	s.grantFor(t, alice, bob, key, srv, 6*time.Second)
	wait := s.shell(t, key, "bob+srv")
	exp := activeGrant(t, alice, bobID).GetExpiresAt().AsTime()

	closedWithin(t, wait, time.Until(exp)+2*time.Second)
	endedAt, reason := sessionEnd(t, s.pool, bobID, srv)
	assert.Equal(t, "access_revoked", reason)
	// Cedar compares whole seconds: the grant ends at the second of expires_at, never later.
	assert.False(t, endedAt.Before(exp.Truncate(time.Second)), "ended %v before the grant's second %v", endedAt, exp)
	assert.LessOrEqual(t, endedAt.Sub(exp), time.Second, "the session ends at most 1s after the grant expires")
	assert.True(t, s.denied(t, key, "bob+srv"), "an expired grant gives no access")
}

func TestE2ERevokeGrantCutsSession(t *testing.T) {
	s, srv, alice, bob, key, bobID := jitStack(t)
	s.grantFor(t, alice, bob, key, srv, time.Hour)
	wait := s.shell(t, key, "bob+srv")
	g := activeGrant(t, alice, bobID)

	called := time.Now()
	_, err := alice.req.RevokeGrant(context.Background(), connect.NewRequest(&werbotv1.RevokeGrantRequest{Id: g.GetId(), Reason: "done"}))
	require.NoError(t, err)
	closedWithin(t, wait, 5*time.Second)
	endedAt, reason := sessionEnd(t, s.pool, bobID, srv)
	assert.Equal(t, "access_revoked", reason)
	assert.LessOrEqual(t, endedAt.Sub(called), 2*time.Second, "revoke ends the session within 2s")
	assert.True(t, s.denied(t, key, "bob+srv"))
}

// lock creates a lock on target and returns its id and the time of the call.
func lock(t *testing.T, c jitClients, typ string, target uuid.UUID) (string, time.Time) {
	t.Helper()
	called := time.Now()
	res, err := c.locks.CreateLock(context.Background(), connect.NewRequest(&werbotv1.CreateLockRequest{
		TargetType: typ, TargetId: target.String(), Reason: "incident"}))
	require.NoError(t, err)
	return res.Msg.GetLock().GetId(), called
}

func TestE2ELockCutsSession(t *testing.T) {
	s, srv, alice, _, key, bobID := jitStack(t)
	srv2 := s.addResource(t, "srv2", "ssh")
	ctx := context.Background()
	for _, r := range []uuid.UUID{srv, srv2} {
		_, err := alice.acc.CreateBinding(ctx, connect.NewRequest(&werbotv1.CreateBindingRequest{
			SubjectType: "user", SubjectId: bobID.String(), ResourceId: r.String()}))
		require.NoError(t, err)
	}
	require.Eventually(t, func() bool { return s.allowed(t, key, "bob+srv") && s.allowed(t, key, "bob+srv2") }, settle, 100*time.Millisecond)
	w1, w2 := s.shell(t, key, "bob+srv"), s.shell(t, key, "bob+srv2")

	id, called := lock(t, alice, "user", bobID)
	closedWithin(t, w1, 5*time.Second)
	closedWithin(t, w2, 5*time.Second)
	for _, r := range []uuid.UUID{srv, srv2} {
		endedAt, reason := sessionEnd(t, s.pool, bobID, r)
		assert.Equal(t, "access_revoked", reason)
		assert.LessOrEqual(t, endedAt.Sub(called), 2*time.Second, "a user lock ends every session of the user within 2s")
	}
	assert.True(t, s.denied(t, key, "bob+srv"), "a lock beats the binding")
	_, err := alice.locks.DeleteLock(ctx, connect.NewRequest(&werbotv1.DeleteLockRequest{Id: id}))
	require.NoError(t, err)
	require.Eventually(t, func() bool { return s.allowed(t, key, "bob+srv") }, 3*time.Second, 50*time.Millisecond)

	wa := s.shell(t, s.key, "alice+srv")
	_, called = lock(t, alice, "resource", srv)
	closedWithin(t, wa, 5*time.Second)
	endedAt, reason := sessionEnd(t, s.pool, uuid.MustParse(s.alice), srv)
	assert.Equal(t, "access_revoked", reason)
	assert.LessOrEqual(t, endedAt.Sub(called), 2*time.Second, "a resource lock ends its sessions within 2s")
	assert.True(t, s.allowed(t, key, "bob+srv2"), "a resource lock leaves other resources alone")
}

func TestE2ECLISessionLockCutsConnection(t *testing.T) {
	s, srv := cliStack(t)
	home, p := s.cliLogin(t, false)
	s.certReady(t, home, p)
	signer, err := p.Signer(context.Background(), home)
	require.NoError(t, err)
	wait := s.shell(t, signer, "alice+srv")
	list, err := s.browser(t).sessions.ListCLISessions(context.Background(), connect.NewRequest(&werbotv1.ListCLISessionsRequest{}))
	require.NoError(t, err)
	require.Len(t, list.Msg.GetSessions(), 1)

	_, called := lock(t, signIn(t, s.url, "alice@example.com"), "cli_session", uuid.MustParse(list.Msg.GetSessions()[0].GetId()))
	closedWithin(t, wait, 5*time.Second)
	endedAt, reason := sessionEnd(t, s.pool, uuid.MustParse(s.alice), srv)
	assert.Equal(t, "access_revoked", reason)
	assert.LessOrEqual(t, endedAt.Sub(called), 2*time.Second, "a cli_session lock ends the open connections of that session")
	assert.True(t, s.denied(t, signer, "alice+srv"), "the locked session's still valid certificate opens nothing")
	assert.True(t, s.allowed(t, s.key, "alice+srv"), "alice's own key is not that CLI session")
}

func TestE2EBreakGlassIsRecorded(t *testing.T) {
	s := startStack(t)
	srv := s.addResource(t, "srv", "ssh")
	alice := signIn(t, s.url, "alice@example.com")
	requestPolicy(t, alice, "srv-break-glass", true, srv)
	key, bob, bobID := s.addMember(t, "bob")
	ctx := context.Background()

	res, err := bob.req.CreateRequest(ctx, connect.NewRequest(&werbotv1.CreateRequestRequest{
		ResourceId: srv.String(), Reason: "incident", BreakGlass: true}))
	require.NoError(t, err)
	assert.Equal(t, "approved", res.Msg.GetRequest().GetStatus(), "break-glass needs no approver")
	require.Eventually(t, func() bool { return s.allowed(t, key, "bob+srv") }, settle, 50*time.Millisecond)

	var sid string
	var mark bool
	require.NoError(t, s.pool.QueryRow(ctx, `SELECT data->>'session_id', (data->>'break_glass')::bool FROM audit_event
		WHERE action = 'session.start' AND actor_user_id = $1 ORDER BY created_at DESC LIMIT 1`, bobID).Scan(&sid, &mark))
	assert.True(t, mark, "session.start of a break-glass session is marked")
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		v, err := alice.sessions.VerifyRecording(ctx, connect.NewRequest(&werbotv1.VerifyRecordingRequest{SessionId: sid}))
		require.NoError(c, err)
		assert.True(c, v.Msg.GetVerified(), v.Msg.GetReason())
	}, 15*time.Second, 100*time.Millisecond)
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		var sent int
		require.NoError(c, s.pool.QueryRow(ctx, `SELECT count(*) FROM river_job WHERE kind = 'send_email'
			AND args->>'template' = 'break_glass' AND args->>'to' = 'alice@example.com' AND state = 'completed'`).Scan(&sent))
		assert.Equal(c, 1, sent, "the owner got the break-glass email (log mailer in development)")
	}, 15*time.Second, 100*time.Millisecond)

	g := activeGrant(t, alice, bobID)
	require.True(t, g.GetBreakGlass())
	require.True(t, g.GetNeedsReview())
	_, err = bob.req.ReviewBreakGlass(ctx, connect.NewRequest(&werbotv1.ReviewBreakGlassRequest{GrantId: g.GetId()}))
	require.Error(t, err, "the grant owner never reviews own break-glass")
	_, err = alice.req.ReviewBreakGlass(ctx, connect.NewRequest(&werbotv1.ReviewBreakGlassRequest{GrantId: g.GetId(), Comment: "ok"}))
	require.NoError(t, err)
	assert.False(t, activeGrant(t, alice, bobID).GetNeedsReview())
}
