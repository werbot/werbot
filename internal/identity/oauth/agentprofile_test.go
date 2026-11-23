package oauth_test

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/identity/oauth"
	"github.com/werbot/werbot/internal/platform/apperr"
	"github.com/werbot/werbot/internal/platform/db/dbtest"
)

// newProfile creates a profile of agent for the member, its sponsor.
func newProfile(t *testing.T, s stand, agent uuid.UUID) (oauth.AgentProfile, string) {
	t.Helper()
	prof, token, err := s.p.CreateAgentProfile(context.Background(), s.fx.Principal(s.fx.MemberID), agent, 0, "build-box")
	require.NoError(t, err)
	return prof, token
}

// TestCreateAgentProfile: the sponsor gets a profile bound to the agent and
// its refresh token once; the row is a werbot-cli session that ends with
// ttl; the audit event carries no token.
func TestCreateAgentProfile(t *testing.T) {
	s := newStand(t)
	ctx := context.Background()
	agent := dbtest.Agent(t, s.pool, s.fx.OrgID, s.fx.MemberID, "ops")

	prof, token, err := s.p.CreateAgentProfile(ctx, s.fx.Principal(s.fx.MemberID), agent, 2*time.Hour, "build-box\x00")
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(token, "wrt_"), token)
	assert.Equal(t, agent, prof.AgentID)
	assert.Equal(t, "ops", prof.AgentName)
	assert.Equal(t, s.fx.MemberID, prof.SponsorID)
	assert.Equal(t, "build-box", prof.Label, "control characters are dropped")
	assert.WithinDuration(t, time.Now().Add(2*time.Hour), prof.AbsoluteExpiresAt, 5*time.Second)
	assert.WithinDuration(t, prof.AbsoluteExpiresAt, prof.ExpiresAt, time.Second, "least(cli_session_ttl, ttl)")

	var (
		bound, user  uuid.UUID
		client, host string
		abs          time.Duration
	)
	require.NoError(t, s.pool.QueryRow(ctx, `SELECT agent_id, user_id, client_id, hostname, absolute_expires_at - now()
		FROM cli_session WHERE id = $1`, prof.ID).Scan(&bound, &user, &client, &host, &abs))
	assert.Equal(t, agent, bound)
	assert.Equal(t, s.fx.MemberID, user)
	assert.Equal(t, oauth.ClientCLI, client)
	assert.Equal(t, "build-box", host)
	assert.InDelta(t, (2 * time.Hour).Seconds(), abs.Seconds(), 5)

	var target, data string
	var actor uuid.UUID
	require.NoError(t, s.pool.QueryRow(ctx, `SELECT actor_user_id, target_type || ':' || target_id, data::text
		FROM audit_event WHERE action = 'agent.profile.create'`).Scan(&actor, &target, &data))
	assert.Equal(t, s.fx.MemberID, actor)
	assert.Equal(t, "agent_profile:"+prof.ID.String(), target)
	assert.Contains(t, data, `"agent_id": "`+agent.String()+`"`)
	assert.Contains(t, data, `"label": "build-box"`)
	assert.Contains(t, data, `"expires_at"`)
	var leaked bool
	require.NoError(t, s.pool.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM audit_event WHERE data::text LIKE '%wrt_%')").Scan(&leaked))
	assert.False(t, leaked, "no refresh token in the audit log")
}

// TestCreateAgentProfileLimits: at most MaxAgentProfiles live profiles per
// agent; ttl 0 is cli_session_max, otherwise 1h..cli_session_max; a frozen
// agent gets none.
func TestCreateAgentProfileLimits(t *testing.T) {
	s := newStand(t)
	ctx := context.Background()
	p := s.fx.Principal(s.fx.MemberID)
	agent := dbtest.Agent(t, s.pool, s.fx.OrgID, s.fx.MemberID, "ops")

	first, _ := newProfile(t, s, agent)
	assert.WithinDuration(t, time.Now().Add(30*24*time.Hour), first.AbsoluteExpiresAt, 5*time.Second, "ttl 0: cli_session_max")
	assert.WithinDuration(t, time.Now().Add(7*24*time.Hour), first.ExpiresAt, 5*time.Second, "idle: cli_session_ttl")
	for range oauth.MaxAgentProfiles - 1 {
		newProfile(t, s, agent)
	}
	_, token, err := s.p.CreateAgentProfile(ctx, p, agent, 0, "")
	require.ErrorIs(t, err, apperr.ErrFailedPrecondition)
	assert.ErrorContains(t, err, "agent has 5 live profiles; revoke one first")
	assert.Empty(t, token)
	require.NoError(t, s.p.RevokeAgentProfile(ctx, p, first.ID))
	newProfile(t, s, agent)

	other := dbtest.Agent(t, s.pool, s.fx.OrgID, s.fx.MemberID, "other")
	for name, ttl := range map[string]time.Duration{"below 1h": 30 * time.Minute, "above cli_session_max": 31 * 24 * time.Hour, "negative": -time.Hour} {
		_, _, err := s.p.CreateAgentProfile(ctx, p, other, ttl, "")
		assert.ErrorIs(t, err, apperr.ErrInvalidArgument, name)
	}
	prof, _, err := s.p.CreateAgentProfile(ctx, p, other, oauth.AgentProfileMinTTL, "")
	require.NoError(t, err, "exactly 1h")
	assert.WithinDuration(t, time.Now().Add(time.Hour), prof.AbsoluteExpiresAt, 5*time.Second)

	frozen := dbtest.Agent(t, s.pool, s.fx.OrgID, s.fx.MemberID, "frozen")
	exec(t, s, "UPDATE agent SET status = 'frozen' WHERE id = $1", frozen)
	_, _, err = s.p.CreateAgentProfile(ctx, p, frozen, 0, "")
	require.ErrorIs(t, err, apperr.ErrFailedPrecondition)
	assert.ErrorContains(t, err, "agent is frozen")
}

// TestAgentProfileRoles: only the sponsor creates a profile (owner and admin
// too get NotFound for another member's agent); members list and revoke
// their own, owner and admin any; an agent token does nothing.
func TestAgentProfileRoles(t *testing.T) {
	s := newStand(t)
	ctx := context.Background()
	member, other, admin := s.fx.Principal(s.fx.MemberID), s.fx.Principal(s.fx.OtherMemberID), s.fx.Principal(s.fx.AdminID)
	agent := dbtest.Agent(t, s.pool, s.fx.OrgID, s.fx.MemberID, "ops")
	foreign := dbtest.Agent(t, s.pool, s.fx.OrgID, s.fx.OtherMemberID, "ops")

	prof, _ := newProfile(t, s, agent)
	for name, p := range map[string]uuid.UUID{"another member": s.fx.OtherMemberID, "admin": s.fx.AdminID, "owner": s.fx.OwnerID} {
		_, _, err := s.p.CreateAgentProfile(ctx, s.fx.Principal(p), agent, 0, "")
		assert.ErrorIs(t, err, apperr.ErrNotFound, name)
	}
	_, _, err := s.p.CreateAgentProfile(ctx, member, uuid.New(), 0, "")
	assert.ErrorIs(t, err, apperr.ErrNotFound, "no such agent")
	otherProf, _, err := s.p.CreateAgentProfile(ctx, other, foreign, 0, "")
	require.NoError(t, err)

	ids := func(p []oauth.AgentProfile) []uuid.UUID {
		out := make([]uuid.UUID, len(p))
		for i, x := range p {
			out[i] = x.ID
		}
		return out
	}
	list, err := s.p.ListAgentProfiles(ctx, member, nil)
	require.NoError(t, err)
	assert.Equal(t, []uuid.UUID{prof.ID}, ids(list), "a member sees own profiles")
	assert.Equal(t, "ops", list[0].AgentName)
	assert.Equal(t, "build-box", list[0].Label)
	list, err = s.p.ListAgentProfiles(ctx, admin, nil)
	require.NoError(t, err)
	assert.Equal(t, []uuid.UUID{otherProf.ID, prof.ID}, ids(list), "an admin sees all, newest first")
	list, err = s.p.ListAgentProfiles(ctx, admin, &agent)
	require.NoError(t, err)
	assert.Equal(t, []uuid.UUID{prof.ID}, ids(list), "filtered by agent")

	assert.ErrorIs(t, s.p.RevokeAgentProfile(ctx, other, prof.ID), apperr.ErrNotFound, "another member's profile")
	require.NoError(t, s.p.RevokeAgentProfile(ctx, admin, prof.ID))
	assert.ErrorIs(t, s.p.RevokeAgentProfile(ctx, admin, prof.ID), apperr.ErrNotFound, "already revoked")
	var data string
	require.NoError(t, s.pool.QueryRow(ctx, "SELECT data::text FROM audit_event WHERE action = 'agent.profile.revoke'").Scan(&data))
	assert.JSONEq(t, `{"agent_id":"`+agent.String()+`","reason":"revoked"}`, data)

	run := uuid.New()
	tokenPrincipal := member
	tokenPrincipal.AgentID, tokenPrincipal.RunID = &agent, &run
	_, _, err = s.p.CreateAgentProfile(ctx, tokenPrincipal, agent, 0, "")
	assert.ErrorIs(t, err, apperr.ErrPermissionDenied)
	_, err = s.p.ListAgentProfiles(ctx, tokenPrincipal, nil)
	assert.ErrorIs(t, err, apperr.ErrPermissionDenied)
	assert.ErrorIs(t, s.p.RevokeAgentProfile(ctx, tokenPrincipal, otherProf.ID), apperr.ErrPermissionDenied)
}

// TestCreateAgentProfileLimitConcurrent: concurrent creates never pass
// MaxAgentProfiles live profiles of one agent.
func TestCreateAgentProfileLimitConcurrent(t *testing.T) {
	s := newStand(t)
	agent := dbtest.Agent(t, s.pool, s.fx.OrgID, s.fx.MemberID, "ops")
	var wg sync.WaitGroup
	for range 2 * oauth.MaxAgentProfiles {
		wg.Go(func() {
			_, _, _ = s.p.CreateAgentProfile(context.Background(), s.fx.Principal(s.fx.MemberID), agent, 0, "")
		})
	}
	wg.Wait()
	assert.Equal(t, oauth.MaxAgentProfiles, countSessions(t, s))
}
