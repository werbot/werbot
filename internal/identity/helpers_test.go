package identity

import (
	"bytes"
	"context"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/platform/db/dbtest"
	"github.com/werbot/werbot/internal/platform/metrics"
	"github.com/werbot/werbot/internal/platform/principal"
	"github.com/werbot/werbot/internal/platform/secretbox"
)

const testPassword = "correct horse battery"

// testPasswordHash is computed once: argon2id is deliberately slow.
var testPasswordHash = sync.OnceValue(func() string {
	h, err := hashPassword(context.Background(), testPassword)
	if err != nil {
		panic(err)
	}
	return h
})

func newTestService(t *testing.T) (*Service, *pgxpool.Pool, *dbtest.Mails) {
	t.Helper()
	pool, _ := dbtest.New(t)
	q := &dbtest.Mails{}
	box, err := secretbox.New(bytes.Repeat([]byte{7}, 32), nil)
	require.NoError(t, err)
	cfg := Config{WebURL: "https://werbot.test", SecureCookies: true, Box: box, Metrics: metrics.NewRegistry()}
	return New(pool, q, cfg), pool, q
}

func seedOrg(t *testing.T, pool *pgxpool.Pool, name string) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	require.NoError(t, pool.QueryRow(context.Background(),
		"INSERT INTO organization (name) VALUES ($1) RETURNING id", name).Scan(&id))
	_, err := pool.Exec(context.Background(), "INSERT INTO org_setting (org_id) VALUES ($1)", id)
	require.NoError(t, err)
	return id
}

// seedUser creates <login>@example.com with testPassword and a membership in orgID.
func seedUser(t *testing.T, pool *pgxpool.Pool, orgID uuid.UUID, login, role string) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	var id uuid.UUID
	require.NoError(t, pool.QueryRow(ctx,
		"INSERT INTO users (email, login, name, password_hash) VALUES ($1, $2, $2, $3) RETURNING id",
		login+"@example.com", login, testPasswordHash()).Scan(&id))
	_, err := pool.Exec(ctx, "INSERT INTO membership (org_id, user_id, role) VALUES ($1, $2, $3)", orgID, id, role)
	require.NoError(t, err)
	return id
}

// loginCtx logs login in and returns a context carrying its principal and the session token.
func loginCtx(t *testing.T, svc *Service, login string) (context.Context, string) {
	t.Helper()
	ctx := context.Background()
	res, err := svc.Login(ctx, login+"@example.com", testPassword, "test")
	require.NoError(t, err)
	p, err := svc.AuthenticateSession(ctx, res.Token)
	require.NoError(t, err)
	return principal.With(ctx, p), res.Token
}
