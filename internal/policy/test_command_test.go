package policy

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	_ "github.com/werbot/werbot/internal/connector/tcp" // a type without an exec action
	"github.com/werbot/werbot/internal/platform/apperr"
	"github.com/werbot/werbot/internal/platform/db/dbtest"
)

func TestTestCommand(t *testing.T) {
	e := setup(t)
	r := e.resource(t, e.org)
	e.bind(e.org, e.owner, r)
	rule, err := e.svc.CreateCommandRule(e.ctx, e.org, noRm(r))
	require.NoError(t, err)
	now := time.Time{}

	got, err := e.svc.TestCommand(e.ctx, e.org, e.owner, r, "rm -rf /", "", ip10, now)
	require.NoError(t, err)
	assert.Equal(t, CommandResult{Classes: []string{"destructive"}, Paths: []string{"/**"},
		Decision: Decision{Reasons: []string{"command:" + rule.ID.String()}, Message: "No rm"}}, got)

	got, err = e.svc.TestCommand(e.ctx, e.org, e.owner, r, "ls", "", ip10, now)
	require.NoError(t, err)
	assert.Equal(t, []string{"read"}, got.Classes)
	assert.True(t, got.Allow)

	// Without a binding connect denies: the result is the connect decision.
	member := dbtest.User(t, e.pool, e.org, "member")
	got, err = e.svc.TestCommand(e.ctx, e.org, member, r, "ls", "", ip10, now)
	require.NoError(t, err)
	connect, err := e.svc.CheckAccess(e.ctx, e.org, member, r, ip10, now, 0)
	require.NoError(t, err)
	assert.Equal(t, CommandResult{Classes: []string{"read"}, Paths: []string{"~"}, Decision: connect}, got)
	assert.False(t, got.Allow)

	// A class rule and an approval rule: the deny asks for an approval.
	_, err = e.svc.CreateClassRule(e.ctx, e.org, "./deploy.sh", []string{"write"})
	require.NoError(t, err)
	approve := noRm(r)
	approve.Name, approve.Classes, approve.Effect, approve.Message = "review", []string{"write"}, "approval", ""
	_, err = e.svc.CreateCommandRule(e.ctx, e.org, approve)
	require.NoError(t, err)
	got, err = e.svc.TestCommand(e.ctx, e.org, e.owner, r, "./deploy.sh", "", ip10, now)
	require.NoError(t, err)
	assert.Equal(t, []string{"write"}, got.Classes)
	assert.False(t, got.Allow)
	assert.Equal(t, []string{"approval"}, got.Obligations)
}

func TestTestCommandRejects(t *testing.T) {
	e := setup(t)
	r := e.resource(t, e.org)
	orgB := dbtest.Org(t, e.pool)
	outsider := dbtest.User(t, e.pool, orgB, "owner")
	resB := e.resource(t, orgB)

	_, err := e.svc.TestCommand(e.ctx, e.org, e.owner, resB, "ls", "", ip10, time.Time{})
	assert.ErrorIs(t, err, apperr.ErrNotFound, "resource of another organization")
	_, err = e.svc.TestCommand(e.ctx, e.org, outsider, r, "ls", "", ip10, time.Time{})
	assert.ErrorIs(t, err, apperr.ErrNotFound, "user of another organization")
	_, err = e.svc.TestCommand(e.ctx, e.org, uuid.New(), r, "ls", "", ip10, time.Time{})
	assert.ErrorIs(t, err, apperr.ErrNotFound, "unknown user")

	e.res[e.org][0].Type = "tcp"
	_, err = e.svc.TestCommand(e.ctx, e.org, e.owner, r, "ls", "", ip10, time.Time{})
	require.ErrorIs(t, err, apperr.ErrFailedPrecondition)
	assert.Contains(t, err.Error(), "resource type has no exec action")
}
