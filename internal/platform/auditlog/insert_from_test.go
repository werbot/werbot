package auditlog_test

import (
	"context"
	"net/netip"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/platform/auditlog"
	"github.com/werbot/werbot/internal/platform/db/dbtest"
	"github.com/werbot/werbot/internal/platform/dbx"
)

// InsertFrom writes the row Write writes, once per source row, with the key added to data.
func TestInsertFromMatchesWrite(t *testing.T) {
	pool, _ := dbtest.New(t)
	ctx := context.Background()
	org, u := uuid.New(), uuid.New()
	_, err := pool.Exec(ctx, "INSERT INTO organization (id, name) VALUES ($1, 'acme')", org)
	require.NoError(t, err)
	e := auditlog.Event{OrgID: &org, ActorUserID: &u, ActorKind: "user", Action: "x.write", TargetType: "resource",
		TargetID: "r1", IP: netip.MustParseAddr("203.0.113.7"), Data: map[string]any{"kind": "k"}}

	require.NoError(t, dbx.InOrg(ctx, pool, org, u, func(tx pgx.Tx) error {
		if err := auditlog.Write(ctx, tx, e); err != nil {
			return err
		}
		e.Action = "x.from"
		sql, args, err := auditlog.InsertFrom(e, "src", "id", "row_id", 2)
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, "WITH src AS (SELECT unnest($1::text[]) AS id) "+sql, append([]any{[]string{"a", "b"}}, args...)...)
		return err
	}))

	rows, err := pool.Query(ctx, `SELECT action, org_id = $1 AND actor_user_id = $2 AND actor_kind = 'user' AND target_type = 'resource'
		AND target_id = 'r1' AND host(ip) = '203.0.113.7', data - 'row_id', data->>'row_id' FROM audit_event ORDER BY action, data->>'row_id'`, org, u)
	require.NoError(t, err)
	type row struct {
		action string
		same   bool
		data   map[string]any
		rowID  *string
	}
	got, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (row, error) {
		var x row
		return x, r.Scan(&x.action, &x.same, &x.data, &x.rowID)
	})
	require.NoError(t, err)
	a, b := "a", "b"
	want := map[string]any{"kind": "k"}
	assert.Equal(t, []row{{"x.from", true, want, &a}, {"x.from", true, want, &b}, {"x.write", true, want, nil}}, got)
}

func TestInsertFromRejectsInvalid(t *testing.T) {
	_, _, err := auditlog.InsertFrom(auditlog.Event{ActorKind: "robot", Action: "x.y"}, "s", "id", "k", 1)
	assert.Error(t, err)
}
