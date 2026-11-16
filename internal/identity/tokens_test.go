package identity

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/platform/apperr"
	"github.com/werbot/werbot/internal/platform/dbx"
	"github.com/werbot/werbot/internal/platform/principal"
)

// inOrg runs fn in a transaction scoped to orgID and userID.
func inOrg(t *testing.T, pool *pgxpool.Pool, orgID, userID uuid.UUID, fn func(st Store) error) error {
	t.Helper()
	return dbx.InOrg(context.Background(), pool, orgID, userID, func(tx pgx.Tx) error { return fn(newStore(tx)) })
}

func TestOneTimeTokenSingleUse(t *testing.T) {
	_, pool, _ := newTestService(t)
	ctx := context.Background()
	orgID := seedOrg(t, pool, "Acme")
	userID := seedUser(t, pool, orgID, "alice", principal.RoleOwner)

	var raw string
	require.NoError(t, inOrg(t, pool, orgID, userID, func(st Store) (err error) {
		raw, err = issueToken(ctx, st, tokenPasswordReset, tokenClaims{UserID: &userID, Email: "a@example.com"})
		return err
	}))
	assert.Len(t, raw, 43)

	// the wrong kind does not spend the token
	require.NoError(t, inOrg(t, pool, orgID, userID, func(st Store) error {
		_, err := consumeToken(ctx, st, tokenInvite, raw)
		assert.ErrorIs(t, err, apperr.ErrNotFound)
		return nil
	}))
	var c tokenClaims
	require.NoError(t, inOrg(t, pool, orgID, userID, func(st Store) (err error) {
		c, err = consumeToken(ctx, st, tokenPasswordReset, raw)
		return err
	}))
	assert.Equal(t, userID, *c.UserID)
	assert.Equal(t, "a@example.com", c.Email)
	assert.Empty(t, c.Role)
	require.NoError(t, inOrg(t, pool, orgID, userID, func(st Store) error {
		_, err := consumeToken(ctx, st, tokenPasswordReset, raw)
		assert.ErrorIs(t, err, apperr.ErrNotFound)
		return nil
	}))

	var stored int
	require.NoError(t, pool.QueryRow(ctx, "SELECT count(*) FROM one_time_token WHERE token_hash = $1", []byte(raw)).Scan(&stored))
	assert.Zero(t, stored, "the database keeps a hash, not the raw token")
}

func TestOneTimeTokenExpired(t *testing.T) {
	_, pool, _ := newTestService(t)
	ctx := context.Background()
	orgID := seedOrg(t, pool, "Acme")
	userID := seedUser(t, pool, orgID, "alice", principal.RoleOwner)

	var raw string
	require.NoError(t, inOrg(t, pool, orgID, userID, func(st Store) (err error) {
		raw, err = issueToken(ctx, st, tokenPasswordReset, tokenClaims{UserID: &userID})
		return err
	}))
	_, err := pool.Exec(ctx, "UPDATE one_time_token SET expires_at = now() - interval '1 second'")
	require.NoError(t, err)
	require.NoError(t, inOrg(t, pool, orgID, userID, func(st Store) error {
		_, err := consumeToken(ctx, st, tokenPasswordReset, raw)
		assert.ErrorIs(t, err, apperr.ErrNotFound)
		return nil
	}))
}
