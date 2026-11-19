package identity

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/rsa"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"

	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
	"github.com/werbot/werbot/internal/platform/apperr"
	"github.com/werbot/werbot/internal/platform/principal"
	"github.com/werbot/werbot/internal/platform/sshca"
)

func authorizedLine(k ssh.PublicKey) string {
	return strings.TrimSuffix(string(ssh.MarshalAuthorizedKey(k)), "\n")
}

// Every replica's first use converges on one sealed key per organization.
func TestUserCALazyOnce(t *testing.T) {
	svc, pool, _ := newTestService(t)
	ctx := context.Background()
	org, other := seedOrg(t, pool, "Acme"), seedOrg(t, pool, "Other")

	keys := make([]string, 8)
	var wg sync.WaitGroup
	for i := range keys {
		wg.Go(func() { // a fresh Service each: replicas with empty caches
			ca, err := New(pool, nil, svc.cfg).UserCA(ctx, org)
			if assert.NoError(t, err) {
				keys[i] = authorizedLine(ca.PublicKey())
			}
		})
	}
	wg.Wait()
	for _, k := range keys {
		assert.Equal(t, keys[0], k)
	}

	var n int
	require.NoError(t, pool.QueryRow(ctx, "SELECT count(*) FROM org_ssh_ca WHERE org_id = $1", org).Scan(&n))
	assert.Equal(t, 1, n)
	var ct []byte
	var keyID, pubLine string
	require.NoError(t, pool.QueryRow(ctx, "SELECT ciphertext, key_id, public_key FROM org_ssh_ca WHERE org_id = $1", org).Scan(&ct, &keyID, &pubLine))
	seed, err := svc.cfg.Box.Open(ct, keyID)
	require.NoError(t, err)
	require.Len(t, seed, ed25519.SeedSize)
	assert.False(t, bytes.Contains(ct, seed), "ciphertext holds the seed in clear")
	assert.Equal(t, keys[0], pubLine)
	sealed, err := ssh.NewPublicKey(ed25519.NewKeyFromSeed(seed).Public())
	require.NoError(t, err)
	assert.Equal(t, keys[0], authorizedLine(sealed))

	got, err := svc.UserCAPublicKeys(ctx, org)
	require.NoError(t, err)
	assert.Equal(t, []string{keys[0]}, got)
	otherKeys, err := svc.UserCAPublicKeys(ctx, other)
	require.NoError(t, err)
	require.Len(t, otherKeys, 1)
	assert.NotEqual(t, keys[0], otherKeys[0], "each organization has its own CA")
}

func TestIssueUserCert(t *testing.T) {
	s := newCLIStand(t)
	ctx := context.Background()
	org := seedOrg(t, s.pool, "Acme")
	alice := seedUser(t, s.pool, org, "alice", principal.RoleMember)
	p := s.asBearer(t, s.cliLogin(t, org, alice).AccessToken, principal.RoleMember)
	audits := auditActions(t, s.pool)
	_, pub := newEd25519(t, "")

	cert, err := s.svc.IssueUserCert(ctx, p, pub)
	require.NoError(t, err)
	assert.Equal(t, alice.String(), cert.KeyId)
	assert.Equal(t, []string{"alice"}, cert.ValidPrincipals)
	assert.Equal(t, uint64((sshca.UserCertTTL + sshca.ClockSkew).Seconds()), cert.ValidBefore-cert.ValidAfter)
	assert.Equal(t, pub.Marshal(), cert.Key.Marshal())
	ca, err := s.svc.UserCA(ctx, org)
	require.NoError(t, err)
	assert.Equal(t, ca.PublicKey().Marshal(), cert.SignatureKey.Marshal())
	id, err := sshca.Parse(cert)
	require.NoError(t, err)
	assert.Equal(t, sshca.Identity{OrgID: org, UserID: alice, CLISessionID: p.CLISessionID, Login: "alice"}, id)

	rec := httptest.NewRecorder()
	s.svc.cfg.Metrics.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	assert.Contains(t, rec.Body.String(), "werbot_user_certs_issued_total 1\n")
	assert.Equal(t, audits, auditActions(t, s.pool), "issuing writes no audit event")

	// over CertService with the CLI token
	res, err := s.certs().IssueUserCert(ctx, bearer(&werbotv1.IssueUserCertRequest{PublicKey: authorizedLine(pub)}, s.cliLogin(t, org, alice).AccessToken))
	require.NoError(t, err)
	parsed, _, _, _, err := ssh.ParseAuthorizedKey([]byte(res.Msg.GetCertificate()))
	require.NoError(t, err)
	got, ok := parsed.(*ssh.Certificate)
	require.True(t, ok)
	assert.Equal(t, time.Unix(int64(got.ValidBefore), 0).UTC(), res.Msg.GetValidBefore().AsTime()) //nolint:gosec // test cert
}

func TestIssueUserCertRequiresOAuth(t *testing.T) {
	s := newCLIStand(t)
	ctx := context.Background()
	org := seedOrg(t, s.pool, "Acme")
	alice := seedUser(t, s.pool, org, "alice", principal.RoleMember)
	tok := s.cliLogin(t, org, alice).AccessToken
	p := s.asBearer(t, tok, principal.RoleMember)
	edLine, edPub := newEd25519(t, "")

	cookie := principal.Principal{UserID: alice, OrgID: org, Role: principal.RoleMember, AuthMethod: principal.AuthMethodCookie}
	_, err := s.svc.IssueUserCert(ctx, cookie, edPub)
	require.ErrorIs(t, err, apperr.ErrPermissionDenied)
	_, err = s.svc.IssueUserCert(ctx, principal.Principal{UserID: alice, OrgID: org}, edPub)
	require.ErrorIs(t, err, apperr.ErrPermissionDenied, "no auth method is no CLI token")

	rk, err := rsa.GenerateKey(rand.Reader, 3072)
	require.NoError(t, err)
	rsaPub, err := ssh.NewPublicKey(&rk.PublicKey)
	require.NoError(t, err)
	_, err = s.svc.IssueUserCert(ctx, p, rsaPub)
	require.ErrorIs(t, err, apperr.ErrInvalidArgument)
	assert.NotContains(t, err.Error(), authorizedLine(rsaPub))

	for _, key := range []string{"ssh-ed25519 AAAAgarbage", authorizedLine(rsaPub), "not a key"} {
		_, err := s.certs().IssueUserCert(ctx, bearer(&werbotv1.IssueUserCertRequest{PublicKey: key}, tok))
		require.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err), key)
		assert.NotContains(t, err.Error(), key)
		assert.NotContains(t, err.Error(), "AAAA")
	}
	_, err = s.certs().IssueUserCert(ctx, connect.NewRequest(&werbotv1.IssueUserCertRequest{PublicKey: edLine}))
	require.Equal(t, connect.CodeUnauthenticated, connect.CodeOf(err))
}

func TestAuthenticateCert(t *testing.T) {
	s := newCLIStand(t)
	ctx := context.Background()
	org, other := seedOrg(t, s.pool, "Acme"), seedOrg(t, s.pool, "Other")
	alice := seedUser(t, s.pool, org, "alice", principal.RoleMember)
	bob := seedUser(t, s.pool, org, "bob", principal.RoleMember)
	pa := s.asBearer(t, s.cliLogin(t, org, alice).AccessToken, principal.RoleMember)
	pb := s.asBearer(t, s.cliLogin(t, org, bob).AccessToken, principal.RoleMember)
	id := sshca.Identity{OrgID: org, UserID: alice, CLISessionID: pa.CLISessionID, Login: "alice"}

	ref, err := s.svc.AuthenticateCert(ctx, org, id)
	require.NoError(t, err)
	assert.Equal(t, UserRef{UserID: alice, Login: "alice", Role: principal.RoleMember}, ref)

	for name, c := range map[string]struct {
		org uuid.UUID
		id  sshca.Identity
	}{
		"other org":   {other, id},
		"other login": {org, sshca.Identity{OrgID: org, UserID: alice, CLISessionID: pa.CLISessionID, Login: "bob"}},
		"other user":  {org, sshca.Identity{OrgID: org, UserID: bob, CLISessionID: pa.CLISessionID, Login: "bob"}},
		"no session":  {org, sshca.Identity{OrgID: org, UserID: alice, CLISessionID: uuid.New(), Login: "alice"}},
	} {
		_, err := s.svc.AuthenticateCert(ctx, c.org, c.id)
		require.ErrorIs(t, err, apperr.ErrNotFound, name)
	}

	require.NoError(t, s.svc.RevokeCLISession(ctx, pa, pa.CLISessionID, "revoked"))
	_, err = s.svc.AuthenticateCert(ctx, org, id)
	require.ErrorIs(t, err, apperr.ErrNotFound, "revoked")

	bobID := sshca.Identity{OrgID: org, UserID: bob, CLISessionID: pb.CLISessionID, Login: "bob"}
	_, err = s.svc.AuthenticateCert(ctx, org, bobID)
	require.NoError(t, err)
	_, err = s.pool.Exec(ctx, "DELETE FROM membership WHERE org_id = $1 AND user_id = $2", org, bob)
	require.NoError(t, err)
	_, err = s.svc.AuthenticateCert(ctx, org, bobID)
	require.ErrorIs(t, err, apperr.ErrNotFound, "member removed")
}
