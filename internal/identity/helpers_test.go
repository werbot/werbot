package identity

import (
	"context"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/platform/db/dbtest"
	"github.com/werbot/werbot/internal/platform/principal"
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

type sentEmail struct {
	To, Template string
	Data         map[string]string
}

// fakeQueue records enqueued emails instead of inserting River jobs.
type fakeQueue struct {
	mu   sync.Mutex
	sent []sentEmail
}

func (q *fakeQueue) EnqueueTx(_ context.Context, _ pgx.Tx, to, template string, data map[string]string) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.sent = append(q.sent, sentEmail{To: to, Template: template, Data: data})
	return nil
}

func newTestService(t *testing.T) (*Service, *pgxpool.Pool, *fakeQueue) {
	t.Helper()
	pool, _ := dbtest.New(t)
	q := &fakeQueue{}
	return New(pool, q, Config{WebURL: "https://werbot.test", SecureCookies: true}), pool, q
}

func seedOrg(t *testing.T, pool *pgxpool.Pool, name string) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	require.NoError(t, pool.QueryRow(context.Background(),
		"INSERT INTO organization (name) VALUES ($1) RETURNING id", name).Scan(&id))
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
	token, err := svc.Login(ctx, login+"@example.com", testPassword, "test")
	require.NoError(t, err)
	p, err := svc.AuthenticateSession(ctx, token)
	require.NoError(t, err)
	return principal.With(ctx, p), token
}

func auditActions(t *testing.T, pool *pgxpool.Pool) []string {
	t.Helper()
	rows, err := pool.Query(context.Background(), "SELECT action FROM audit_event ORDER BY created_at")
	require.NoError(t, err)
	actions, err := pgx.CollectRows(rows, pgx.RowTo[string])
	require.NoError(t, err)
	return actions
}
