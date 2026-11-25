package session_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/platform/apperr"
)

// A web terminal ticket carries the browser's cookie session (the web:
// binding of MFA) and dies with it; a principal without one stores none.
func TestTerminalTicketUserSession(t *testing.T) {
	e := newTicketEnv(t)
	ctx := t.Context()
	var sid uuid.UUID
	require.NoError(t, e.pool.QueryRow(ctx, `INSERT INTO user_session (token_hash, user_id, org_id, expires_at)
		VALUES ($1, $2, $3, now() + interval '1 hour') RETURNING id`, []byte(uuid.NewString()), e.a.MemberID, e.a.OrgID).Scan(&sid))
	p := e.a.Principal(e.a.MemberID)
	p.SessionID = sid

	tk, err := e.svc.CreateTerminalTicket(ctx, p, "web", 80, 24, uuid.New())
	require.NoError(t, err)
	assert.Equal(t, &sid, tk.UserSessionID)
	got, err := e.svc.LookupStreamTicket(ctx, e.a.OrgID, e.a.InstanceID, tk.ID)
	require.NoError(t, err)
	assert.Equal(t, &sid, got.UserSessionID)
	spent, err := e.svc.ConsumeStreamTicket(ctx, tk.ID)
	require.NoError(t, err)
	assert.Equal(t, &sid, spent.UserSessionID)
	assert.Nil(t, e.terminal(t, e.a.MemberID).UserSessionID, "no cookie session")

	_, err = e.pool.Exec(ctx, "DELETE FROM user_session WHERE id = $1", sid)
	require.NoError(t, err)
	_, err = e.svc.LookupStreamTicket(ctx, e.a.OrgID, e.a.InstanceID, tk.ID)
	require.ErrorIs(t, err, apperr.ErrNotFound, "the ticket goes with its session")
}
