package dbtest

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func insertID(t testing.TB, pool *pgxpool.Pool, sql string, args ...any) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	if err := pool.QueryRow(context.Background(), sql, args...).Scan(&id); err != nil {
		t.Fatalf("dbtest: %v", err)
	}
	return id
}

func exec(t testing.TB, pool *pgxpool.Pool, sql string, args ...any) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), sql, args...); err != nil {
		t.Fatalf("dbtest: %v", err)
	}
}

// Org inserts an organization.
func Org(t testing.TB, pool *pgxpool.Pool) uuid.UUID {
	t.Helper()
	return insertID(t, pool, "INSERT INTO organization (name) VALUES ('test') RETURNING id")
}

// User inserts a user and its membership in orgID with the given role.
func User(t testing.TB, pool *pgxpool.Pool, orgID uuid.UUID, role string) uuid.UUID {
	t.Helper()
	h := strings.ReplaceAll(uuid.NewString(), "-", "")
	id := insertID(t, pool,
		"INSERT INTO users (email, login, name, password_hash) VALUES ($1, $2, 'Test', 'x') RETURNING id",
		h+"@example.com", h)
	exec(t, pool, "INSERT INTO membership (org_id, user_id, role) VALUES ($1, $2, $3)", orgID, id, role)
	return id
}

// Team inserts a team in orgID with the given members (who must belong to orgID).
func Team(t testing.TB, pool *pgxpool.Pool, orgID uuid.UUID, members ...uuid.UUID) uuid.UUID {
	t.Helper()
	id := insertID(t, pool, "INSERT INTO team (org_id, name) VALUES ($1, $2) RETURNING id", orgID, uuid.NewString())
	for _, m := range members {
		exec(t, pool, "INSERT INTO team_member (team_id, org_id, user_id) VALUES ($1, $2, $3)", id, orgID, m)
	}
	return id
}

// Resource inserts an ssh/servers resource with the given labels.
func Resource(t testing.TB, pool *pgxpool.Pool, orgID uuid.UUID, name string, labels map[string]string) uuid.UUID {
	t.Helper()
	id := insertID(t, pool,
		"INSERT INTO resource (org_id, name, type, category, config) VALUES ($1, $2, 'ssh', 'servers', '{}') RETURNING id",
		orgID, name)
	for k, v := range labels {
		exec(t, pool, "INSERT INTO resource_label (resource_id, org_id, key, value) VALUES ($1, $2, $3, $4)", id, orgID, k, v)
	}
	return id
}
