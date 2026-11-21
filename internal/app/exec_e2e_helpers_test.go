package app

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	gossh "golang.org/x/crypto/ssh"

	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
	"github.com/werbot/werbot/gen/werbot/v1/werbotv1connect"
	"github.com/werbot/werbot/internal/platform/db/dbtest"
	"github.com/werbot/werbot/internal/platform/principal"
)

// execSplit runs cmd as alice on the resource srv without a PTY and returns
// stdout, stderr and the exit code. A connection or channel failure returns
// its text in stderr and -1. It waits up to 60s, longer than the shortest
// approval wait.
func (s *stack) execSplit(t *testing.T, cmd string) (stdout, stderr string, code int) {
	t.Helper()
	c, _, err := s.dial(s.gwSSH, "alice+srv", s.key)
	if err != nil {
		return "", err.Error(), -1
	}
	defer c.Close()                                                  //nolint:errcheck // test client
	hung := time.AfterFunc(60*time.Second, func() { _ = c.Close() }) // a hung session returns -1
	defer hung.Stop()
	sess, err := c.NewSession()
	if err != nil {
		return "", err.Error(), -1
	}
	defer sess.Close() //nolint:errcheck // closed by the server after exit
	var out, errOut bytes.Buffer
	sess.Stdout, sess.Stderr = &out, &errOut
	err = sess.Run(cmd)
	var ee *gossh.ExitError
	switch {
	case err == nil:
		return out.String(), errOut.String(), 0
	case errors.As(err, &ee):
		return out.String(), errOut.String(), ee.ExitStatus()
	}
	return out.String(), errOut.String() + err.Error(), -1
}

// addAdmin adds login as an admin of alice's organization with alice's
// password and signs it in: the approver of alice's commands, since nobody
// approves their own.
func (s *stack) addAdmin(t *testing.T, login string) jitClients {
	t.Helper()
	ctx := context.Background()
	var org uuid.UUID
	require.NoError(t, s.pool.QueryRow(ctx, "SELECT org_id FROM membership WHERE user_id = $1", s.alice).Scan(&org))
	id := dbtest.User(t, s.pool, org, principal.RoleAdmin)
	_, err := s.pool.Exec(ctx, `UPDATE users SET login = $2, email = $2 || '@example.com',
		password_hash = (SELECT password_hash FROM users WHERE login = 'alice') WHERE id = $1`, id, login)
	require.NoError(t, err)
	return signIn(t, s.url, login+"@example.com")
}

// aliceHTTP is an HTTP client signed in as alice, in a new cookie jar.
func (s *stack) aliceHTTP(t *testing.T) *http.Client {
	t.Helper()
	hc, _ := jarClient(t, http.DefaultTransport)
	_, err := werbotv1connect.NewAuthServiceClient(hc, s.url).Login(context.Background(),
		connect.NewRequest(&werbotv1.LoginRequest{Email: "alice@example.com", Password: ownerPassword}))
	require.NoError(t, err)
	return hc
}

// commandRule creates a command rule for every user on srv.
func (s *stack) commandRule(t *testing.T, srv uuid.UUID, effect, message string, classes ...string) {
	t.Helper()
	_, err := s.api.pol.CreateCommandRule(context.Background(), connect.NewRequest(&werbotv1.CreateCommandRuleRequest{
		Name: effect + "-" + classes[0], Effect: effect, Message: message, Classes: classes,
		Subjects: &werbotv1.Subjects{AllUsers: true}, ResourceSelector: &werbotv1.ResourceSelector{ResourceIds: []string{srv.String()}}}))
	require.NoError(t, err)
}

const denyMessage = "Destructive commands are blocked"

// denyDestructive creates a deny rule with denyMessage for destructive commands
// on srv and waits until the gateway refuses one. A snapshot holds every rule,
// so the rules created before it have reached the gateway too.
func (s *stack) denyDestructive(t *testing.T, srv uuid.UUID) {
	t.Helper()
	s.commandRule(t, srv, "deny", denyMessage, "destructive")
	require.Eventually(t, func() bool {
		_, stderr, code := s.execSplit(t, "rm -rf /")
		return stderr == "Command denied: "+denyMessage+"\r\n" && code == 1
	}, 5*time.Second, 100*time.Millisecond)
}

// approvalStack is a stack with resource srv, where "hostname" is a write
// command that needs an approval, and the admin carol, who may approve it.
func approvalStack(t *testing.T) (s *stack, srv uuid.UUID, carol jitClients) {
	t.Helper()
	s = startStack(t)
	srv = s.addResource(t, "srv", "ssh")
	require.Eventually(t, func() bool { return s.allowed(t, s.key, "alice+srv") }, settle, 100*time.Millisecond)
	carol = s.addAdmin(t, "carol")
	_, err := s.api.pol.CreateClassRule(context.Background(), connect.NewRequest(&werbotv1.CreateClassRuleRequest{
		Pattern: "hostname", Classes: []string{"write"}}))
	require.NoError(t, err)
	s.commandRule(t, srv, "approval", "", "write")
	requestPolicy(t, signIn(t, s.url, "alice@example.com"), "srv-commands", false, srv)
	s.denyDestructive(t, srv)
	return s, srv, carol
}

// pendingCommand waits up to 5s for the only pending command approval and returns its id.
func (s *stack) pendingCommand(t *testing.T) string {
	t.Helper()
	var id string
	require.Eventually(t, func() bool {
		err := s.pool.QueryRow(context.Background(),
			"SELECT id::text FROM approval WHERE kind = 'command' AND status = 'pending'").Scan(&id)
		return err == nil
	}, 5*time.Second, 50*time.Millisecond)
	return id
}
