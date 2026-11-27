package gateway

import (
	"encoding/json"
	"net/netip"
	"testing"
	"time"

	"github.com/cedar-policy/cedar-go"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	gatewayv1 "github.com/werbot/werbot/gen/gateway/v1"
	_ "github.com/werbot/werbot/internal/connector/postgres" // registers "postgres" with its query action
)

// queryPolicies permit everything and forbid by the query context keys.
const queryPolicies = `
permit (principal, action, resource);
forbid (principal, action, resource) when { context.write_tables.contains("public.orders") };
forbid (principal, action, resource) when { context.db_name == "billing" || context.tables.contains("billing.cards") };`

func TestDecideQueryContext(t *testing.T) {
	m := newIDs("u1", "pg")
	p := compileTest(t, m, queryPolicies, nil, res{"pg", "pg", "postgres"})
	r, ok := p.Resource("pg")
	require.True(t, ok)
	ip := netip.MustParseAddr("10.1.2.3")
	query := func(db string, tables, writes []string) AccessRequest {
		return AccessRequest{User: User{ID: m["u1"]}, Resource: r, IP: ip, Action: "postgres.query", Kind: KindStream,
			Command: "sql", Classes: []string{"read"}, DBName: db, Tables: tables, WriteTables: writes}
	}
	assert.False(t, p.Decide(query("app", nil, []string{"public.orders"})).Allow)
	assert.True(t, p.Decide(query("app", nil, []string{"public.items"})).Allow, "another table")
	assert.False(t, p.Decide(query("app", []string{"billing.cards"}, nil)).Allow)
	assert.False(t, p.Decide(query("billing", nil, nil)).Allow, "db_name")

	d := p.Decide(AccessRequest{User: User{ID: m["u1"]}, Resource: r, IP: ip, Kind: KindStream, Port: 5432})
	assert.True(t, d.Allow, "a connect evaluates the query keys without error: %v", d.Reasons)
}

// TestAgentReadonlyMissingAttributeDeniedGateway: a snapshot whose resource
// entity lacks agent_readonly (Ruling C6) denies agents by the db_readonly
// forbid itself, never by a skipped erroring policy.
func TestAgentReadonlyMissingAttributeDeniedGateway(t *testing.T) {
	m := newIDs("u1", "pg")
	baseline := `permit (principal, action, resource);
@id("agent:baseline:db_readonly")
forbid (principal is Werbot::Agent, action in [Werbot::Action::"postgres.connect", Werbot::Action::"postgres.query"], resource)
unless { resource has agent_readonly && resource.agent_readonly };`
	s := testSnapshot(t, m, baseline, nil, res{"pg", "pg", "postgres"})
	p, err := CompilePolicy(s)
	require.NoError(t, err)
	r, ok := p.Resource("pg")
	require.True(t, ok)
	now := time.Now()
	agent := User{ID: m["u1"], Agent: &Agent{ID: uuid.New(), RunID: uuid.New(), AuthAt: now}}
	for name, req := range map[string]AccessRequest{
		"connect": {User: agent, Resource: r, IP: netip.MustParseAddr("10.1.2.3"), Time: now, Kind: KindStream, Port: 5432},
		"query":   {User: agent, Resource: r, IP: netip.MustParseAddr("10.1.2.3"), Time: now, Kind: KindStream, Action: "postgres.query", Classes: []string{"read"}},
	} {
		d := p.Decide(req)
		assert.False(t, d.Allow, name)
		assert.Contains(t, d.Reasons, "agent:baseline:db_readonly", name)
		assert.NotContains(t, d.Reasons, "error:agent:baseline:db_readonly", name)
	}

	verified := withReadonly(t, s, m["pg"])
	p, err = CompilePolicy(verified)
	require.NoError(t, err)
	d := p.Decide(AccessRequest{User: agent, Resource: r, IP: netip.MustParseAddr("10.1.2.3"), Time: now, Kind: KindStream, Port: 5432})
	assert.True(t, d.Allow, "a verified credential lifts the forbid: %v", d.Reasons)
}

// withReadonly is s with agent_readonly true on the entity of id.
func withReadonly(t *testing.T, s *gatewayv1.Snapshot, id uuid.UUID) *gatewayv1.Snapshot {
	t.Helper()
	var ents cedar.EntityMap
	require.NoError(t, json.Unmarshal(s.GetEntitiesJson(), &ents))
	uid := cedar.NewEntityUID("Werbot::Resource", cedar.String(id.String()))
	e := ents[uid]
	e.Attributes = cedar.NewRecord(cedar.RecordMap{"name": cedar.String("pg"), "type": cedar.String("postgres"), "agent_readonly": cedar.True})
	ents[uid] = e
	out := &gatewayv1.Snapshot{CedarPolicies: s.GetCedarPolicies(), Resources: s.GetResources(), Version: s.GetVersion()}
	var err error
	out.EntitiesJson, err = json.Marshal(ents)
	require.NoError(t, err)
	return out
}
