package policy

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/cedar-policy/cedar-go"
	"github.com/cedar-policy/cedar-go/x/exp/schema"
	"github.com/cedar-policy/cedar-go/x/exp/schema/resolved"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
	"github.com/werbot/werbot/internal/platform/apperr"
)

// fakeAgents are the agents of each organization.
type fakeAgents map[uuid.UUID][]SnapshotAgent

func (f fakeAgents) PolicyAgents(_ context.Context, orgID uuid.UUID) ([]SnapshotAgent, []uuid.UUID, error) {
	return f[orgID], nil, nil
}

// agentSetup is setup(t) with agent a1 of the owner.
func agentSetup(t *testing.T) (env, uuid.UUID) {
	t.Helper()
	e := setup(t)
	a1 := uuid.New()
	e.svc = New(Deps{Pool: e.pool, Resources: e.res, Bindings: e.binds, Teams: noTeams{},
		Agents: fakeAgents{e.org: {{ID: a1, SponsorID: e.owner, Status: "active"}}}, Logger: slog.New(slog.DiscardHandler)})
	return e, a1
}

func TestSchemaAgent(t *testing.T) {
	var s schema.Schema
	require.NoError(t, s.UnmarshalCedar([]byte(Schema())))
	res, err := s.Resolve()
	require.NoError(t, err)
	agent, ok := res.Entities["Werbot::Agent"]
	require.True(t, ok)
	assert.Equal(t, resolved.EntityType("Werbot::User"), agent.Shape["sponsor"].Type)
	assert.Equal(t, resolved.StringType{}, agent.Shape["status"].Type)
	a := res.Actions[cedar.NewEntityUID("Werbot::Action", "ssh.exec")].AppliesTo
	assert.ElementsMatch(t, []cedar.EntityType{"Werbot::User", "Werbot::Agent"}, a.Principals)
	assert.Equal(t, resolved.StringType{}, a.Context["run_id"].Type)
	assert.Equal(t, resolved.StringType{}, a.Context["kind"].Type)
}

func TestTestCommandAgent(t *testing.T) {
	e, a1 := agentSetup(t)
	h := NewHandler(e.svc)
	r := e.resource(t, e.org)
	e.bind(e.org, e.owner, r)
	ctx := withClientIP(e.ctx, "10.1.2.3")
	probe := func(agent, command string) (*werbotv1.TestCommandResponse, error) {
		res, err := h.TestCommand(ctx, connect.NewRequest(&werbotv1.TestCommandRequest{
			Subject: &werbotv1.TestCommandRequest_AgentId{AgentId: agent}, ResourceId: r.String(), Command: command}))
		if err != nil {
			return nil, err
		}
		return res.Msg, nil
	}

	got, err := probe(a1.String(), "ls")
	require.NoError(t, err)
	assert.True(t, got.GetAllow())
	got, err = probe(a1.String(), "rm -rf /")
	require.NoError(t, err)
	assert.False(t, got.GetAllow())
	assert.Contains(t, got.GetReasons(), "agent:baseline:dangerous")
	_, err = probe(uuid.NewString(), "ls")
	assert.Equal(t, connect.CodeNotFound, connect.CodeOf(err), "unknown agent")
	_, err = h.TestCommand(ctx, connect.NewRequest(&werbotv1.TestCommandRequest{ResourceId: r.String(), Command: "ls"}))
	assert.Equal(t, connect.CodeInvalidArgument, connect.CodeOf(err), "no subject")

	only, err := e.svc.CreatePathRule(e.ctx, e.org, PathRuleInput{Name: "app", Subjects: Subjects{AllAgents: true},
		Selector: Selector{ResourceIDs: []uuid.UUID{r}}, Paths: []string{"~/app/**"}, Access: "any", Effect: "only"})
	require.NoError(t, err)
	got, err = probe(a1.String(), "cat app/x")
	require.NoError(t, err)
	assert.Equal(t, []string{"~/app/x"}, got.GetPaths())
	assert.Empty(t, got.GetPathOutside())
	assert.True(t, got.GetAllow())
	got, err = probe(a1.String(), "cat /tmp/x")
	require.NoError(t, err)
	assert.Equal(t, []string{only.ID.String()}, got.GetPathOutside())
	assert.False(t, got.GetAllow())
}

func TestTestCommandAgentUnknown(t *testing.T) {
	e, _ := agentSetup(t)
	r := e.resource(t, e.org)
	_, err := e.svc.TestCommandAgent(e.ctx, e.org, uuid.New(), r, "ls", "", ip10, time.Time{})
	assert.ErrorIs(t, err, apperr.ErrNotFound)
}

func TestAccessibleResourcesAgent(t *testing.T) {
	e, a1 := agentSetup(t)
	r1, r2 := e.resource(t, e.org), e.resource(t, e.org)
	e.bind(e.org, e.owner, r1)

	got, err := e.svc.AgentResources(e.ctx, e.org, a1, ip10, time.Time{})
	require.NoError(t, err)
	assert.Equal(t, map[uuid.UUID]ClassAccess{r1: {Allowed: []string{"read"}}}, got, "r2: the sponsor has no binding")
	assert.NotContains(t, got, r2)

	_, err = e.svc.CreateCommandRule(e.ctx, e.org, CommandRuleInput{Name: "writes", Subjects: Subjects{AllAgents: true},
		Selector: Selector{ResourceIDs: []uuid.UUID{r1}}, Classes: []string{"write"}, Effect: "approval"})
	require.NoError(t, err)
	got, err = e.svc.AgentResources(e.ctx, e.org, a1, ip10, time.Time{})
	require.NoError(t, err)
	assert.Equal(t, ClassAccess{Allowed: []string{"read"}, Approval: []string{"write"}}, got[r1])

	_, err = e.svc.AgentResources(e.ctx, e.org, uuid.New(), ip10, time.Time{})
	assert.ErrorIs(t, err, apperr.ErrNotFound, "unknown agent")
}
