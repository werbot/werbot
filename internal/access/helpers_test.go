package access_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/access"
	"github.com/werbot/werbot/internal/platform/db/dbtest"
)

// fakeSettings returns S, the defaults of migration 00124 unless a test changes them.
type fakeSettings struct{ S access.Settings }

func (f *fakeSettings) AccessSettings(context.Context, uuid.UUID) (access.Settings, error) {
	return f.S, nil
}

// newTestService seeds a fixture and returns a Service with fake emails and settings.
func newTestService(t *testing.T) (*access.Service, *pgxpool.Pool, dbtest.Fixture, *dbtest.Mails, *fakeSettings) {
	t.Helper()
	pool, _ := dbtest.New(t)
	q := &dbtest.Mails{}
	set := &fakeSettings{access.Settings{GrantDefaultTTL: 8 * time.Hour, GrantMaxTTL: 14 * 24 * time.Hour,
		RequestPendingTTL: time.Hour, CommandApprovalTTL: 5 * time.Minute}}
	return access.New(pool, q, set, access.Config{WebURL: "https://werbot.test"}), pool, dbtest.Seed(t, pool), q, set
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
