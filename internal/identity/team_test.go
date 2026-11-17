package identity

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/platform/apperr"
	"github.com/werbot/werbot/internal/platform/principal"
)

func TestTeamLifecycle(t *testing.T) {
	svc, pool, _ := newTestService(t)
	org := seedOrg(t, pool, "Acme")
	seedUser(t, pool, org, "alice", principal.RoleOwner)
	bob := seedUser(t, pool, org, "bob", principal.RoleMember)
	ctx, _ := loginCtx(t, svc, "alice")

	team, err := svc.CreateTeam(ctx, "ops")
	require.NoError(t, err)
	assert.Equal(t, "ops", team.Name)
	_, err = svc.CreateTeam(ctx, "ops")
	assert.ErrorIs(t, err, apperr.ErrAlreadyExists)
	_, err = svc.CreateTeam(ctx, " ")
	assert.ErrorIs(t, err, apperr.ErrInvalidArgument)

	renamed, err := svc.RenameTeam(ctx, team.ID, "sre")
	require.NoError(t, err)
	assert.Equal(t, "sre", renamed.Name)

	require.NoError(t, svc.AddTeamMember(ctx, team.ID, bob))
	assert.ErrorIs(t, svc.AddTeamMember(ctx, team.ID, bob), apperr.ErrAlreadyExists)
	teams, err := svc.ListTeams(ctx)
	require.NoError(t, err)
	require.Len(t, teams, 1)
	assert.Equal(t, []uuid.UUID{bob}, teams[0].MemberUserIDs)

	ids, err := svc.UserTeams(context.Background(), org, bob)
	require.NoError(t, err)
	assert.Equal(t, []uuid.UUID{team.ID}, ids)

	require.NoError(t, svc.RemoveTeamMember(ctx, team.ID, bob))
	assert.ErrorIs(t, svc.RemoveTeamMember(ctx, team.ID, bob), apperr.ErrNotFound)
	ids, err = svc.UserTeams(context.Background(), org, bob)
	require.NoError(t, err)
	assert.Empty(t, ids)

	require.NoError(t, svc.DeleteTeam(ctx, team.ID))
	assert.ErrorIs(t, svc.DeleteTeam(ctx, team.ID), apperr.ErrNotFound)
	teams, err = svc.ListTeams(ctx)
	require.NoError(t, err)
	assert.Empty(t, teams)

	assert.Equal(t, []string{"team.create", "team.rename", "team.member_add", "team.member_remove", "team.delete"}, auditActions(t, pool)[1:]) // [0] is auth.login
}

func TestMemberCannotManageTeams(t *testing.T) {
	svc, pool, _ := newTestService(t)
	org := seedOrg(t, pool, "Acme")
	seedUser(t, pool, org, "bob", principal.RoleMember)
	ctx, _ := loginCtx(t, svc, "bob")

	_, err := svc.ListTeams(ctx)
	assert.ErrorIs(t, err, apperr.ErrPermissionDenied)
	_, err = svc.CreateTeam(ctx, "ops")
	assert.ErrorIs(t, err, apperr.ErrPermissionDenied)
}

func TestCrossOrgIsolationTeams(t *testing.T) {
	svc, pool, _ := newTestService(t)
	org1, org2 := seedOrg(t, pool, "Acme"), seedOrg(t, pool, "Beta")
	alice := seedUser(t, pool, org1, "alice", principal.RoleOwner)
	zed := seedUser(t, pool, org2, "zed", principal.RoleOwner)
	ctx1, _ := loginCtx(t, svc, "alice")
	ctx2, _ := loginCtx(t, svc, "zed")

	other, err := svc.CreateTeam(ctx2, "ops")
	require.NoError(t, err)
	mine, err := svc.CreateTeam(ctx1, "ops")
	require.NoError(t, err)

	_, err = svc.RenameTeam(ctx1, other.ID, "x")
	assert.ErrorIs(t, err, apperr.ErrNotFound)
	assert.ErrorIs(t, svc.DeleteTeam(ctx1, other.ID), apperr.ErrNotFound)
	assert.ErrorIs(t, svc.AddTeamMember(ctx1, other.ID, alice), apperr.ErrNotFound)
	assert.ErrorIs(t, svc.AddTeamMember(ctx1, mine.ID, zed), apperr.ErrNotFound)
	assert.ErrorIs(t, svc.RemoveTeamMember(ctx1, other.ID, zed), apperr.ErrNotFound)

	teams, err := svc.ListTeams(ctx1)
	require.NoError(t, err)
	require.Len(t, teams, 1)
	assert.Equal(t, mine.ID, teams[0].ID)
}

func TestRemovedMemberLeavesTeams(t *testing.T) {
	svc, pool, _ := newTestService(t)
	org := seedOrg(t, pool, "Acme")
	seedUser(t, pool, org, "alice", principal.RoleOwner)
	bob := seedUser(t, pool, org, "bob", principal.RoleMember)
	ctx, _ := loginCtx(t, svc, "alice")

	team, err := svc.CreateTeam(ctx, "ops")
	require.NoError(t, err)
	require.NoError(t, svc.AddTeamMember(ctx, team.ID, bob))
	require.NoError(t, svc.RemoveMember(ctx, bob))

	ids, err := svc.UserTeams(context.Background(), org, bob)
	require.NoError(t, err)
	assert.Empty(t, ids)
	teams, err := svc.ListTeams(ctx)
	require.NoError(t, err)
	require.Len(t, teams, 1)
	assert.Empty(t, teams[0].MemberUserIDs)
}

func TestOrgMembers(t *testing.T) {
	svc, pool, _ := newTestService(t)
	org, other := seedOrg(t, pool, "Acme"), seedOrg(t, pool, "Beta")
	alice := seedUser(t, pool, org, "alice", principal.RoleOwner)
	bob := seedUser(t, pool, org, "bob", principal.RoleMember)
	seedUser(t, pool, other, "zed", principal.RoleOwner)
	ctx, _ := loginCtx(t, svc, "alice")
	ops, err := svc.CreateTeam(ctx, "ops")
	require.NoError(t, err)
	dev, err := svc.CreateTeam(ctx, "dev")
	require.NoError(t, err)
	for _, team := range []uuid.UUID{ops.ID, dev.ID} {
		require.NoError(t, svc.AddTeamMember(ctx, team, bob))
	}

	got, err := svc.OrgMembers(context.Background(), org)
	require.NoError(t, err)
	want := []OrgMember{{UserID: alice, TeamIDs: []uuid.UUID{}}, {UserID: bob, TeamIDs: sortedIDs(ops.ID, dev.ID)}}
	slices.SortFunc(want, func(a, b OrgMember) int { return strings.Compare(a.UserID.String(), b.UserID.String()) })
	assert.Equal(t, want, got, "members of org only, by user id, teams sorted")
}

func sortedIDs(ids ...uuid.UUID) []uuid.UUID {
	slices.SortFunc(ids, func(a, b uuid.UUID) int { return strings.Compare(a.String(), b.String()) })
	return ids
}
