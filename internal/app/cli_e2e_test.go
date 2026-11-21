package app

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	gossh "golang.org/x/crypto/ssh"

	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
	"github.com/werbot/werbot/gen/werbot/v1/werbotv1connect"
	"github.com/werbot/werbot/internal/clientauth"
	"github.com/werbot/werbot/internal/platform/db/dbtest"
	"github.com/werbot/werbot/internal/platform/principal"
	"github.com/werbot/werbot/internal/platform/sshca"
)

// cliStack is a stack whose pool has an SSH address and a resource "srv"
// bound to alice.
func cliStack(t *testing.T) (*stack, uuid.UUID) {
	t.Helper()
	s := startStack(t)
	s.setSSHAddress(t)
	return s, s.addResource(t, "srv", "ssh")
}

// certReady waits until the profile's certificate opens srv: the resource reached the snapshot.
func (s *stack) certReady(t *testing.T, home string, p *clientauth.Profile) {
	t.Helper()
	require.Eventually(t, func() bool {
		out, err := s.certExec(t, home, p, "echo hello")
		return err == nil && out == "hello\n"
	}, settle, 100*time.Millisecond)
}

// keyReady waits until alice's SSH key opens srv: the pool has a snapshot with the resource.
func (s *stack) keyReady(t *testing.T) {
	t.Helper()
	require.Eventually(t, func() bool {
		out, code := s.exec(t, s.gwSSH, "alice+srv", "echo hello")
		return out == "hello\n" && code == 0
	}, settle, 100*time.Millisecond)
}

func TestE2ECLIDeviceLoginAndSSH(t *testing.T) {
	s, srv := cliStack(t)
	home, p := s.cliLogin(t, true)
	assert.Equal(t, "alice", p.Login)
	assert.Equal(t, s.alice, p.UserID.String())
	s.certReady(t, home, p)

	var all, alice int
	require.NoError(t, s.pool.QueryRow(context.Background(),
		"SELECT count(*), count(*) FILTER (WHERE user_id = $2) FROM session WHERE resource_id = $1", srv, s.alice).Scan(&all, &alice))
	assert.Positive(t, all)
	assert.Equal(t, all, alice, "the certificate session belongs to alice")
}

func TestE2ECLILoopbackLogin(t *testing.T) {
	s, _ := cliStack(t)
	_, p := s.cliLogin(t, false)
	list, err := werbotv1connect.NewResourceServiceClient(p.HTTPClient(), p.Server).
		ListAccessibleResources(context.Background(), connect.NewRequest(&werbotv1.ListAccessibleResourcesRequest{}))
	require.NoError(t, err)
	require.Len(t, list.Msg.GetResources(), 1)
	r := list.Msg.GetResources()[0]
	assert.Equal(t, "srv", r.GetName())
	assert.Equal(t, s.gwSSH, r.GetSshAddress())

	_, host, _ := s.dial(s.gwSSH, "alice+srv", s.key) // the host key arrives before user auth
	require.NotNil(t, host)
	assert.Equal(t, strings.TrimSpace(string(gossh.MarshalAuthorizedKey(host))), r.GetHostKey())
}

func TestE2ERevokedCLISessionRejectsNextConnection(t *testing.T) {
	s, _ := cliStack(t)
	home, p := s.cliLogin(t, false)
	s.certReady(t, home, p)
	ctx := context.Background()
	signer, err := p.Signer(ctx, home)
	require.NoError(t, err)
	a, _, err := s.dial(s.gwSSH, "alice+srv", signer)
	require.NoError(t, err)
	t.Cleanup(func() { _ = a.Close() })

	b := s.browser(t)
	list, err := b.sessions.ListCLISessions(ctx, connect.NewRequest(&werbotv1.ListCLISessionsRequest{}))
	require.NoError(t, err)
	require.Len(t, list.Msg.GetSessions(), 1)
	_, err = b.sessions.RevokeCLISession(ctx, connect.NewRequest(&werbotv1.RevokeCLISessionRequest{Id: list.Msg.GetSessions()[0].GetId()}))
	require.NoError(t, err)

	_, _, err = s.dial(s.gwSSH, "alice+srv", signer)
	require.ErrorContains(t, err, "unable to authenticate", "the certificate is still valid; its session is not")
	require.EqualError(t, Run(ctx, []string{"ls"}, env(map[string]string{"WERBOT_HOME": home}), &bytes.Buffer{}),
		clientauth.ErrLoginRequired.Error(), "the API refuses the access token of the revoked session")
	out, err := run(a, "echo still")
	require.NoError(t, err)
	assert.Equal(t, "still\n", out, "revoke leaves open connections running; a cli_session lock ends them (TestE2ECLISessionLockCutsConnection)")
}

func TestE2EOtherOrgCertRejected(t *testing.T) {
	s, _ := cliStack(t)
	s.keyReady(t)

	ctx, ids := context.Background(), s.services(t).Identity
	orgB := dbtest.Org(t, s.pool)
	userB := dbtest.User(t, s.pool, orgB, "owner")
	hash := make([]byte, 32)
	_, _ = rand.Read(hash)
	var sid uuid.UUID
	var loginB string
	require.NoError(t, s.pool.QueryRow(ctx, `INSERT INTO cli_session (org_id, user_id, client_id, refresh_hash, expires_at, absolute_expires_at)
		VALUES ($1, $2, 'werbot-cli', $3, now() + interval '1 hour', now() + interval '1 day') RETURNING id`, orgB, userB, hash).Scan(&sid))
	require.NoError(t, s.pool.QueryRow(ctx, "SELECT login FROM users WHERE id = $1", userB).Scan(&loginB))

	key := newSigner(t)
	own, err := ids.IssueUserCert(ctx, principal.Principal{OrgID: orgB, UserID: userB, CLISessionID: sid,
		AuthMethod: principal.AuthMethodOAuth}, key.PublicKey())
	require.NoError(t, err)
	caB, err := ids.UserCA(ctx, orgB)
	require.NoError(t, err)
	asAlice, err := sshca.Sign(caB, key.PublicKey(), sshca.Identity{OrgID: orgB, UserID: uuid.MustParse(s.alice),
		CLISessionID: sid, Login: "alice"}, time.Now(), sshca.UserCertTTL)
	require.NoError(t, err)

	for login, cert := range map[string]*gossh.Certificate{loginB: own, "alice": asAlice} {
		signer, err := gossh.NewCertSigner(cert, key)
		require.NoError(t, err)
		_, _, err = s.dial(s.gwSSH, login+"+srv", signer)
		require.ErrorContains(t, err, "unable to authenticate", "org B's CA signed the certificate for %s", login)
	}
}

func TestE2EFirstCertWithoutSnapshotWait(t *testing.T) {
	s, _ := cliStack(t)
	s.keyReady(t) // the pool is connected before the first login

	home, p := s.cliLogin(t, false)
	out, err := s.certExec(t, home, p, "echo first")
	require.NoError(t, err, "the first snapshot already carries the organization's user CA")
	assert.Equal(t, "first\n", out)
}

func TestE2ERefreshReuseRevokes(t *testing.T) {
	s, _ := cliStack(t)
	home, p := s.cliLogin(t, false)
	s.certReady(t, home, p)
	ctx := context.Background()
	signer, err := p.Signer(ctx, home)
	require.NoError(t, err)

	file := filepath.Join(clientauth.Dir(home, p.Server), "profile.json")
	raw, err := os.ReadFile(file) //nolint:gosec // test temp dir
	require.NoError(t, err)
	var doc map[string]any
	require.NoError(t, json.Unmarshal(raw, &doc))
	doc["access_expiry"] = time.Now().Add(-time.Minute).UTC() // the access token counts as expired: Token refreshes
	stale, err := json.Marshal(doc)
	require.NoError(t, err)

	tokenAfter := func() error { // a copy of profile.json from before the refresh, then Token
		require.NoError(t, os.WriteFile(file, stale, 0o600))
		cur, err := clientauth.Current(home, env(nil))
		require.NoError(t, err)
		_, err = cur.Token(ctx)
		return err
	}
	require.NoError(t, tokenAfter(), "the first refresh rotates")
	require.ErrorIs(t, tokenAfter(), clientauth.ErrLoginRequired, "the old refresh token again")

	var reason string
	require.NoError(t, s.pool.QueryRow(ctx, "SELECT revoke_reason FROM cli_session").Scan(&reason))
	assert.Equal(t, "refresh_reuse", reason)
	_, _, err = s.dial(s.gwSSH, "alice+srv", signer)
	require.ErrorContains(t, err, "unable to authenticate")
}

func TestE2EOpenSSHConfig(t *testing.T) {
	if _, err := exec.LookPath("ssh"); err != nil {
		t.Skip("ssh is not in PATH")
	}
	s, _ := cliStack(t)
	home, p := s.cliLogin(t, false)
	s.certReady(t, home, p)
	ctx, getenv := context.Background(), env(map[string]string{"WERBOT_HOME": home})

	var cfg bytes.Buffer
	require.NoError(t, Run(ctx, []string{"ssh-config"}, getenv, &cfg))
	path := filepath.Join(t.TempDir(), "ssh_config")
	require.NoError(t, os.WriteFile(path, cfg.Bytes(), 0o600))
	host, _, err := net.SplitHostPort(strings.TrimPrefix(s.url, "http://"))
	require.NoError(t, err)
	out, err := exec.CommandContext(ctx, "ssh", "-F", path, "-o", "BatchMode=yes", "srv."+host, "echo", "hi").CombinedOutput() //nolint:gosec // test temp config and loopback host
	require.NoError(t, err, "%s\nconfig:\n%s", out, cfg.String())
	assert.Equal(t, "hi\n", string(out))

	var stdout bytes.Buffer
	require.NoError(t, Run(ctx, []string{"ssh", "srv", "--", "echo", "hi"}, getenv, &stdout))
	assert.Equal(t, "hi\n", stdout.String())
}
