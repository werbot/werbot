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

func TestLookupSSHKeyAndLogin(t *testing.T) {
	svc, pool, _ := newTestService(t)
	org1, org2 := seedOrg(t, pool, "One"), seedOrg(t, pool, "Two")
	alice := seedUser(t, pool, org1, "alice", principal.RoleOwner)
	ctx, _ := loginCtx(t, svc, "alice")
	line, _ := newEd25519(t, "")
	k, err := svc.AddSSHKey(ctx, "k", line)
	require.NoError(t, err)

	bg := context.Background()
	want := UserRef{UserID: alice, Login: "alice", Role: "owner"}
	got, err := svc.LookupSSHKey(bg, org1, k.FingerprintSHA256)
	require.NoError(t, err)
	assert.Equal(t, want, got)
	_, err = svc.LookupSSHKey(bg, org2, k.FingerprintSHA256)
	assert.ErrorIs(t, err, apperr.ErrNotFound)

	got, err = svc.LookupLogin(bg, org1, "alice")
	require.NoError(t, err)
	assert.Equal(t, want, got)
	_, err = svc.LookupLogin(bg, org2, "alice")
	assert.ErrorIs(t, err, apperr.ErrNotFound)
}

// The lookup returns the member's teams in the organization, sorted, as
// UserTeams does, and nil when the member is in no team.
func TestLookupCarriesTeams(t *testing.T) {
	svc, pool, _ := newTestService(t)
	org1, org2 := seedOrg(t, pool, "One"), seedOrg(t, pool, "Two")
	alice := seedUser(t, pool, org1, "alice", principal.RoleOwner)
	ctx, _ := loginCtx(t, svc, "alice")
	line, _ := newEd25519(t, "")
	k, err := svc.AddSSHKey(ctx, "k", line)
	require.NoError(t, err)
	bg := context.Background()

	got, err := svc.LookupSSHKey(bg, org1, k.FingerprintSHA256)
	require.NoError(t, err)
	assert.Nil(t, got.TeamIDs)

	team := func(org uuid.UUID, name string) uuid.UUID {
		var id uuid.UUID
		require.NoError(t, pool.QueryRow(bg, "INSERT INTO team (org_id, name) VALUES ($1, $2) RETURNING id", org, name).Scan(&id))
		_, err := pool.Exec(bg, "INSERT INTO team_member (team_id, org_id, user_id) VALUES ($1, $2, $3)", id, org, alice)
		require.NoError(t, err)
		return id
	}
	_, err = pool.Exec(bg, "INSERT INTO membership (org_id, user_id, role) VALUES ($1, $2, 'member')", org2, alice)
	require.NoError(t, err)
	team(org2, "other")
	want := []uuid.UUID{team(org1, "b"), team(org1, "a")}
	slices.SortFunc(want, func(a, b uuid.UUID) int { return strings.Compare(a.String(), b.String()) })

	got, err = svc.LookupSSHKey(bg, org1, k.FingerprintSHA256)
	require.NoError(t, err)
	assert.Equal(t, want, got.TeamIDs)
	got, err = svc.LookupLogin(bg, org1, "alice")
	require.NoError(t, err)
	assert.Equal(t, want, got.TeamIDs)
	teams, err := svc.UserTeams(bg, org1, alice)
	require.NoError(t, err)
	assert.Equal(t, teams, got.TeamIDs)
}
