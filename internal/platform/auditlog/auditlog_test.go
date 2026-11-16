package auditlog_test

import (
	"context"
	"errors"
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

func TestWriteInsertsEvent(t *testing.T) {
	pool, _ := dbtest.New(t)
	ctx := context.Background()
	org, u := uuid.New(), uuid.New()
	_, err := pool.Exec(ctx, "INSERT INTO organization (id, name) VALUES ($1, 'acme')", org)
	require.NoError(t, err)

	require.NoError(t, dbx.InOrg(ctx, pool, org, u, func(tx pgx.Tx) error {
		return auditlog.Write(ctx, tx, auditlog.Event{
			OrgID: &org, ActorUserID: &u, ActorKind: "user", Action: "member.invite",
			TargetType: "user", TargetID: "42",
			IP:   netip.MustParseAddr("203.0.113.7"),
			Data: map[string]any{"role": "admin"},
		})
	}))

	var ip, role, action string
	require.NoError(t, pool.QueryRow(ctx,
		"SELECT host(ip), data->>'role', action FROM audit_event WHERE org_id = $1", org).Scan(&ip, &role, &action))
	assert.Equal(t, "203.0.113.7", ip)
	assert.Equal(t, "admin", role)
	assert.Equal(t, "member.invite", action)
}

func TestWriteRollsBackWithTx(t *testing.T) {
	pool, _ := dbtest.New(t)
	ctx := context.Background()
	org := uuid.New()
	_, err := pool.Exec(ctx, "INSERT INTO organization (id, name) VALUES ($1, 'acme')", org)
	require.NoError(t, err)

	err = dbx.InOrg(ctx, pool, org, uuid.Nil, func(tx pgx.Tx) error {
		require.NoError(t, auditlog.Write(ctx, tx, auditlog.Event{OrgID: &org, ActorKind: "system", Action: "x.y"}))
		return errors.New("boom")
	})
	require.EqualError(t, err, "boom")

	var n int
	require.NoError(t, pool.QueryRow(ctx, "SELECT count(*) FROM audit_event").Scan(&n))
	assert.Zero(t, n)
}

func TestWriteDefaults(t *testing.T) {
	pool, _ := dbtest.New(t)
	ctx := context.Background()
	require.NoError(t, dbx.System(ctx, pool, dbx.ReasonSetup, func(tx pgx.Tx) error {
		return auditlog.Write(ctx, tx, auditlog.Event{ActorKind: "system", Action: "setup.done"})
	}))

	var orgNull, ipNull, dataEmpty bool
	require.NoError(t, pool.QueryRow(ctx,
		"SELECT org_id IS NULL, ip IS NULL, data = '{}' FROM audit_event").Scan(&orgNull, &ipNull, &dataEmpty))
	assert.True(t, orgNull)
	assert.True(t, ipNull)
	assert.True(t, dataEmpty)
}

func TestWriteRejectsInvalid(t *testing.T) {
	pool, _ := dbtest.New(t)
	ctx := context.Background()
	for name, e := range map[string]auditlog.Event{
		"actor kind": {ActorKind: "robot", Action: "x.y"},
		"action":     {ActorKind: "user", Action: ""},
	} {
		require.NoError(t, dbx.System(ctx, pool, dbx.ReasonSetup, func(tx pgx.Tx) error {
			assert.Error(t, auditlog.Write(ctx, tx, e), name)
			return nil
		}))
	}
	var n int
	require.NoError(t, pool.QueryRow(ctx, "SELECT count(*) FROM audit_event").Scan(&n))
	assert.Zero(t, n)
}
