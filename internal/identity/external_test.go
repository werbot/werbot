package identity

import (
	"context"
	"encoding/json"
	"testing"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
	"github.com/werbot/werbot/internal/platform/apperr"
	"github.com/werbot/werbot/internal/platform/principal"
)

// seedProvider stores an IdP of orgID; the secret is never read here.
func seedProvider(t *testing.T, pool *pgxpool.Pool, orgID uuid.UUID, name string, enabled bool) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	require.NoError(t, pool.QueryRow(context.Background(), `INSERT INTO idp_oidc (org_id, name, issuer, client_id, client_secret_ciphertext, key_id, enabled)
		VALUES ($1, $2, 'https://idp.test', 'werbot', '\x00', 'k', $3) RETURNING id`, orgID, name, enabled).Scan(&id))
	return id
}

func externalIn(orgID, idpID uuid.UUID, sub, email string) ExternalLogin {
	return ExternalLogin{OrgID: orgID, IdPID: idpID, Subject: sub, Email: email, Name: "Ext", EmailVerified: true}
}

// lastLoginData is the data of the newest auth.login event.
func lastLoginData(t *testing.T, pool *pgxpool.Pool) map[string]any {
	t.Helper()
	var raw []byte
	require.NoError(t, pool.QueryRow(context.Background(),
		"SELECT data FROM audit_event WHERE action = 'auth.login' ORDER BY created_at DESC LIMIT 1").Scan(&raw))
	var data map[string]any
	require.NoError(t, json.Unmarshal(raw, &data))
	return data
}

func TestOIDCLoginLinksVerifiedEmail(t *testing.T) {
	svc, pool, _ := newTestService(t)
	ctx := context.Background()
	org := seedOrg(t, pool, "acme")
	bob := seedUser(t, pool, org, "bob", principal.RoleMember)
	idp := seedProvider(t, pool, org, "corp", true)

	token, err := svc.LoginExternal(ctx, externalIn(org, idp, "sub-bob", "Bob@Example.com"), "ua")
	require.NoError(t, err)
	p, err := svc.AuthenticateSession(ctx, token)
	require.NoError(t, err)
	assert.Equal(t, bob, p.UserID)
	assert.Nil(t, p.MFAAt, "an IdP login passes no Werbot factor")
	assert.Equal(t, map[string]any{"method": "oidc", "idp_id": idp.String()}, lastLoginData(t, pool))

	// The second login finds the user by subject, whatever the email says.
	in := externalIn(org, idp, "sub-bob", "other@example.com")
	in.EmailVerified = false
	token, err = svc.LoginExternal(ctx, in, "ua")
	require.NoError(t, err)
	p, err = svc.AuthenticateSession(ctx, token)
	require.NoError(t, err)
	assert.Equal(t, bob, p.UserID)
}

func TestLoginExternalUnverifiedEmailDoesNotLink(t *testing.T) {
	svc, pool, _ := newTestService(t)
	org := seedOrg(t, pool, "acme")
	seedUser(t, pool, org, "bob", principal.RoleMember)
	idp := seedProvider(t, pool, org, "corp", true)
	in := externalIn(org, idp, "sub-bob", "bob@example.com")
	in.EmailVerified, in.AllowSignup = false, true

	_, err := svc.LoginExternal(context.Background(), in, "ua")
	require.ErrorIs(t, err, ErrNoAccount)
	var links int
	require.NoError(t, pool.QueryRow(context.Background(), "SELECT count(*) FROM user_idp_link").Scan(&links))
	assert.Zero(t, links)
}

func TestOIDCSignup(t *testing.T) {
	svc, pool, _ := newTestService(t)
	ctx := context.Background()
	org := seedOrg(t, pool, "acme")
	idp := seedProvider(t, pool, org, "corp", true)

	in := externalIn(org, idp, "sub-carol", "carol@example.com")
	_, err := svc.LoginExternal(ctx, in, "ua")
	require.ErrorIs(t, err, ErrNoAccount, "allow_signup off")

	in.AllowSignup, in.Role = true, "admin"
	token, err := svc.LoginExternal(ctx, in, "ua")
	require.NoError(t, err)
	p, err := svc.AuthenticateSession(ctx, token)
	require.NoError(t, err)
	assert.Equal(t, principal.RoleAdmin, p.Role)
	var login, hash string
	require.NoError(t, pool.QueryRow(ctx, "SELECT login, password_hash FROM users WHERE id = $1", p.UserID).Scan(&login, &hash))
	assert.Equal(t, "carol", login)
	assert.Empty(t, hash, "an IdP-provisioned user has no password")

	// carol@other.test wants the same login: it gets a suffix.
	_, err = svc.LoginExternal(ctx, externalIn(org, idp, "sub-carol2", "carol@other.test").withSignup(), "ua")
	require.NoError(t, err)
	require.NoError(t, pool.QueryRow(ctx, "SELECT login FROM users WHERE email = 'carol@other.test'").Scan(&login))
	assert.Equal(t, "carol-2", login)
}

func (in ExternalLogin) withSignup() ExternalLogin { in.AllowSignup = true; return in }

func TestOIDCSyncsMappedTeamsOnly(t *testing.T) {
	svc, pool, _ := newTestService(t)
	ctx := context.Background()
	org := seedOrg(t, pool, "acme")
	bob := seedUser(t, pool, org, "bob", principal.RoleMember)
	idp := seedProvider(t, pool, org, "corp", true)
	x, y := seedTeam(t, pool, org, "x", bob), seedTeam(t, pool, org, "y", bob)
	z := seedTeam(t, pool, org, "z")

	in := externalIn(org, idp, "sub-bob", "bob@example.com")
	in.Teams = map[uuid.UUID]bool{x: false, z: true, uuid.New(): true} // a deleted team is skipped
	_, err := svc.LoginExternal(ctx, in, "ua")
	require.NoError(t, err)
	teams, err := svc.UserTeams(ctx, org, bob)
	require.NoError(t, err)
	assert.ElementsMatch(t, []uuid.UUID{y, z}, teams, "x removed, y (not mapped) kept, z added")
}

func seedTeam(t *testing.T, pool *pgxpool.Pool, orgID uuid.UUID, name string, members ...uuid.UUID) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	var id uuid.UUID
	require.NoError(t, pool.QueryRow(ctx, "INSERT INTO team (org_id, name) VALUES ($1, $2) RETURNING id", orgID, name).Scan(&id))
	for _, u := range members {
		_, err := pool.Exec(ctx, "INSERT INTO team_member (team_id, org_id, user_id) VALUES ($1, $2, $3)", id, orgID, u)
		require.NoError(t, err)
	}
	return id
}

func TestOIDCOwnerRoleUnchanged(t *testing.T) {
	svc, pool, _ := newTestService(t)
	ctx := context.Background()
	org := seedOrg(t, pool, "acme")
	seedUser(t, pool, org, "olga", principal.RoleOwner)
	admin := seedUser(t, pool, org, "adam", principal.RoleAdmin)
	idp := seedProvider(t, pool, org, "corp", true)

	in := externalIn(org, idp, "sub-olga", "olga@example.com")
	in.Role = principal.RoleMember
	token, err := svc.LoginExternal(ctx, in, "ua")
	require.NoError(t, err)
	p, err := svc.AuthenticateSession(ctx, token)
	require.NoError(t, err)
	assert.Equal(t, principal.RoleOwner, p.Role, "owner is never changed")

	in = externalIn(org, idp, "sub-adam", "adam@example.com")
	in.Role = principal.RoleMember
	token, err = svc.LoginExternal(ctx, in, "ua")
	require.NoError(t, err)
	p, err = svc.AuthenticateSession(ctx, token)
	require.NoError(t, err)
	assert.Equal(t, admin, p.UserID)
	assert.Equal(t, principal.RoleMember, p.Role, "role rules change other roles")
	assert.Equal(t, "member", lastLoginData(t, pool)["role"])

	in.Role = "owner" // never granted, even if asked
	token, err = svc.LoginExternal(ctx, in, "ua")
	require.NoError(t, err)
	p, err = svc.AuthenticateSession(ctx, token)
	require.NoError(t, err)
	assert.Equal(t, principal.RoleMember, p.Role)
}

func TestIdPUserHasNoPassword(t *testing.T) {
	svc, pool, mails := newTestService(t)
	ctx := context.Background()
	org := seedOrg(t, pool, "acme")
	idp := seedProvider(t, pool, org, "corp", true)
	_, err := svc.LoginExternal(ctx, externalIn(org, idp, "sub-carol", "carol@example.com").withSignup(), "ua")
	require.NoError(t, err)

	for _, pw := range []string{"", "x", testPassword} {
		_, err = svc.Login(ctx, "carol@example.com", pw, "ua")
		assert.ErrorIs(t, err, errInvalidCredentials, "same answer as a wrong password")
	}
	ok, err := verifyPassword(ctx, "", "")
	assert.False(t, ok, "an empty hash never verifies")
	assert.ErrorIs(t, err, errMalformedHash)

	require.NoError(t, svc.RequestPasswordReset(ctx, "carol@example.com"))
	assert.Empty(t, mails.Sent(), "no reset link for a user without a password")
}

func TestIdPUserChangePasswordRefused(t *testing.T) {
	svc, pool, _ := newTestService(t)
	ctx := context.Background()
	org := seedOrg(t, pool, "acme")
	idp := seedProvider(t, pool, org, "corp", true)
	token, err := svc.LoginExternal(ctx, externalIn(org, idp, "sub-carol", "carol@example.com").withSignup(), "ua")
	require.NoError(t, err)
	p, err := svc.AuthenticateSession(ctx, token)
	require.NoError(t, err)

	err = svc.ChangePassword(principal.With(ctx, p), "", "new correct horse")
	assert.ErrorIs(t, err, apperr.ErrFailedPrecondition)
}

func TestRevokeUserSessions(t *testing.T) {
	svc, pool, _ := newTestService(t)
	ctx := context.Background()
	org := seedOrg(t, pool, "acme")
	seedUser(t, pool, org, "bob", principal.RoleMember)
	seedUser(t, pool, org, "eve", principal.RoleMember)
	bctx, _ := loginCtx(t, svc, "bob")
	_, eveToken := loginCtx(t, svc, "eve")
	p, _ := principal.From(bctx)
	_, err := pool.Exec(ctx, `INSERT INTO cli_session (org_id, user_id, client_id, refresh_hash, expires_at, absolute_expires_at)
		VALUES ($1, $2, 'werbot-cli', '\x01', now() + interval '1 day', now() + interval '1 day')`, org, p.UserID)
	require.NoError(t, err)

	require.NoError(t, svc.RevokeUserSessions(ctx, org, p.UserID, "idp"))
	var sessions int
	require.NoError(t, pool.QueryRow(ctx, "SELECT count(*) FROM user_session WHERE user_id = $1", p.UserID).Scan(&sessions))
	assert.Zero(t, sessions)
	var reason string
	require.NoError(t, pool.QueryRow(ctx, "SELECT revoke_reason FROM cli_session WHERE user_id = $1", p.UserID).Scan(&reason))
	assert.Equal(t, "idp", reason)
	_, err = svc.AuthenticateSession(ctx, eveToken)
	assert.NoError(t, err, "other users keep their sessions")
}

func TestListIdentityProvidersPublic(t *testing.T) {
	svc, pool, _ := newTestService(t)
	org := seedOrg(t, pool, "acme")
	on := seedProvider(t, pool, org, "corp", true)
	seedProvider(t, pool, org, "old", false)

	res, err := NewHandler(svc).ListIdentityProviders(context.Background(), connect.NewRequest(&werbotv1.AuthServiceListIdentityProvidersRequest{}))
	require.NoError(t, err, "no session needed")
	require.Len(t, res.Msg.GetProviders(), 1)
	assert.Equal(t, on.String(), res.Msg.GetProviders()[0].GetId())
	assert.Equal(t, "corp", res.Msg.GetProviders()[0].GetName())
}

// An IdP-provisioned user has no password: an invite accepted without a
// session answers as a wrong password, never Internal.
func TestAcceptInviteExistingUserWithoutPassword(t *testing.T) {
	svc, pool, q := newTestService(t)
	org1, org2 := seedOrg(t, pool, "Acme"), seedOrg(t, pool, "Other")
	seedUser(t, pool, org1, "alice", principal.RoleOwner)
	dave := seedUser(t, pool, org2, "dave", principal.RoleMember)
	_, err := pool.Exec(context.Background(), "UPDATE users SET password_hash = '' WHERE id = $1", dave)
	require.NoError(t, err)
	ctx, _ := loginCtx(t, svc, "alice")
	raw := invite(ctx, t, svc, q, "dave@example.com", principal.RoleMember)

	for _, pw := range []string{"", testPassword} {
		_, err = svc.AcceptInvite(context.Background(), AcceptInput{Token: raw, Password: pw}, "test")
		assert.ErrorIs(t, err, apperr.ErrUnauthenticated, "password %q", pw)
	}
}
