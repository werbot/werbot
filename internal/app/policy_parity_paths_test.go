package app

import (
	"cmp"
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
	"github.com/werbot/werbot/internal/platform/shellclass"
	"github.com/werbot/werbot/internal/policy"
)

// TestPolicyParityPaths decides path and terminal rules on the server
// (TestCommand, policy.PolicySet) and on the gateway (CompilePolicy of the
// tunnel snapshot) from one database: the paths, the path rules hit or left,
// Allow, Reasons, Obligations and Message must be equal and as expected.
func TestPolicyParityPaths(t *testing.T) {
	s := newStage4(t, slog.New(slog.DiscardHandler))
	ctx, org, user := s.org(t, "owner")
	p := parity{s: s, org: org, user: user, gw: dbtest.Gateway(t, s.pool, org, "gw"),
		snaps: newTunnelDeps(s.d, metrics.NewRegistry(), slog.New(slog.DiscardHandler)).Snapshots}
	team := dbtest.Team(t, s.pool, org, user)
	srv, err := s.d.Inventory.Create(ctx, org, inventory.CreateInput{Name: "srv", Type: "ssh", GatewayID: &p.gw,
		Config: &sshv1.Config{Host: "db.internal", Login: "root"}})
	require.NoError(t, err)
	p.srv = srv.ID
	tel := dbtest.Resource(t, s.pool, org, "tel", nil)
	_, err = s.pool.Exec(ctx, "UPDATE resource SET gateway_id = $2, type = 'telnet' WHERE id = $1", tel, p.gw)
	require.NoError(t, err)
	for _, id := range []uuid.UUID{p.srv, tel} {
		s.bind(ctx, t, &werbotv1.CreateBindingRequest{SubjectType: "team", SubjectId: team.String(), ResourceId: id.String()})
	}
	target := func() (policy.Subjects, policy.Selector) {
		return policy.Subjects{Teams: []uuid.UUID{team}}, policy.Selector{ResourceIDs: []uuid.UUID{p.srv, tel}}
	}
	path := func(effect, message string, globs ...string) func() {
		return func() {
			subj, sel := target()
			_, err := s.d.Policy.CreatePathRule(ctx, org, policy.PathRuleInput{Name: effect + uuid.NewString()[:8], Subjects: subj, Selector: sel,
				Paths: globs, Access: "any", Effect: effect, Message: message})
			require.NoError(t, err)
		}
	}
	terminal := func(mode string) func() {
		return func() {
			subj, sel := target()
			_, err := s.d.Policy.CreateTerminalRule(ctx, org, policy.TerminalRuleInput{Name: mode + uuid.NewString()[:8], Subjects: subj, Selector: sel, Mode: mode})
			require.NoError(t, err)
		}
	}
	cases := []struct {
		name, command, cwd string
		setup              func()
		want               policy.Decision // exec; Reasons are compared between the sides only
		shell              policy.Decision // ssh.shell without the console; telnet.shell is compared only
	}{
		{name: "only inside", setup: path("only", "", "~/app/**"), command: "cat ~/app/a", want: allowed, shell: allowed},
		{name: "only outside", setup: path("only", "", "~/app/**"), command: "cp ~/app/a /etc/hosts",
			want: policy.Decision{Message: policy.PathOutsideMessage}, shell: allowed},
		{name: "unresolved", setup: path("only", "", "~/app/**"), command: "cat $F", want: policy.Decision{Message: policy.PathOutsideMessage}, shell: allowed},
		{name: "deny hit", setup: path("deny", "No /etc", "/etc/**"), command: "cat /etc/hosts", want: policy.Decision{Message: "No /etc"}, shell: allowed},
		{name: "deny from cwd", setup: path("deny", "No /srv", "/srv/**"), command: "cat x", cwd: "/srv", want: policy.Decision{Message: "No /srv"}, shell: allowed},
		{name: "approval", setup: path("approval", "Needs an OK", "/srv/**"), command: "cat /srv/a",
			want: policy.Decision{Obligations: []string{policy.ObligationApproval}, Message: "Needs an OK"}, shell: allowed},
		{name: "console", setup: terminal("console"), command: "ls", want: allowed,
			shell: policy.Decision{Obligations: []string{policy.ObligationConsole}}},
		{name: "terminal deny", setup: terminal("deny"), command: "ls", want: allowed, shell: policy.Decision{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for _, table := range []string{"path_rule", "terminal_rule"} {
				_, err := s.pool.Exec(ctx, "DELETE FROM "+table+" WHERE org_id = $1", org)
				require.NoError(t, err)
			}
			tc.setup()
			ps, pol, teams := p.compiled(ctx, t)
			server, gw := p.decidePaths(ctx, t, pol, teams, tc.command, tc.cwd)
			requireSame(t, server, gw, "exec")
			server.Reasons = nil
			assert.Equal(t, tc.want, server, "exec")

			for _, typ := range []string{"ssh", "telnet"} {
				for _, console := range []bool{false, true} {
					server, gw := p.decideShell(t, ps, pol, teams, typ, console)
					requireSame(t, server, gw, typ)
					if typ == "ssh" && !console {
						server.Reasons = nil
						assert.Equal(t, tc.shell, server, "shell")
					}
				}
			}
		})
	}
}

var allowed = policy.Decision{Allow: true}

func requireSame(t *testing.T, server policy.Decision, gw gateway.Decision, msg string) {
	t.Helper()
	assert.Equal(t, server.Allow, gw.Allow, msg)
	assert.Equal(t, server.Obligations, gw.Obligations, msg)
	assert.Equal(t, server.Message, gw.Message, msg)
	assert.Equal(t, server.Reasons, gw.Reasons, msg)
}

// compiled is the server PolicySet and the gateway Policy of the organization now.
func (p parity) compiled(ctx context.Context, t *testing.T) (*policy.PolicySet, *gateway.Policy, []uuid.UUID) {
	t.Helper()
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
	return ps, pol, teams
}

// decidePaths runs TestCommand on the server and analyses and decides the
// same command and cwd on the gateway: paths, hits and outside must be equal.
func (p parity) decidePaths(ctx context.Context, t *testing.T, pol *gateway.Policy, teams []uuid.UUID, command, cwd string) (policy.Decision, gateway.Decision) {
	t.Helper()
	ip := netip.MustParseAddr("10.1.2.3")
	res, err := p.s.d.Policy.TestCommand(ctx, p.org, p.user, p.srv, command, cwd, ip, monday10)
	require.NoError(t, err)
	r, ok := pol.Resource("srv")
	require.True(t, ok)
	ep, _, err := r.Endpoint()
	require.NoError(t, err)
	a := pol.Analyze(command, ep.Host, cmp.Or(cwd, "~"))
	req := gateway.AccessRequest{User: gateway.User{ID: p.user, TeamIDs: teams}, Resource: r, IP: ip, Time: monday10,
		Action: "ssh.exec", Command: command}
	for _, c := range a.Classes {
		req.Classes = append(req.Classes, string(c))
	}
	for _, ref := range a.Paths {
		req.Paths = append(req.Paths, ref.String())
	}
	req.PathHits, req.PathOutside = shellclass.EvalPaths(pol.PathRules(), a.Paths, a.Unresolved)
	assert.Equal(t, res.Classes, req.Classes, "classes")
	assert.Equal(t, res.Paths, req.Paths, "paths")
	assert.Equal(t, res.PathHits, req.PathHits, "path_hits")
	assert.Equal(t, res.PathOutside, req.PathOutside, "path_outside")
	return res.Decision, pol.Decide(req)
}

// decideShell decides the shell action of the resource of typ on both sides.
func (p parity) decideShell(t *testing.T, ps *policy.PolicySet, pol *gateway.Policy, teams []uuid.UUID, typ string, console bool) (policy.Decision, gateway.Decision) {
	t.Helper()
	name := map[string]string{"ssh": "srv", "telnet": "tel"}[typ]
	r, ok := pol.Resource(name)
	require.True(t, ok)
	ip := netip.MustParseAddr("10.1.2.3")
	server := ps.Authorize(policy.Request{UserID: p.user, TeamIDs: teams, ResourceID: r.ID, Action: typ + ".shell", IP: ip, Time: monday10, Console: console})
	gw := pol.Decide(gateway.AccessRequest{User: gateway.User{ID: p.user, TeamIDs: teams}, Resource: r, IP: ip, Time: monday10,
		Action: typ + ".shell", Console: console})
	return server, gw
}
