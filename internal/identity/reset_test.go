package identity

import (
	"context"
	"strings"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
	"github.com/werbot/werbot/internal/platform/apperr"
	"github.com/werbot/werbot/internal/platform/principal"
)

const resetURLPrefix = "https://werbot.test/reset-password/"

// requestReset seeds alice, requests a reset for her and returns the emailed token.
func requestReset(t *testing.T, svc *Service, q *fakeQueue) string {
	t.Helper()
	require.NoError(t, svc.RequestPasswordReset(context.Background(), "Alice@Example.com"))
	require.Len(t, q.sent, 1)
	url := q.sent[0].Data["url"]
	require.True(t, strings.HasPrefix(url, resetURLPrefix))
	return strings.TrimPrefix(url, resetURLPrefix)
}

func TestRequestPasswordResetDoesNotRevealAccount(t *testing.T) {
	svc, pool, q := newTestService(t)
	seedUser(t, pool, seedOrg(t, pool, "Acme"), "alice", principal.RoleOwner)
	ctx := context.Background()

	require.NoError(t, svc.RequestPasswordReset(ctx, "alice@example.com"))
	require.NoError(t, svc.RequestPasswordReset(ctx, "nobody@example.com"))
	require.Len(t, q.sent, 1)
	assert.Equal(t, "alice@example.com", q.sent[0].To)
	assert.Equal(t, "password_reset", q.sent[0].Template)
	assert.Equal(t, "alice", q.sent[0].Data["name"])
	assert.True(t, strings.HasPrefix(q.sent[0].Data["url"], resetURLPrefix))
	assert.Equal(t, []string{"auth.password_reset_request"}, auditActions(t, pool))

	client := newAuthClient(t, svc)
	for _, email := range []string{"alice@example.com", "nobody@example.com"} {
		res, err := client.RequestPasswordReset(ctx, connect.NewRequest(&werbotv1.RequestPasswordResetRequest{Email: email}))
		require.NoError(t, err)
		assert.True(t, proto.Equal(res.Msg, &werbotv1.RequestPasswordResetResponse{}))
	}
}

func TestResetPassword(t *testing.T) {
	svc, pool, q := newTestService(t)
	seedUser(t, pool, seedOrg(t, pool, "Acme"), "alice", principal.RoleOwner)
	ctx := context.Background()
	token := requestReset(t, svc, q)

	assert.ErrorIs(t, svc.ResetPassword(ctx, token, "elevenchars"), apperr.ErrInvalidArgument)
	require.NoError(t, svc.ResetPassword(ctx, token, "new correct horse"), "a rejected password does not spend the token")

	_, err := svc.Login(ctx, "alice@example.com", "new correct horse", "test")
	require.NoError(t, err)
	_, err = svc.Login(ctx, "alice@example.com", testPassword, "test")
	assert.ErrorIs(t, err, apperr.ErrUnauthenticated)

	assert.ErrorIs(t, svc.ResetPassword(ctx, token, "another correct horse"), apperr.ErrNotFound)
	assert.ErrorIs(t, svc.ResetPassword(ctx, "no-such-token", "another correct horse"), apperr.ErrNotFound)
	assert.Contains(t, auditActions(t, pool), "auth.password_reset")
}

func TestResetPasswordRevokesSessions(t *testing.T) {
	svc, pool, q := newTestService(t)
	seedUser(t, pool, seedOrg(t, pool, "Acme"), "alice", principal.RoleOwner)
	ctx := context.Background()
	_, sessionToken := loginCtx(t, svc, "alice")

	require.NoError(t, svc.ResetPassword(ctx, requestReset(t, svc, q), "new correct horse"))
	_, err := svc.AuthenticateSession(ctx, sessionToken)
	assert.ErrorIs(t, err, apperr.ErrUnauthenticated)
}
