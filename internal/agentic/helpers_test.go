package agentic_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/agentic"
	"github.com/werbot/werbot/internal/platform/db/dbtest"
)

var bg = context.Background()

// newTestService seeds a fixture and returns a Service with fake emails.
func newTestService(t *testing.T) (*agentic.Service, *pgxpool.Pool, dbtest.Fixture) {
	t.Helper()
	pool, _ := dbtest.New(t)
	return agentic.New(pool, &dbtest.Mails{}, agentic.Config{WebURL: "https://werbot.test"}), pool, dbtest.Seed(t, pool)
}

// event is one audit_event row.
type event struct {
	Actor     *uuid.UUID
	ActorKind string
	TargetID  string
	Data      map[string]any
}

// events lists the audit events of action in insertion order.
func events(t *testing.T, pool *pgxpool.Pool, action string) []event {
	t.Helper()
	rows, err := pool.Query(bg,
		"SELECT actor_user_id, actor_kind, target_id, data FROM audit_event WHERE action = $1 ORDER BY id", action)
	require.NoError(t, err)
	out, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (event, error) {
		var e event
		var raw []byte
		if err := r.Scan(&e.Actor, &e.ActorKind, &e.TargetID, &raw); err != nil {
			return e, err
		}
		return e, json.Unmarshal(raw, &e.Data)
	})
	require.NoError(t, err)
	return out
}

// exec runs sql in the test database.
func exec(t *testing.T, pool *pgxpool.Pool, sql string, args ...any) {
	t.Helper()
	_, err := pool.Exec(bg, sql, args...)
	require.NoError(t, err)
}

// runState is the status, denial counter and end reason of a run.
type runState struct {
	Status       string
	DenialsInRow int
	EndReason    *string
	Ended        bool
}

func stateOf(t *testing.T, pool *pgxpool.Pool, runID uuid.UUID) runState {
	t.Helper()
	var s runState
	require.NoError(t, pool.QueryRow(bg,
		"SELECT status, denials_in_row, end_reason, ended_at IS NOT NULL FROM agent_run WHERE id = $1", runID).
		Scan(&s.Status, &s.DenialsInRow, &s.EndReason, &s.Ended))
	return s
}

// pause sets run to paused with denials denials in a row.
func pause(t *testing.T, pool *pgxpool.Pool, runID uuid.UUID, denials int) {
	t.Helper()
	exec(t, pool, "UPDATE agent_run SET status = 'paused', denials_in_row = $2 WHERE id = $1", runID, denials)
}

// end sets run to ended ago (a Postgres interval text) with reason client_exit.
func end(t *testing.T, pool *pgxpool.Pool, runID uuid.UUID, ago string) {
	t.Helper()
	exec(t, pool, "UPDATE agent_run SET status = 'ended', ended_at = now() - $2::interval, end_reason = 'client_exit' WHERE id = $1",
		runID, ago)
}
