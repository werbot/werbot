package app

import (
	"context"
	"crypto/rand"
	"errors"
	"log/slog"
	"net/netip"
	"testing"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/werbot/werbot/internal/platform/apperr"
	"github.com/werbot/werbot/internal/platform/db/dbtest"
	"github.com/werbot/werbot/internal/platform/metrics"
	"github.com/werbot/werbot/internal/platform/sshca"
	"github.com/werbot/werbot/internal/tunnel"
)

// agentStage is a stage-4 stack with a seeded organization and the tunnel adapters.
type agentStage struct {
	stage4
	f dbtest.Fixture
	d tunnel.Deps
}

func newAgentStage(t *testing.T) agentStage {
	t.Helper()
	s := newStage4(t, slog.New(slog.DiscardHandler))
	return agentStage{stage4: s, f: dbtest.Seed(t, s.pool), d: newTunnelDeps(s.d, metrics.NewRegistry(), slog.New(slog.DiscardHandler))}
}

// run inserts an agent of sponsor and a running run of it with maxActions.
func (s agentStage) run(t *testing.T, sponsor uuid.UUID, name string, maxActions int) (agent, run uuid.UUID) {
	t.Helper()
	agent = dbtest.Agent(t, s.pool, s.f.OrgID, sponsor, name)
	return agent, dbtest.AgentRun(t, s.pool, s.f.OrgID, agent, maxActions, 4)
}

func (s agentStage) exec(t *testing.T, sql string, args ...any) {
	t.Helper()
	_, err := s.pool.Exec(context.Background(), sql, args...)
	require.NoError(t, err)
}

// certIdentity is a live CLI session of user with its login, as a certificate names it.
func (s agentStage) certIdentity(t *testing.T, user uuid.UUID) sshca.Identity {
	t.Helper()
	hash := make([]byte, 32)
	_, _ = rand.Read(hash)
	id := sshca.Identity{OrgID: s.f.OrgID, UserID: user}
	ctx := context.Background()
	require.NoError(t, s.pool.QueryRow(ctx, `INSERT INTO cli_session (org_id, user_id, client_id, refresh_hash, expires_at, absolute_expires_at)
		VALUES ($1, $2, 'werbot-cli', $3, now() + interval '1 hour', now() + interval '1 day') RETURNING id`, s.f.OrgID, user, hash).Scan(&id.CLISessionID))
	require.NoError(t, s.pool.QueryRow(ctx, "SELECT login FROM users WHERE id = $1", user).Scan(&id.Login))
	return id
}

// An agent certificate authenticates only while its run may act, and only as
// the run's own agent and sponsor; a person's certificate has no agent.
func TestAuthenticateUserAgentCert(t *testing.T) {
	s := newAgentStage(t)
	agent, run := s.run(t, s.f.MemberID, "a1", 10)
	other, _ := s.run(t, s.f.MemberID, "a2", 10)
	id := s.certIdentity(t, s.f.MemberID)
	byCert := func(agentID, runID uuid.UUID) (tunnel.User, error) {
		c := id
		c.AgentID, c.RunID = &agentID, &runID
		return s.d.Users.ByCert(context.Background(), s.f.OrgID, c)
	}

	u, err := byCert(agent, run)
	require.NoError(t, err)
	assert.Equal(t, s.f.MemberID, u.ID)
	assert.Equal(t, &tunnel.AgentRef{AgentID: agent, RunID: run}, u.Agent)

	_, err = byCert(other, run)
	require.ErrorIs(t, err, apperr.ErrUnauthenticated, "another agent's certificate")
	_, err = byCert(agent, uuid.New())
	require.ErrorIs(t, err, apperr.ErrNotFound, "an unknown run")

	s.exec(t, "UPDATE agent_run SET status = 'ended', ended_at = now(), end_reason = 'client_exit' WHERE id = $1", run)
	_, err = byCert(agent, run)
	require.Error(t, err, "an ended run")

	u, err = s.d.Users.ByCert(context.Background(), s.f.OrgID, id)
	require.NoError(t, err)
	assert.Nil(t, u.Agent, "a person's certificate")
}

// startExec starts an exec session of user, of the agent run when set.
func (s agentStage) startExec(user uuid.UUID, run *uuid.UUID) (uuid.UUID, error) {
	return s.d.Sessions.Start(context.Background(), tunnel.SessionStart{OrgID: s.f.OrgID, GatewayID: s.f.GatewayID,
		InstanceID: s.f.InstanceID, UserID: user, ResourceID: s.f.ResourceID, ClientIP: netip.MustParseAddr("10.0.0.1"),
		Kind: "exec", Command: "ls", AgentRunID: run})
}

// sessionsOf counts the sessions of run.
func (s agentStage) sessionsOf(t *testing.T, run uuid.UUID) (n int) {
	t.Helper()
	require.NoError(t, s.pool.QueryRow(context.Background(), "SELECT count(*) FROM session WHERE agent_run_id = $1", run).Scan(&n))
	return n
}

// An agent's session counts an action of its run in the start transaction; a
// refused action leaves no session: the limit and an inactive run are
// FailedPrecondition with a fixed text, another user's or organization's run a denial.
func TestStartSessionCountsAction(t *testing.T) {
	s := newAgentStage(t)
	_, run := s.run(t, s.f.MemberID, "a1", 1)
	_, err := s.startExec(s.f.MemberID, &run)
	require.NoError(t, err)
	var actions int
	require.NoError(t, s.pool.QueryRow(context.Background(), "SELECT actions FROM agent_run WHERE id = $1", run).Scan(&actions))
	assert.Equal(t, 1, actions)

	_, err = s.startExec(s.f.MemberID, &run)
	assertConnect(t, err, connect.CodeFailedPrecondition, "agent limit exceeded")
	assert.Equal(t, 1, s.sessionsOf(t, run), "no session past the limit")

	_, paused := s.run(t, s.f.MemberID, "a2", 10)
	s.exec(t, "UPDATE agent_run SET status = 'paused' WHERE id = $1", paused)
	_, err = s.startExec(s.f.MemberID, &paused)
	assertConnect(t, err, connect.CodeFailedPrecondition, "agent run is not active")

	_, err = s.startExec(s.f.OtherMemberID, &paused)
	require.ErrorIs(t, err, apperr.ErrPermissionDenied, "another user's run")
	foreign := uuid.New()
	_, err = s.startExec(s.f.MemberID, &foreign)
	require.ErrorIs(t, err, apperr.ErrNotFound, "a run of no organization")
	assert.Zero(t, s.sessionsOf(t, paused))

	_, err = s.startExec(s.f.MemberID, nil)
	require.NoError(t, err, "a person's session counts nothing")
}

func assertConnect(t *testing.T, err error, code connect.Code, msg string) {
	t.Helper()
	var ce *connect.Error
	require.True(t, errors.As(err, &ce), "connect error: %v", err)
	assert.Equal(t, code, ce.Code())
	assert.Equal(t, msg, ce.Message())
}

// RecordDenial pauses the run at the fifth denial in a row and audits each as
// session.deny; a run of another organization is NotFound.
func TestRecordDenialRPC(t *testing.T) {
	s := newAgentStage(t)
	_, run := s.run(t, s.f.MemberID, "a1", 10)
	d := tunnel.AgentDenial{RunID: run, Resource: "web", Command: "rm -rf /", Classes: []string{"destructive"},
		Reasons: []string{"agent:baseline:dangerous"}}
	for i := range 5 {
		paused, err := s.d.Agents.RecordDenial(context.Background(), s.f.OrgID, d)
		require.NoError(t, err)
		assert.Equal(t, i == 4, paused, "denial %d", i+1)
	}
	var n int
	require.NoError(t, s.pool.QueryRow(context.Background(), `SELECT count(*) FROM audit_event
		WHERE action = 'session.deny' AND data->>'agent_run_id' = $1`, run.String()).Scan(&n))
	assert.Equal(t, 5, n)

	_, err := s.d.Agents.RecordDenial(context.Background(), dbtest.Org(t, s.pool), d)
	require.ErrorIs(t, err, apperr.ErrNotFound)
}
