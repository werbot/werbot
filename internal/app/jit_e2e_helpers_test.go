package app

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	gossh "golang.org/x/crypto/ssh"

	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
	"github.com/werbot/werbot/gen/werbot/v1/werbotv1connect"
	"github.com/werbot/werbot/internal/platform/db/dbtest"
	"github.com/werbot/werbot/internal/platform/principal"
)

// jitClients are the Connect clients of one signed-in user (session cookie).
type jitClients struct {
	req      werbotv1connect.AccessRequestServiceClient
	locks    werbotv1connect.LockServiceClient
	pols     werbotv1connect.ApprovalPolicyServiceClient
	acc      werbotv1connect.AccessServiceClient
	sessions werbotv1connect.SessionServiceClient
	keys     werbotv1connect.SSHKeyServiceClient
}

// signIn logs in to the server at api as email with the owner password, in a new cookie jar.
func signIn(t *testing.T, api, email string) jitClients {
	t.Helper()
	hc, _ := jarClient(t, http.DefaultTransport)
	_, err := werbotv1connect.NewAuthServiceClient(hc, api).Login(context.Background(),
		connect.NewRequest(&werbotv1.LoginRequest{Email: email, Password: ownerPassword}))
	require.NoError(t, err)
	return jitClients{
		req: werbotv1connect.NewAccessRequestServiceClient(hc, api), locks: werbotv1connect.NewLockServiceClient(hc, api),
		pols: werbotv1connect.NewApprovalPolicyServiceClient(hc, api), acc: werbotv1connect.NewAccessServiceClient(hc, api),
		sessions: werbotv1connect.NewSessionServiceClient(hc, api), keys: werbotv1connect.NewSSHKeyServiceClient(hc, api),
	}
}

// addMember adds login as a member of alice's organization with alice's
// password, signs it in and registers a new SSH key for it.
func (s *stack) addMember(t *testing.T, login string) (gossh.Signer, jitClients, uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	var org uuid.UUID
	require.NoError(t, s.pool.QueryRow(ctx, "SELECT org_id FROM membership WHERE user_id = $1", s.alice).Scan(&org))
	id := dbtest.User(t, s.pool, org, principal.RoleMember)
	_, err := s.pool.Exec(ctx, `UPDATE users SET login = $2, email = $2 || '@example.com',
		password_hash = (SELECT password_hash FROM users WHERE login = 'alice') WHERE id = $1`, id, login)
	require.NoError(t, err)
	c := signIn(t, s.url, login+"@example.com")
	key := newSigner(t)
	_, err = c.keys.AddSSHKey(ctx, connect.NewRequest(&werbotv1.AddSSHKeyRequest{
		PublicKey: string(gossh.MarshalAuthorizedKey(key.PublicKey()))}))
	require.NoError(t, err)
	return key, c, id
}

// execKey runs cmd as user with key without a PTY and returns stdout+stderr
// and the exit code. A connection or channel failure returns its text and -1,
// so execKey is safe in an Eventually condition.
func (s *stack) execKey(t *testing.T, key gossh.Signer, addr, user, cmd string) (out string, code int) {
	t.Helper()
	c, _, err := s.dial(addr, user, key)
	if err != nil {
		return err.Error(), -1
	}
	defer c.Close() //nolint:errcheck // test client
	// a hung session returns -1 instead of waiting for the global timeout
	hung := time.AfterFunc(15*time.Second, func() { _ = c.Close() })
	defer hung.Stop()
	sess, err := c.NewSession()
	if err != nil {
		return err.Error(), -1
	}
	defer sess.Close() //nolint:errcheck // closed by the server after exit
	b, err := sess.CombinedOutput(cmd)
	var ee *gossh.ExitError
	switch {
	case err == nil:
		return string(b), 0
	case errors.As(err, &ee):
		return string(b), ee.ExitStatus()
	}
	return string(b) + err.Error(), -1
}

// shell opens a PTY shell as user with key and waits until it answers; wait
// returns when the gateway closes it.
func (s *stack) shell(t *testing.T, key gossh.Signer, user string) (wait func() error) {
	t.Helper()
	c, _, err := s.dial(s.gwSSH, user, key)
	require.NoError(t, err)
	t.Cleanup(func() { _ = c.Close() })
	sess, err := c.NewSession()
	require.NoError(t, err)
	require.NoError(t, sess.RequestPty("xterm", 40, 120, gossh.TerminalModes{}))
	stdin, err := sess.StdinPipe()
	require.NoError(t, err)
	stdout, err := sess.StdoutPipe()
	require.NoError(t, err)
	require.NoError(t, sess.Shell())
	_, err = io.WriteString(stdin, "echo ready\r")
	require.NoError(t, err)
	readUntil(t, stdout, "ready")
	return sess.Wait
}

// sessionEnd waits up to 10s for the latest session of user on resource to
// end and returns its end time and reason.
func sessionEnd(t *testing.T, pool *pgxpool.Pool, user, resource uuid.UUID) (endedAt time.Time, reason string) {
	t.Helper()
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		var at *time.Time
		err := pool.QueryRow(context.Background(), `SELECT ended_at, coalesce(end_reason, '') FROM session
			WHERE user_id = $1 AND resource_id = $2 ORDER BY started_at DESC LIMIT 1`, user, resource).Scan(&at, &reason)
		if err == pgx.ErrNoRows {
			c.Errorf("no session")
			return
		}
		require.NoError(c, err)
		require.NotNil(c, at, "session still open")
		endedAt = *at
	}, 10*time.Second, 50*time.Millisecond)
	return endedAt, reason
}

// closedWithin waits for wait (a shell) to return within d.
func closedWithin(t *testing.T, wait func() error, d time.Duration) {
	t.Helper()
	done := make(chan struct{})
	go func() { _ = wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(d):
		t.Fatalf("shell still open after %v", d)
	}
}

// requestPolicy creates a manual approval policy for every user on the resources, approved by owners and admins.
func requestPolicy(t *testing.T, c jitClients, name string, breakGlass bool, resources ...uuid.UUID) {
	t.Helper()
	ids := make([]string, len(resources))
	for i, r := range resources {
		ids[i] = r.String()
	}
	_, err := c.pols.CreateApprovalPolicy(context.Background(), connect.NewRequest(&werbotv1.CreateApprovalPolicyRequest{
		Name: name, ResourceSelector: &werbotv1.ResourceSelector{ResourceIds: ids},
		Requesters: &werbotv1.Subjects{AllUsers: true}, Mode: "manual", BreakGlass: breakGlass}))
	require.NoError(t, err)
}

// activeGrant is the only active grant of user.
func activeGrant(t *testing.T, c jitClients, user uuid.UUID) *werbotv1.Grant {
	t.Helper()
	res, err := c.req.ListGrants(context.Background(), connect.NewRequest(&werbotv1.ListGrantsRequest{
		UserId: new(user.String()), Active: true}))
	require.NoError(t, err)
	require.Len(t, res.Msg.GetGrants(), 1)
	return res.Msg.GetGrants()[0]
}

// denied tells whether user with key is refused on the resource.
func (s *stack) denied(t *testing.T, key gossh.Signer, user string) bool {
	out, code := s.execKey(t, key, s.gwSSH, user, "echo ok")
	return strings.TrimSpace(out) == "Access denied" && code == 1
}

// allowed tells whether user with key runs a command on the resource.
func (s *stack) allowed(t *testing.T, key gossh.Signer, user string) bool {
	out, code := s.execKey(t, key, s.gwSSH, user, "echo ok")
	return out == "ok\n" && code == 0
}
