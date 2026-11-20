package app

import (
	"context"
	"log/slog"
	"net/netip"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	sshv1 "github.com/werbot/werbot/gen/werbot/connector/ssh/v1"
	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
	"github.com/werbot/werbot/internal/gateway"
	"github.com/werbot/werbot/internal/inventory"
	"github.com/werbot/werbot/internal/platform/db/dbtest"
	"github.com/werbot/werbot/internal/platform/metrics"
	"github.com/werbot/werbot/internal/policy"
)

// TestPolicyParityExec decides exec requests on the server (policy.PolicySet)
// and on the gateway (CompilePolicy of the tunnel snapshot) from one database:
// classes, Allow, Reasons, Obligations and Message must be equal and as
// expected. The connect request of every scenario must agree too: it carries
// an empty command and classes, so exec rules never touch it.
func TestPolicyParityExec(t *testing.T) {
	s := newStage4(t, slog.New(slog.DiscardHandler))
	ctx, org, user := s.org(t, "owner")
	p := parity{s: s, org: org, user: user, gw: dbtest.Gateway(t, s.pool, org, "gw"),
		snaps: newTunnelDeps(s.d, metrics.NewRegistry(), slog.New(slog.DiscardHandler)).Snapshots}
	team := dbtest.Team(t, s.pool, org, user)
	srv, err := s.d.Inventory.Create(ctx, org, inventory.CreateInput{Name: "srv", Type: "ssh", GatewayID: &p.gw,
		Config: &sshv1.Config{Host: "db.internal", Login: "root"}})
	require.NoError(t, err)
	p.srv = srv.ID
	s.bind(ctx, t, &werbotv1.CreateBindingRequest{SubjectType: "team", SubjectId: team.String(), ResourceId: p.srv.String()})
	_, err = s.d.Policy.CreateClassRule(ctx, org, "./deploy.sh", []string{"write"})
	require.NoError(t, err)

	rule := func(effect, message string, classes ...string) {
		_, err := s.d.Policy.CreateCommandRule(ctx, org, policy.CommandRuleInput{Name: effect + uuid.NewString()[:8], Effect: effect,
			Message: message, Subjects: policy.Subjects{Teams: []uuid.UUID{team}},
			Selector: policy.Selector{ResourceIDs: []uuid.UUID{p.srv}}, Classes: classes})
		require.NoError(t, err)
	}
	custom := func(text string) {
		_, err := s.d.Policy.CreateCustomPolicy(ctx, org, policy.CustomPolicyInput{Name: "c" + uuid.NewString()[:8], Cedar: text, Enabled: true})
		require.NoError(t, err)
	}
	cases := []struct {
		name    string
		setup   func()
		command string
		granted bool
		want    policy.Decision // Reasons are compared between the sides only
	}{
		{name: "allow", command: "ls -l /tmp", want: policy.Decision{Allow: true}},
		{name: "resource host is not egress", setup: func() { rule("deny", "", "egress") }, command: "curl http://db.internal/health",
			want: policy.Decision{Allow: true}},
		{name: "deny with message", setup: func() { rule("deny", "No deploys", "write") }, command: "./deploy.sh",
			want: policy.Decision{Message: "No deploys"}},
		{name: "approval", setup: func() { rule("approval", "Needs an OK", "write") }, command: "./deploy.sh",
			want: policy.Decision{Obligations: []string{policy.ObligationApproval}, Message: "Needs an OK"}},
		{name: "approval granted", setup: func() { rule("approval", "Needs an OK", "write") }, command: "./deploy.sh", granted: true,
			want: policy.Decision{Allow: true}},
		{name: "approval and deny", setup: func() { rule("approval", "", "write"); rule("deny", "blocked", "write") }, command: "./deploy.sh",
			want: policy.Decision{Message: "blocked"}},
		{name: "custom forbid", setup: func() {
			custom(`@message("no egress") forbid (principal, action == Werbot::Action::"ssh.exec", resource) when { context.classes.contains("egress") };`)
		}, command: "curl https://example.com", want: policy.Decision{Message: "no egress"}},
		{name: "custom eval error", setup: func() {
			// passes the validator, fails at evaluation; stored directly since the API trial evaluation refuses it
			_, err := s.pool.Exec(ctx, "INSERT INTO custom_policy (org_id, name, cedar) VALUES ($1, 'overflow', $2)", org,
				`forbid (principal, action == Werbot::Action::"ssh.exec", resource) when { context.hour + 9223372036854775807 > 0 };`)
			require.NoError(t, err)
		}, command: "ls", want: policy.Decision{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for _, table := range []string{"command_rule", "custom_policy"} {
				_, err := s.pool.Exec(ctx, "DELETE FROM "+table+" WHERE org_id = $1", org)
				require.NoError(t, err)
			}
			if tc.setup != nil {
				tc.setup()
			}
			for _, action := range []string{"ssh.connect", "ssh.exec"} {
				server, gw := p.decideExec(ctx, t, action, tc.command, tc.granted)
				assert.Equal(t, server.Allow, gw.Allow, action)
				assert.Equal(t, server.Obligations, gw.Obligations, action)
				assert.Equal(t, server.Message, gw.Message, action)
				assert.ElementsMatch(t, server.Reasons, gw.Reasons, action)
				if action == "ssh.connect" {
					assert.True(t, server.Allow, "connect: %v", server.Reasons)
					continue
				}
				server.Reasons = nil
				assert.Equal(t, tc.want, server, "exec")
			}
		})
	}
}

// decideExec asks both sides for action on srv with command (connect: empty
// command and classes). The gateway classes must equal those of the server's
// TestCommand, which classifies with the same class rules and resource host.
func (p parity) decideExec(ctx context.Context, t *testing.T, action, command string, granted bool) (policy.Decision, gateway.Decision) {
	t.Helper()
	ip := netip.MustParseAddr("10.1.2.3")
	teams, err := p.s.d.Identity.UserTeams(ctx, p.org, p.user)
	require.NoError(t, err)
	snap, err := p.s.d.Policy.Snapshot(ctx, p.org)
	require.NoError(t, err)
	ps, _, err := policy.Compile(snap)
	require.NoError(t, err)

	gsnap, err := buildSnapshot(ctx, p.snaps, p.org, p.gw)
	require.NoError(t, err)
	pol, err := gateway.CompilePolicy(gsnap)
	require.NoError(t, err)
	r, ok := pol.Resource("srv")
	require.True(t, ok)

	req := gateway.AccessRequest{User: gateway.User{ID: p.user, TeamIDs: teams}, Resource: r, IP: ip, Time: monday10,
		Action: action, ApprovalGranted: granted}
	if action == "ssh.exec" {
		ep, _, err := r.Endpoint()
		require.NoError(t, err)
		req.Command, req.Classes = command, pol.Classify(command, ep.Host)
		res, err := p.s.d.Policy.TestCommand(ctx, p.org, p.user, p.srv, command, ip, monday10)
		require.NoError(t, err)
		require.Equal(t, res.Classes, req.Classes, "classes")
	}
	server := ps.Authorize(policy.Request{UserID: p.user, TeamIDs: teams, ResourceID: p.srv, Action: action, IP: ip, Time: monday10,
		Command: req.Command, Classes: req.Classes, ApprovalGranted: granted})
	return server, pol.Decide(req)
}
