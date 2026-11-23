package app

import (
	"net/netip"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/identity"
	"github.com/werbot/werbot/internal/platform/apperr"
	"github.com/werbot/werbot/internal/platform/db/dbtest"
	"github.com/werbot/werbot/internal/session"
	"github.com/werbot/werbot/internal/tunnel"
)

// A web ticket resolves to its member with teams and the ticket's mode,
// resource, session, window and browser address, only for the organization
// and instance it was issued to and only while it lives; a removed member's
// ticket resolves to nothing.
func TestTunnelUsersByWebTicket(t *testing.T) {
	pool, _ := dbtest.New(t)
	ctx := t.Context()
	a, b := dbtest.Seed(t, pool), dbtest.Seed(t, pool)
	a.OnlineInstance(t, pool, a.InstanceID, "ssh")
	team := dbtest.Team(t, pool, a.OrgID, a.MemberID)
	sessions := session.New(pool)
	users := tunnelUsers{ids: identity.New(pool, nil, identity.Config{}), sessions: sessions}

	web, err := sessions.CreateTerminalTicket(ctx, a.Principal(a.MemberID), "web", 120, 40, uuid.New())
	require.NoError(t, err)
	_, err = pool.Exec(ctx, "UPDATE stream_ticket SET client_ip = '203.0.113.7' WHERE id = $1", web.ID)
	require.NoError(t, err)
	u, wt, err := users.ByWebTicket(ctx, a.OrgID, a.InstanceID, web.ID)
	require.NoError(t, err)
	assert.Equal(t, a.MemberID, u.ID)
	assert.NotEmpty(t, u.Login)
	assert.Equal(t, []uuid.UUID{team}, u.TeamIDs)
	assert.Equal(t, tunnel.WebTicket{Mode: "web", Resource: "web", Cols: 120, Rows: 40, ClientIP: netip.MustParseAddr("203.0.113.7")}, wt)

	live := a.StartSession(t, pool, a.MemberID)
	observe, err := sessions.CreateObserveTicket(ctx, a.Principal(a.OwnerID), live, uuid.New())
	require.NoError(t, err)
	u, wt, err = users.ByWebTicket(ctx, a.OrgID, a.InstanceID, observe.ID)
	require.NoError(t, err)
	assert.Equal(t, a.OwnerID, u.ID)
	assert.Equal(t, tunnel.WebTicket{Mode: "observe", SessionID: &live}, wt)

	for name, c := range map[string][3]uuid.UUID{
		"another organization": {b.OrgID, a.InstanceID, web.ID},
		"another instance":     {a.OrgID, a.OtherInstanceID, web.ID},
		"unknown ticket":       {a.OrgID, a.InstanceID, uuid.New()},
	} {
		_, _, err := users.ByWebTicket(ctx, c[0], c[1], c[2])
		require.ErrorIs(t, err, apperr.ErrNotFound, name)
	}

	_, err = pool.Exec(ctx, "UPDATE stream_ticket SET expires_at = now() - interval '1 second' WHERE id = $1", web.ID)
	require.NoError(t, err)
	_, _, err = users.ByWebTicket(ctx, a.OrgID, a.InstanceID, web.ID)
	require.ErrorIs(t, err, apperr.ErrNotFound, "expired")

	_, err = pool.Exec(ctx, "DELETE FROM membership WHERE org_id = $1 AND user_id = $2", a.OrgID, a.OwnerID)
	require.NoError(t, err)
	_, _, err = users.ByWebTicket(ctx, a.OrgID, a.InstanceID, observe.ID)
	require.ErrorIs(t, err, apperr.ErrNotFound, "removed member")
}
