package app

import (
	"log/slog"
	"net/netip"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	postgresv1 "github.com/werbot/werbot/gen/werbot/connector/postgres/v1"
	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
	"github.com/werbot/werbot/internal/gateway"
	"github.com/werbot/werbot/internal/inventory"
	"github.com/werbot/werbot/internal/platform/db/dbtest"
	"github.com/werbot/werbot/internal/platform/metrics"
	"github.com/werbot/werbot/internal/policy"
)

// TestPolicyParityQuery decides the postgres.query and postgres.connect
// requests of a person and of an agent on the server (policy.PolicySet) and
// on the gateway (CompilePolicy of the pool snapshot) from one database:
// Allow, Reasons, Obligations and Message must be equal and as expected.
// The snapshot edit sets agent_readonly, which the server reads from the
// credential status from Task 6 on.
func TestPolicyParityQuery(t *testing.T) {
	s := newStage4(t, slog.New(slog.DiscardHandler))
	ctx, org, user := s.org(t, "owner")
	p := agentParity{parity: parity{s: s, org: org, user: user, gw: dbtest.Gateway(t, s.pool, org, "gw"),
		snaps: newTunnelDeps(s.d, metrics.NewRegistry(), slog.New(slog.DiscardHandler)).Snapshots}, agent: uuid.New(), run: uuid.New()}
	pg, err := s.d.Inventory.Create(ctx, org, inventory.CreateInput{Name: "srv", Type: "postgres", GatewayID: &p.gw,
		Config: &postgresv1.Config{Host: "db.internal", Username: "app", Database: "app"}})
	require.NoError(t, err)
	p.srv = pg.ID
	s.bind(ctx, t, &werbotv1.CreateBindingRequest{SubjectType: "user", SubjectId: user.String(), ResourceId: p.srv.String()})
	sel := policy.Selector{ResourceIDs: []uuid.UUID{p.srv}}
	rule := func(subjects policy.Subjects, effect, message string, classes ...string) {
		_, err := s.d.Policy.CreateCommandRule(ctx, org, policy.CommandRuleInput{Name: effect + uuid.NewString()[:8], Effect: effect,
			Message: message, Subjects: subjects, Selector: sel, Classes: classes})
		require.NoError(t, err)
	}
	people, agents := policy.Subjects{AllUsers: true}, policy.Subjects{AllAgents: true}
	readonly := func(on bool) func(*policy.Snapshot) {
		return func(s *policy.Snapshot) { s.Resources[0].AgentReadonly = on }
	}
	permitAll := func(s *policy.Snapshot) {
		readonly(false)(s)
		s.CustomPolicies = append(s.CustomPolicies, policy.SnapshotCustomPolicy{ID: uuid.New(), Cedar: `permit (principal, action, resource);`})
	}
	cases := []struct {
		name        string
		agent       bool
		action      string // "" is postgres.query
		classes     []string
		writes      []string
		granted     bool
		setup       func()
		edit        func(*policy.Snapshot)
		allow       bool
		reason      string
		obligations []string
		message     string
	}{
		{name: "person read", classes: []string{"read"}, allow: true},
		{name: "person connect", action: "postgres.connect", allow: true},
		{name: "person deny", classes: []string{"destructive"}, message: "No drops",
			setup: func() { rule(people, "deny", "No drops", "destructive") }},
		{name: "person opaque under a destructive rule", classes: []string{"opaque"}, allow: true,
			setup: func() { rule(people, "deny", "No drops", "destructive") }},
		{name: "person approval", classes: []string{"write"}, obligations: []string{policy.ObligationApproval}, message: "Needs an OK",
			setup: func() { rule(people, "approval", "Needs an OK", "write") }},
		{name: "person approval granted", classes: []string{"write"}, granted: true, allow: true,
			setup: func() { rule(people, "approval", "Needs an OK", "write") }},
		{name: "person write_tables", classes: []string{"write"}, writes: []string{"public.orders"}, edit: func(s *policy.Snapshot) {
			s.CustomPolicies = append(s.CustomPolicies, policy.SnapshotCustomPolicy{ID: uuid.New(),
				Cedar: `@message("No orders") forbid (principal, action, resource) when { context.write_tables.contains("public.orders") };`})
		}, message: "No orders"},
		{name: "agent connect", agent: true, action: "postgres.connect", allow: true, reason: "agent:baseline:db_connect", edit: readonly(true)},
		{name: "agent read", agent: true, classes: []string{"read"}, allow: true, reason: "agent:baseline:db_read", edit: readonly(true)},
		{name: "agent write", agent: true, classes: []string{"write"}, edit: readonly(true)},
		{name: "agent destructive", agent: true, classes: []string{"destructive"}, reason: "agent:baseline:dangerous", edit: readonly(true)},
		{name: "agent approval", agent: true, classes: []string{"write"}, obligations: []string{policy.ObligationApproval}, message: "Needs an OK",
			setup: func() { rule(agents, "approval", "Needs an OK", "write") }, edit: readonly(true)},
		{name: "agent approval granted", agent: true, classes: []string{"write"}, granted: true, allow: true,
			setup: func() { rule(agents, "approval", "Needs an OK", "write") }, edit: readonly(true)},
		{name: "agent unverified connect", agent: true, action: "postgres.connect", reason: "agent:baseline:db_readonly", edit: readonly(false)},
		{name: "agent unverified read", agent: true, classes: []string{"read"}, reason: "agent:baseline:db_readonly", edit: readonly(false)},
		{name: "agent unverified custom permit", agent: true, classes: []string{"read"}, reason: "agent:baseline:db_readonly", edit: permitAll},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := s.pool.Exec(ctx, "DELETE FROM command_rule WHERE org_id = $1", org)
			require.NoError(t, err)
			if tc.setup != nil {
				tc.setup()
			}
			ps, pol, teams := p.compiledWith(ctx, t, tc.edit)
			server, gw := p.decideQuery(t, ps, pol, teams, tc.agent, tc.action, tc.classes, tc.writes, tc.granted)
			requireSame(t, server, gw, tc.name)
			assert.Equal(t, tc.allow, server.Allow, "%v", server.Reasons)
			assert.Equal(t, tc.obligations, server.Obligations)
			assert.Equal(t, tc.message, server.Message)
			if tc.reason != "" {
				assert.Contains(t, server.Reasons, tc.reason)
			}
		})
	}
}

// decideQuery decides action (default postgres.query) on srv with classes
// and write tables in database app on both sides, for the agent of p when
// agent is set, else for its sponsor.
func (p agentParity) decideQuery(t *testing.T, ps *policy.PolicySet, pol *gateway.Policy, teams []uuid.UUID, agent bool,
	action string, classes, writes []string, granted bool) (policy.Decision, gateway.Decision) {
	t.Helper()
	r, ok := pol.Resource("srv")
	require.True(t, ok)
	ip := netip.MustParseAddr("10.1.2.3")
	req := gateway.AccessRequest{User: gateway.User{ID: p.user, TeamIDs: teams}, Resource: r, IP: ip, Time: monday10,
		Action: action, Kind: gateway.KindStream, ApprovalGranted: granted}
	sreq := policy.Request{UserID: p.user, TeamIDs: teams, ResourceID: r.ID, Action: "postgres.connect", IP: ip, Time: monday10,
		Kind: string(gateway.KindStream), ApprovalGranted: granted}
	if action == "" {
		req.Action, req.Command, req.Classes, req.DBName, req.WriteTables = "postgres.query", "sql", classes, "app", writes
		sreq.Action, sreq.Command, sreq.Classes, sreq.DBName, sreq.WriteTables = req.Action, req.Command, classes, "app", writes
	}
	if agent {
		req.User.Agent = &gateway.Agent{ID: p.agent, RunID: p.run, AuthAt: monday10.Add(-time.Hour)}
		sreq.AgentID, sreq.RunID = &p.agent, &p.run
	}
	return ps.Authorize(sreq), pol.Decide(req)
}
