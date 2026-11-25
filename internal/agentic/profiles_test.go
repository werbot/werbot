package agentic_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/agentic"
	"github.com/werbot/werbot/internal/platform/db/dbtest"
)

// fakeProfiles.RevokeAgentProfiles is agentic.Config.RevokeProfiles: it
// records each call and fails with err.
type fakeProfiles struct {
	calls []string // agent id and reason
	actor uuid.UUID
	inTx  bool
	err   error
}

func (f *fakeProfiles) RevokeAgentProfiles(ctx context.Context, tx pgx.Tx, _, agentID, actor uuid.UUID, reason string) error {
	var status string
	f.inTx = tx.QueryRow(ctx, "SELECT status FROM agent WHERE id = $1", agentID).Scan(&status) == nil && status == "frozen"
	f.calls = append(f.calls, agentID.String()+" "+reason)
	f.actor = actor
	return f.err
}

// TestUpdateAgentFreezeRevokesProfiles: freezing an agent revokes its
// profiles in the same transaction (reason agent_frozen, the caller as
// actor); other updates do not; a failed revoke keeps the agent active.
func TestUpdateAgentFreezeRevokesProfiles(t *testing.T) {
	pool, _ := dbtest.New(t)
	f := dbtest.Seed(t, pool)
	profiles := &fakeProfiles{}
	svc := agentic.New(pool, &dbtest.Mails{}, agentic.Config{RevokeProfiles: profiles.RevokeAgentProfiles})
	p := f.Principal(f.MemberID)
	agent := dbtest.Agent(t, pool, f.OrgID, f.MemberID, "ops")

	_, err := svc.UpdateAgent(bg, p, agentic.UpdateAgentInput{ID: agent, Limits: &agentic.Limits{MaxActions: 5}})
	require.NoError(t, err)
	assert.Empty(t, profiles.calls, "a limits change keeps profiles")

	profiles.err = errors.New("store down")
	_, err = svc.UpdateAgent(bg, p, agentic.UpdateAgentInput{ID: agent, Status: "frozen"})
	require.Error(t, err)
	var status string
	require.NoError(t, pool.QueryRow(bg, "SELECT status FROM agent WHERE id = $1", agent).Scan(&status))
	assert.Equal(t, "active", status, "the freeze rolls back with the revoke")

	profiles.err, profiles.calls = nil, nil
	_, err = svc.UpdateAgent(bg, p, agentic.UpdateAgentInput{ID: agent, Status: "frozen"})
	require.NoError(t, err)
	assert.Equal(t, []string{agent.String() + " agent_frozen"}, profiles.calls)
	assert.True(t, profiles.inTx, "called inside the freezing transaction")
	assert.Equal(t, f.MemberID, profiles.actor)

	_, err = svc.UpdateAgent(bg, p, agentic.UpdateAgentInput{ID: agent, Status: "active"})
	require.NoError(t, err)
	assert.Len(t, profiles.calls, 1, "unfreezing restores nothing")
}
