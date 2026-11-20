package access_test

import (
	"crypto/sha256"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/access"
	"github.com/werbot/werbot/internal/platform/apperr"
	"github.com/werbot/werbot/internal/platform/db/dbtest"
	"github.com/werbot/werbot/internal/platform/pgnotify"
)

const commandText = "psql -c 'drop table users' # marker-4711"

func commandInput(f dbtest.Fixture) access.CommandApprovalInput {
	return access.CommandApprovalInput{OrgID: f.OrgID, UserID: f.MemberID, ResourceID: f.ResourceID, InstanceID: f.InstanceID,
		Command: commandText, Classes: []string{"destructive"}, Reasons: []string{"drops a table"}}
}

func TestCommandApprovalNotify(t *testing.T) {
	svc, pool, f, q, _ := newTestService(t)
	a, err := svc.RequestCommandApproval(bg, commandInput(f))
	require.NoError(t, err)
	assert.Equal(t, access.StatusPending, a.Status)
	assert.Equal(t, access.KindCommand, a.Kind)
	assert.Equal(t, &f.InstanceID, a.InstanceID)
	sum := sha256.Sum256([]byte(commandText))
	assert.Equal(t, sum[:], a.CommandSHA256)
	assert.Equal(t, []string{"destructive"}, a.Classes)
	assert.WithinDuration(t, time.Now().Add(5*time.Minute), a.ExpiresAt, 5*time.Second)
	assert.ElementsMatch(t, []string{emailOf(t, pool, f.OwnerID), emailOf(t, pool, f.AdminID)}, q.to("access_request"),
		"no policy: owner and admin decide")
	m := q.of("access_request")[0]
	assert.Equal(t, commandText, m.Data["command"])
	assert.Equal(t, "5m", m.Data["duration"])

	var kind, actor string
	var data string
	require.NoError(t, pool.QueryRow(bg, "SELECT actor_kind, actor_user_id::text, data::text FROM audit_event WHERE action = 'access.request.create'").
		Scan(&kind, &actor, &data))
	assert.Equal(t, "gateway", kind)
	assert.Equal(t, f.MemberID.String(), actor)
	assert.JSONEq(t, `{"kind": "command", "classes": ["destructive"], "reasons": ["drops a table"]}`, data)
	assert.NotContains(t, data, "marker-4711", "no command text in audit")

	got, err := svc.GetApproval(bg, f.OrgID, a.ID)
	require.NoError(t, err)
	assert.Equal(t, a.ID, got.ID)
	_, err = svc.GetApproval(bg, dbtest.Org(t, pool), a.ID)
	assert.ErrorIs(t, err, apperr.ErrNotFound)

	ch := listenOnce(t, pool, pgnotify.ApprovalDecided)
	q.reset()
	_, err = svc.Decide(bg, f.Principal(f.OwnerID), a.ID, true, "")
	require.NoError(t, err)
	select {
	case payload := <-ch:
		assert.Equal(t, f.InstanceID.String()+":"+a.ID.String(), payload)
	case <-time.After(5 * time.Second):
		t.Fatal("no NOTIFY")
	}
	assert.Empty(t, grants(t, pool, a.ID), "a command approval grants nothing")
	assert.Empty(t, q.sent, "no decision email for a command")
	assert.NotContains(t, auditData(t, pool), "marker-4711")
}

func TestCommandApprovalInput(t *testing.T) {
	svc, pool, f, _, _ := newTestService(t)
	in := commandInput(f)
	in.Command = ""
	_, err := svc.RequestCommandApproval(bg, in)
	assert.ErrorIs(t, err, apperr.ErrInvalidArgument)
	in = commandInput(f)
	in.ResourceID = uuid.New()
	_, err = svc.RequestCommandApproval(bg, in)
	assert.ErrorIs(t, err, apperr.ErrNotFound)
	in = commandInput(f)
	in.UserID = dbtest.Seed(t, pool).MemberID
	_, err = svc.RequestCommandApproval(bg, in)
	assert.ErrorIs(t, err, apperr.ErrNotFound, "a user of another organization")
}

func TestCommandApprovalSponsorFourEyes(t *testing.T) {
	svc, pool, f, _, _ := newTestService(t)
	pol := f.ApprovalPolicy(t, pool, "agents", access.ModeManual, onResource(f.ResourceID), `{"all_agents": true}`,
		`{"users": ["`+f.MemberID.String()+`"]}`)
	in := commandInput(f)
	in.AgentRunID = new(uuid.New())
	a, err := svc.RequestCommandApproval(bg, in)
	require.NoError(t, err)
	got, err := svc.Decide(bg, f.Principal(f.MemberID), a.ID, true, "")
	require.NoError(t, err, "the sponsor approves the agent's command")
	assert.Equal(t, access.StatusApproved, got.Status)

	setPolicy(t, pool, pol, "four_eyes = true")
	_, err = svc.RequestCommandApproval(bg, in)
	require.ErrorIs(t, err, apperr.ErrFailedPrecondition, "the sponsor is the only approver")
	assert.ErrorContains(t, err, "no one can approve this request")

	setPolicy(t, pool, pol, `approvers = '{"users": ["`+f.MemberID.String()+`", "`+f.OtherMemberID.String()+`"]}'`)
	b, err := svc.RequestCommandApproval(bg, in)
	require.NoError(t, err)
	_, err = svc.Decide(bg, f.Principal(f.MemberID), b.ID, true, "")
	require.ErrorIs(t, err, apperr.ErrPermissionDenied)
	assert.ErrorContains(t, err, "you cannot decide this request")
	_, err = svc.Decide(bg, f.Principal(f.OtherMemberID), b.ID, true, "")
	require.NoError(t, err)
}

func TestDecidedCommands(t *testing.T) {
	svc, pool, f, _, _ := newTestService(t)
	decide := func(in access.CommandApprovalInput, approve bool) access.Approval {
		a, err := svc.RequestCommandApproval(bg, in)
		require.NoError(t, err)
		_, err = svc.Decide(bg, f.Principal(f.OwnerID), a.ID, approve, "")
		require.NoError(t, err)
		return a
	}
	approved := decide(commandInput(f), true)
	denied := decide(commandInput(f), false)
	old := decide(commandInput(f), true)
	_, err := pool.Exec(bg, "UPDATE approval SET decided_at = now() - interval '11 minutes' WHERE id = $1", old.ID)
	require.NoError(t, err)
	other := commandInput(f)
	other.InstanceID = f.OtherInstanceID
	decide(other, true)
	_, err = svc.RequestCommandApproval(bg, commandInput(f)) // pending
	require.NoError(t, err)
	expired, err := svc.RequestCommandApproval(bg, commandInput(f))
	require.NoError(t, err)
	_, err = pool.Exec(bg, "UPDATE approval SET status = 'expired', decided_at = now() WHERE id = $1", expired.ID)
	require.NoError(t, err)

	got, err := svc.DecidedCommands(bg, f.OrgID, f.InstanceID, time.Now().Add(-10*time.Minute))
	require.NoError(t, err)
	var ids []uuid.UUID
	for _, a := range got {
		ids = append(ids, a.ID)
	}
	assert.Equal(t, []uuid.UUID{approved.ID, denied.ID}, ids)
}

// auditData is every audit data object as text.
func auditData(t *testing.T, pool *pgxpool.Pool) string {
	t.Helper()
	var s string
	require.NoError(t, pool.QueryRow(bg, "SELECT coalesce(string_agg(data::text, ' '), '') FROM audit_event").Scan(&s))
	return s
}
