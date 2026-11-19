package identity

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/platform/apperr"
	"github.com/werbot/werbot/internal/platform/principal"
)

func setupInput(email, login string) SetupInput {
	return SetupInput{Email: email, Login: login, Name: "Owner", Password: testPassword, OrganizationName: "Acme"}
}

func TestSetupOnlyOnce(t *testing.T) {
	svc, pool, _ := newTestService(t)
	ctx := context.Background()

	needs, err := svc.NeedsSetup(ctx)
	require.NoError(t, err)
	assert.True(t, needs)

	token, err := svc.Setup(ctx, setupInput(" Owner@Example.com", "owner"), "test")
	require.NoError(t, err)
	assert.NotEmpty(t, token)

	needs, err = svc.NeedsSetup(ctx)
	require.NoError(t, err)
	assert.False(t, needs)

	_, err = svc.Setup(ctx, setupInput("other@example.com", "other"), "test")
	assert.ErrorIs(t, err, apperr.ErrFailedPrecondition)

	p, err := svc.AuthenticateSession(ctx, token)
	require.NoError(t, err)
	assert.Equal(t, principal.RoleOwner, p.Role)
	var email string
	require.NoError(t, pool.QueryRow(ctx, "SELECT email FROM users WHERE id = $1", p.UserID).Scan(&email))
	assert.Equal(t, "owner@example.com", email)
	assert.Equal(t, []string{"auth.setup"}, auditActions(t, pool))
}

func TestConcurrentSetupCreatesOneOwner(t *testing.T) {
	svc, pool, _ := newTestService(t)
	ctx := context.Background()
	tokens := make([]string, 2)
	errs := make([]error, 2)
	var wg sync.WaitGroup
	for i := range tokens {
		wg.Go(func() {
			login := fmt.Sprintf("owner%d", i)
			tokens[i], errs[i] = svc.Setup(ctx, setupInput(login+"@example.com", login), "test")
		})
	}
	wg.Wait()

	ok := 0
	for i := range tokens {
		if errs[i] == nil {
			ok++
			assert.NotEmpty(t, tokens[i])
		} else {
			assert.ErrorIs(t, errs[i], apperr.ErrFailedPrecondition)
		}
	}
	assert.Equal(t, 1, ok)
	var users, orgs int
	require.NoError(t, pool.QueryRow(ctx, "SELECT count(*) FROM users").Scan(&users))
	require.NoError(t, pool.QueryRow(ctx, "SELECT count(*) FROM organization").Scan(&orgs))
	assert.Equal(t, 1, users)
	assert.Equal(t, 1, orgs)
}

func TestSetupRejectsShortPassword(t *testing.T) {
	svc, _, _ := newTestService(t)
	in := setupInput("owner@example.com", "owner")
	in.Password = "12345678901"
	_, err := svc.Setup(context.Background(), in, "test")
	assert.ErrorIs(t, err, apperr.ErrInvalidArgument)
}

func TestSetupRejectsBadLogin(t *testing.T) {
	svc, _, _ := newTestService(t)
	_, err := svc.Setup(context.Background(), setupInput("owner@example.com", "Owner"), "test")
	assert.ErrorIs(t, err, apperr.ErrInvalidArgument)
}

func TestLogin(t *testing.T) {
	svc, pool, _ := newTestService(t)
	ctx := context.Background()
	orgID := seedOrg(t, pool, "Acme")
	userID := seedUser(t, pool, orgID, "alice", principal.RoleOwner)

	token, err := svc.Login(ctx, "alice@example.com", testPassword, "test")
	require.NoError(t, err)
	p, err := svc.AuthenticateSession(ctx, token)
	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, p.SessionID)
	assert.Equal(t, principal.Principal{UserID: userID, OrgID: orgID, Role: principal.RoleOwner, SessionID: p.SessionID, AuthMethod: principal.AuthMethodCookie}, p)

	_, wrongPw := svc.Login(ctx, "alice@example.com", "wrong horse battery", "test")
	assert.ErrorIs(t, wrongPw, apperr.ErrUnauthenticated)
	_, unknown := svc.Login(ctx, "bob@example.com", testPassword, "test")
	assert.ErrorIs(t, unknown, apperr.ErrUnauthenticated)
	assert.Equal(t, wrongPw.Error(), unknown.Error())

	// A user without membership gets the same answer.
	_, err = pool.Exec(ctx, "DELETE FROM membership WHERE user_id = $1", userID)
	require.NoError(t, err)
	_, noMember := svc.Login(ctx, "alice@example.com", testPassword, "test")
	assert.ErrorIs(t, noMember, apperr.ErrUnauthenticated)
	assert.Equal(t, wrongPw.Error(), noMember.Error())

	assert.Contains(t, auditActions(t, pool), "auth.login")
}

func TestLoginEmailCaseInsensitive(t *testing.T) {
	svc, pool, _ := newTestService(t)
	seedUser(t, pool, seedOrg(t, pool, "Globex"), "bob", principal.RoleMember)
	_, err := svc.Login(context.Background(), "  Bob@Example.COM ", testPassword, "test")
	assert.NoError(t, err)
}

func TestAuthenticateSessionExpired(t *testing.T) {
	svc, pool, _ := newTestService(t)
	ctx := context.Background()
	seedUser(t, pool, seedOrg(t, pool, "Acme"), "alice", principal.RoleMember)
	_, token := loginCtx(t, svc, "alice")

	_, err := pool.Exec(ctx, "UPDATE user_session SET expires_at = now() - interval '1 second'")
	require.NoError(t, err)
	_, err = svc.AuthenticateSession(ctx, token)
	assert.ErrorIs(t, err, apperr.ErrUnauthenticated)
	_, err = svc.AuthenticateSession(ctx, "garbage")
	assert.ErrorIs(t, err, apperr.ErrUnauthenticated)
}

func TestAuthenticateSessionSlides(t *testing.T) {
	svc, pool, _ := newTestService(t)
	ctx := context.Background()
	seedUser(t, pool, seedOrg(t, pool, "Acme"), "alice", principal.RoleMember)
	_, token := loginCtx(t, svc, "alice")

	_, err := pool.Exec(ctx, "UPDATE user_session SET last_seen_at = now() - interval '2 minutes', expires_at = now() + interval '1 hour'")
	require.NoError(t, err)
	_, err = svc.AuthenticateSession(ctx, token)
	require.NoError(t, err)
	var slid bool
	require.NoError(t, pool.QueryRow(ctx, "SELECT expires_at > now() + interval '6 days 23 hours' FROM user_session").Scan(&slid))
	assert.True(t, slid)
}

func TestLogout(t *testing.T) {
	svc, pool, _ := newTestService(t)
	seedUser(t, pool, seedOrg(t, pool, "Acme"), "alice", principal.RoleMember)
	ctx, token := loginCtx(t, svc, "alice")

	require.NoError(t, svc.Logout(ctx))
	_, err := svc.AuthenticateSession(context.Background(), token)
	assert.ErrorIs(t, err, apperr.ErrUnauthenticated)
	assert.Contains(t, auditActions(t, pool), "auth.logout")
}
