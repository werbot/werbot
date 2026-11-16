package identity

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/platform/apperr"
	"github.com/werbot/werbot/internal/platform/principal"
)

const deleteURLPrefix = "https://werbot.test/profile/delete/"

func TestUpdateProfile(t *testing.T) {
	svc, pool, _ := newTestService(t)
	org := seedOrg(t, pool, "Acme")
	seedUser(t, pool, org, "alice", principal.RoleOwner)
	seedUser(t, pool, org, "bob", principal.RoleMember)
	ctx, _ := loginCtx(t, svc, "alice")

	u, err := svc.UpdateProfile(ctx, "Alice New", "  Alice.New@Example.com ", testPassword)
	require.NoError(t, err)
	assert.Equal(t, "Alice New", u.Name)
	assert.Equal(t, "alice.new@example.com", u.Email)
	got, err := svc.Profile(ctx)
	require.NoError(t, err)
	assert.Equal(t, u, got)

	_, err = svc.UpdateProfile(ctx, "Alice", "BOB@example.com", testPassword)
	assert.ErrorIs(t, err, apperr.ErrAlreadyExists)
	assert.Contains(t, auditActions(t, pool), "profile.update")
}

// profileAuditData returns the data of the only profile.update audit event.
func profileAuditData(t *testing.T, pool *pgxpool.Pool) map[string]any {
	t.Helper()
	rows, err := pool.Query(context.Background(), "SELECT data FROM audit_event WHERE action = 'profile.update'")
	require.NoError(t, err)
	data, err := pgx.CollectRows(rows, pgx.RowTo[map[string]any])
	require.NoError(t, err)
	require.Len(t, data, 1)
	return data[0]
}

func TestUpdateProfileEmailRequiresPassword(t *testing.T) {
	svc, pool, q := newTestService(t)
	seedUser(t, pool, seedOrg(t, pool, "Acme"), "alice", principal.RoleOwner)
	resetToken := requestReset(t, svc, q)
	ctx, _ := loginCtx(t, svc, "alice")
	_, other := loginCtx(t, svc, "alice")
	audit := auditActions(t, pool)

	for _, pw := range []string{"", "wrong password!"} {
		_, err := svc.UpdateProfile(ctx, "Alice", "new@example.com", pw)
		require.ErrorIs(t, err, apperr.ErrInvalidArgument, "password %q", pw)
	}
	got, err := svc.Profile(ctx)
	require.NoError(t, err)
	assert.Equal(t, "alice@example.com", got.Email)
	assert.Equal(t, "alice", got.Name, "a refused change keeps the name too")
	assert.Equal(t, audit, auditActions(t, pool), "a refused change writes no audit event")

	u, err := svc.UpdateProfile(ctx, "Alice", "New@Example.com", testPassword)
	require.NoError(t, err)
	assert.Equal(t, "new@example.com", u.Email)
	assert.Equal(t, map[string]any{"email_changed": true}, profileAuditData(t, pool))

	bg := context.Background()
	_, err = svc.AuthenticateSession(bg, other)
	assert.ErrorIs(t, err, apperr.ErrUnauthenticated, "other sessions are revoked")
	_, err = svc.CurrentSession(ctx)
	require.NoError(t, err, "the current session survives")
	assert.ErrorIs(t, svc.ResetPassword(bg, resetToken, "new correct horse"), apperr.ErrNotFound,
		"a reset link sent to the old address is void")
}

func TestUpdateProfileNameOnlyNoPassword(t *testing.T) {
	svc, pool, _ := newTestService(t)
	seedUser(t, pool, seedOrg(t, pool, "Acme"), "alice", principal.RoleOwner)
	ctx, _ := loginCtx(t, svc, "alice")
	_, other := loginCtx(t, svc, "alice")

	u, err := svc.UpdateProfile(ctx, "Alice New", "  ALICE@Example.com ", "")
	require.NoError(t, err)
	assert.Equal(t, "Alice New", u.Name)
	assert.Equal(t, "alice@example.com", u.Email)
	assert.Empty(t, profileAuditData(t, pool))
	_, err = svc.AuthenticateSession(context.Background(), other)
	assert.NoError(t, err, "a name change keeps other sessions")
}

func TestChangePassword(t *testing.T) {
	svc, pool, _ := newTestService(t)
	seedUser(t, pool, seedOrg(t, pool, "Acme"), "alice", principal.RoleOwner)
	ctx, _ := loginCtx(t, svc, "alice")
	_, other := loginCtx(t, svc, "alice")

	assert.ErrorIs(t, svc.ChangePassword(ctx, "wrong password!", "new correct horse"), apperr.ErrInvalidArgument)
	assert.ErrorIs(t, svc.ChangePassword(ctx, testPassword, "short"), apperr.ErrInvalidArgument)
	require.NoError(t, svc.ChangePassword(ctx, testPassword, "new correct horse"))

	bg := context.Background()
	_, err := svc.AuthenticateSession(bg, other)
	assert.ErrorIs(t, err, apperr.ErrUnauthenticated)
	p, _ := principal.From(ctx)
	_, err = svc.CurrentSession(ctx)
	require.NoError(t, err, "current session %s survives", p.SessionID)
	_, err = svc.Login(bg, "alice@example.com", "new correct horse", "test")
	require.NoError(t, err)
	assert.Contains(t, auditActions(t, pool), "profile.password_change")
}

// deletionToken has ctx's user request deletion and returns the emailed token.
func deletionToken(ctx context.Context, t *testing.T, svc *Service, q *fakeQueue) string {
	t.Helper()
	n := len(q.sent)
	require.NoError(t, svc.RequestAccountDeletion(ctx))
	require.Len(t, q.sent, n+1)
	last := q.sent[n]
	assert.Equal(t, "account_deletion", last.Template)
	require.True(t, strings.HasPrefix(last.Data["url"], deleteURLPrefix))
	return strings.TrimPrefix(last.Data["url"], deleteURLPrefix)
}

func TestAccountDeletion(t *testing.T) {
	svc, pool, q := newTestService(t)
	org := seedOrg(t, pool, "Acme")
	seedUser(t, pool, org, "alice", principal.RoleOwner)
	bobID := seedUser(t, pool, org, "bob", principal.RoleAdmin)
	ctx, _ := loginCtx(t, svc, "bob")

	token := deletionToken(ctx, t, svc, q)
	assert.Equal(t, "bob@example.com", q.sent[0].To)
	assert.Equal(t, "bob", q.sent[0].Data["name"])
	require.NoError(t, svc.ConfirmAccountDeletion(ctx, token))

	bg := context.Background()
	for _, table := range []string{"users WHERE id", "membership WHERE user_id", "user_session WHERE user_id"} {
		var n int
		require.NoError(t, pool.QueryRow(bg, "SELECT count(*) FROM "+table+" = $1", bobID).Scan(&n))
		assert.Zero(t, n, table)
	}
	var actor uuid.UUID
	require.NoError(t, pool.QueryRow(bg, "SELECT actor_user_id FROM audit_event WHERE action = 'profile.delete'").Scan(&actor))
	assert.Equal(t, bobID, actor)
	assert.Contains(t, auditActions(t, pool), "profile.deletion_request")
}

func TestAccountDeletionTokenOfOtherUser(t *testing.T) {
	svc, pool, q := newTestService(t)
	org := seedOrg(t, pool, "Acme")
	seedUser(t, pool, org, "alice", principal.RoleOwner)
	bobID := seedUser(t, pool, org, "bob", principal.RoleAdmin)
	seedUser(t, pool, org, "carol", principal.RoleMember)
	bobCtx, _ := loginCtx(t, svc, "bob")
	carolCtx, _ := loginCtx(t, svc, "carol")
	token := deletionToken(bobCtx, t, svc, q)

	assert.ErrorIs(t, svc.ConfirmAccountDeletion(carolCtx, token), apperr.ErrNotFound)
	var n int
	require.NoError(t, pool.QueryRow(context.Background(), "SELECT count(*) FROM users WHERE id = $1", bobID).Scan(&n))
	assert.Equal(t, 1, n)
	require.NoError(t, svc.ConfirmAccountDeletion(bobCtx, token), "the token is not spent by the refused attempt")
}

func TestSoleOwnerOfOtherOrgCannotDeleteAccount(t *testing.T) {
	svc, pool, q := newTestService(t)
	org1, org2 := seedOrg(t, pool, "Acme"), seedOrg(t, pool, "Other")
	seedUser(t, pool, org1, "alice", principal.RoleOwner)
	dave := seedUser(t, pool, org1, "dave", principal.RoleMember)
	ctx, _ := loginCtx(t, svc, "dave") // session in org1: the first membership
	token := deletionToken(ctx, t, svc, q)
	bg := context.Background()
	_, err := pool.Exec(bg, "INSERT INTO membership (org_id, user_id, role) VALUES ($1, $2, 'owner')", org2, dave)
	require.NoError(t, err)

	assert.ErrorIs(t, svc.RequestAccountDeletion(ctx), apperr.ErrFailedPrecondition)
	assert.ErrorIs(t, svc.ConfirmAccountDeletion(ctx, token), apperr.ErrFailedPrecondition)
	var n int
	require.NoError(t, pool.QueryRow(bg, "SELECT count(*) FROM membership WHERE user_id = $1", dave).Scan(&n))
	assert.Equal(t, 2, n, "nothing deleted")

	_, err = pool.Exec(bg, "DELETE FROM membership WHERE org_id = $1", org2)
	require.NoError(t, err)
	require.NoError(t, svc.ConfirmAccountDeletion(ctx, token), "the refused attempt left the token unspent")
}

func TestLastOwnerCannotDeleteAccount(t *testing.T) {
	svc, pool, q := newTestService(t)
	org := seedOrg(t, pool, "Acme")
	seedUser(t, pool, org, "alice", principal.RoleOwner)
	bobID := seedUser(t, pool, org, "bob", principal.RoleAdmin)
	ctx, _ := loginCtx(t, svc, "alice")

	assert.ErrorIs(t, svc.RequestAccountDeletion(ctx), apperr.ErrFailedPrecondition)
	_, err := pool.Exec(context.Background(), "UPDATE membership SET role = 'owner' WHERE user_id = $1", bobID)
	require.NoError(t, err)
	token := deletionToken(ctx, t, svc, q)
	require.NoError(t, svc.ConfirmAccountDeletion(ctx, token))
}
