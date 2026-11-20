package access_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/access"
	"github.com/werbot/werbot/internal/platform/db/dbtest"
)

// mail is one queued email.
type mail struct {
	To, Template string
	Data         map[string]string
}

// fakeQueue collects the queued emails.
type fakeQueue struct {
	mu   sync.Mutex
	sent []mail
}

func (q *fakeQueue) EnqueueTx(_ context.Context, _ pgx.Tx, to, template string, data map[string]string) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.sent = append(q.sent, mail{to, template, data})
	return nil
}

// of returns the emails queued with template.
func (q *fakeQueue) of(template string) []mail {
	q.mu.Lock()
	defer q.mu.Unlock()
	var out []mail
	for _, m := range q.sent {
		if m.Template == template {
			out = append(out, m)
		}
	}
	return out
}

// to returns the recipients of template.
func (q *fakeQueue) to(template string) []string {
	var out []string
	for _, m := range q.of(template) {
		out = append(out, m.To)
	}
	return out
}

func (q *fakeQueue) reset() {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.sent = nil
}

// fakeSettings returns S, the defaults of migration 00124 unless a test changes them.
type fakeSettings struct{ S access.Settings }

func (f *fakeSettings) AccessSettings(context.Context, uuid.UUID) (access.Settings, error) {
	return f.S, nil
}

// newTestService seeds a fixture and returns a Service with fake emails and settings.
func newTestService(t *testing.T) (*access.Service, *pgxpool.Pool, dbtest.Fixture, *fakeQueue, *fakeSettings) {
	t.Helper()
	pool, _ := dbtest.New(t)
	q := &fakeQueue{}
	set := &fakeSettings{access.Settings{GrantDefaultTTL: 8 * time.Hour, GrantMaxTTL: 14 * 24 * time.Hour,
		RequestPendingTTL: time.Hour, CommandApprovalTTL: 5 * time.Minute}}
	return access.New(pool, q, set, access.Config{WebURL: "https://werbot.test"}), pool, dbtest.Seed(t, pool), q, set
}

// auditActions lists the audit actions in insertion order.
func auditActions(t *testing.T, pool *pgxpool.Pool) []string {
	t.Helper()
	rows, err := pool.Query(context.Background(), "SELECT action FROM audit_event ORDER BY id")
	require.NoError(t, err)
	out, err := pgx.CollectRows(rows, pgx.RowTo[string])
	require.NoError(t, err)
	return out
}

// listenOnce returns the payload of the first NOTIFY on channel.
func listenOnce(t *testing.T, pool *pgxpool.Pool, channel string) <-chan string {
	t.Helper()
	conn, err := pool.Acquire(context.Background())
	require.NoError(t, err)
	_, err = conn.Exec(context.Background(), "LISTEN "+pgx.Identifier{channel}.Sanitize())
	require.NoError(t, err)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	out := make(chan string, 1)
	go func() {
		defer conn.Release()
		defer cancel()
		if n, err := conn.Conn().WaitForNotification(ctx); err == nil {
			out <- n.Payload
		}
		close(out)
	}()
	t.Cleanup(cancel)
	return out
}

func emailOf(t *testing.T, pool *pgxpool.Pool, id uuid.UUID) string {
	t.Helper()
	var e string
	require.NoError(t, pool.QueryRow(context.Background(), "SELECT email FROM users WHERE id = $1", id).Scan(&e))
	return e
}

// setPolicy changes columns of a policy; set is trusted SQL of the test.
func setPolicy(t *testing.T, pool *pgxpool.Pool, id uuid.UUID, set string) {
	t.Helper()
	_, err := pool.Exec(context.Background(), "UPDATE approval_policy SET "+set+" WHERE id = $1", id)
	require.NoError(t, err)
}

// grants returns resource id → expires_at of the grants of approval id.
func grants(t *testing.T, pool *pgxpool.Pool, approvalID uuid.UUID) map[uuid.UUID]time.Time {
	t.Helper()
	rows, err := pool.Query(context.Background(), "SELECT resource_id, expires_at FROM access_grant WHERE approval_id = $1", approvalID)
	require.NoError(t, err)
	out := map[uuid.UUID]time.Time{}
	for rows.Next() {
		var r uuid.UUID
		var at time.Time
		require.NoError(t, rows.Scan(&r, &at))
		out[r] = at
	}
	require.NoError(t, rows.Err())
	return out
}

func status(t *testing.T, pool *pgxpool.Pool, id uuid.UUID) (s string) {
	t.Helper()
	require.NoError(t, pool.QueryRow(context.Background(), "SELECT status FROM approval WHERE id = $1", id).Scan(&s))
	return s
}

const (
	allUsers = `{"all_users": true}`
	noOne    = `{}`
)

// onResource is a resource_ids selector of id.
func onResource(id uuid.UUID) string { return `{"resource_ids": ["` + id.String() + `"]}` }
