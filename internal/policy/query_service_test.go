package policy

import (
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
)

// dbResource inserts a resource of org and lists it in the fake source as a
// postgres resource of database "app" with secret tables billing.*.
func (e env) dbResource(t *testing.T, org uuid.UUID, readonly bool) uuid.UUID {
	t.Helper()
	id := e.resource(t, org)
	r := &e.res[org][len(e.res[org])-1]
	r.Type, r.Database, r.SecretTables, r.AgentReadonly = "postgres", "app", []string{"billing.*"}, readonly
	return id
}

func TestTestCommandSQL(t *testing.T) {
	e, a1 := agentSetup(t)
	h := NewHandler(e.svc)
	r := e.dbResource(t, e.org, true)
	e.bind(e.org, e.owner, r)
	rule, err := e.svc.CreateCommandRule(e.ctx, e.org, CommandRuleInput{Name: "no-drops", Subjects: Subjects{AllUsers: true},
		Selector: Selector{ResourceIDs: []uuid.UUID{r}}, Classes: []string{"destructive"}, Effect: "deny"})
	require.NoError(t, err)
	ctx := withClientIP(e.ctx, "192.0.2.10")
	probe := func(sub *werbotv1.TestCommandRequest, command string) *werbotv1.TestCommandResponse {
		t.Helper()
		sub.ResourceId, sub.Command = r.String(), command
		res, err := h.TestCommand(ctx, connect.NewRequest(sub))
		require.NoError(t, err)
		return res.Msg
	}
	user := func() *werbotv1.TestCommandRequest {
		return &werbotv1.TestCommandRequest{Subject: &werbotv1.TestCommandRequest_UserId{UserId: e.owner.String()}}
	}
	agent := func() *werbotv1.TestCommandRequest {
		return &werbotv1.TestCommandRequest{Subject: &werbotv1.TestCommandRequest_AgentId{AgentId: a1.String()}}
	}

	got := probe(user(), "DROP TABLE t")
	assert.Equal(t, []string{"destructive"}, got.GetClasses())
	assert.False(t, got.GetAllow())
	assert.Equal(t, []string{"command:" + rule.ID.String()}, got.GetReasons())
	assert.Empty(t, got.GetPaths(), "a query has no paths")

	got = probe(agent(), "SELECT 1")
	assert.True(t, got.GetAllow(), "%v", got.GetReasons())
	assert.Contains(t, got.GetReasons(), "agent:baseline:db_read")

	got = probe(user(), "SELECT * FROM billing.cards")
	assert.Equal(t, []string{"read", "secrets"}, got.GetClasses(), "secret_tables of the resource")

	got = probe(user(), "VACUUM t")
	assert.Equal(t, []string{"opaque"}, got.GetClasses(), "a text the parser does not classify")

	e.res[e.org][0].Type = "tcp"
	_, err = h.TestCommand(ctx, connect.NewRequest(&werbotv1.TestCommandRequest{
		Subject: &werbotv1.TestCommandRequest_UserId{UserId: e.owner.String()}, ResourceId: r.String(), Command: "SELECT 1"}))
	assert.Equal(t, connect.CodeFailedPrecondition, connect.CodeOf(err))
	assert.ErrorContains(t, err, "resource type has no exec or query action")
}

func TestTestCommandSQLDatabaseName(t *testing.T) {
	e := setup(t)
	r := e.dbResource(t, e.org, false)
	e.bind(e.org, e.owner, r)
	_, err := e.svc.CreateCustomPolicy(e.ctx, e.org, CustomPolicyInput{Name: "app db", Enabled: true,
		Cedar: `@message("not app") forbid (principal, action, resource) when { context.db_name == "app" && context.classes.contains("read") };`})
	require.NoError(t, err)
	got, err := e.svc.TestCommand(e.ctx, e.org, e.owner, r, "SELECT 1", "", ip10, time.Time{})
	require.NoError(t, err)
	assert.False(t, got.Allow, "the query runs in the resource database")
	assert.Equal(t, "not app", got.Message)
}

func TestAccessibleResourcesAgentDB(t *testing.T) {
	e, a1 := agentSetup(t)
	ro, rw := e.dbResource(t, e.org, true), e.dbResource(t, e.org, false)
	e.bind(e.org, e.owner, ro)
	e.bind(e.org, e.owner, rw)

	got, err := e.svc.AgentResources(e.ctx, e.org, a1, ip10, time.Time{})
	require.NoError(t, err)
	assert.Equal(t, map[uuid.UUID]ClassAccess{ro: {Allowed: []string{"read"}}}, got, "no verified credential: not listed")

	_, err = e.svc.CreateCommandRule(e.ctx, e.org, CommandRuleInput{Name: "writes", Subjects: Subjects{AllAgents: true},
		Selector: Selector{ResourceIDs: []uuid.UUID{ro, rw}}, Classes: []string{"write"}, Effect: "approval"})
	require.NoError(t, err)
	got, err = e.svc.AgentResources(e.ctx, e.org, a1, ip10, time.Time{})
	require.NoError(t, err)
	assert.Equal(t, map[uuid.UUID]ClassAccess{ro: {Allowed: []string{"read"}, Approval: []string{"write"}}}, got)
}
