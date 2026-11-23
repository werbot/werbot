package app

import (
	"cmp"
	"context"
	"log/slog"
	"net/netip"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	gatewayv1 "github.com/werbot/werbot/gen/gateway/v1"
	sshv1 "github.com/werbot/werbot/gen/werbot/connector/ssh/v1"
	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
	"github.com/werbot/werbot/internal/gateway"
	"github.com/werbot/werbot/internal/inventory"
	"github.com/werbot/werbot/internal/platform/db/dbtest"
	"github.com/werbot/werbot/internal/platform/metrics"
	"github.com/werbot/werbot/internal/platform/shellclass"
	"github.com/werbot/werbot/internal/policy"
)

// agentParity is one agent run of the parity user on the srv resource.
type agentParity struct {
	parity
	agent, run uuid.UUID
}

// TestPolicyParityAgents decides the requests of an agent on the server
// (policy.PolicySet) and on the gateway (CompilePolicy of the snapshot the
// tunnel builds) from one database, with the agents, blocked runs and locks
// the agent source would add: Allow, Reasons, Obligations and Message must be
// equal and as expected, for the exec and its connect.
func TestPolicyParityAgents(t *testing.T) {
	s := newStage4(t, slog.New(slog.DiscardHandler))
	ctx, org, user := s.org(t, "owner")
	p := agentParity{parity: parity{s: s, org: org, user: user, gw: dbtest.Gateway(t, s.pool, org, "gw"),
		snaps: newTunnelDeps(s.d, metrics.NewRegistry(), slog.New(slog.DiscardHandler)).Snapshots}, agent: uuid.New(), run: uuid.New()}
	team := dbtest.Team(t, s.pool, org, user)
	srv, err := s.d.Inventory.Create(ctx, org, inventory.CreateInput{Name: "srv", Type: "ssh", GatewayID: &p.gw,
		Config: &sshv1.Config{Host: "db.internal", Login: "root"}})
	require.NoError(t, err)
	p.srv = srv.ID
	s.bind(ctx, t, &werbotv1.CreateBindingRequest{SubjectType: "team", SubjectId: team.String(), ResourceId: p.srv.String()})
	agents := policy.Subjects{AllAgents: true}
	sel := policy.Selector{ResourceIDs: []uuid.UUID{p.srv}}
	command := func(effect string, classes ...string) {
		_, err := s.d.Policy.CreateCommandRule(ctx, org, policy.CommandRuleInput{Name: effect + uuid.NewString()[:8], Effect: effect,
			Message: "Needs an OK", Subjects: agents, Selector: sel, Classes: classes})
		require.NoError(t, err)
	}
	path := func(effect, message string, globs ...string) {
		_, err := s.d.Policy.CreatePathRule(ctx, org, policy.PathRuleInput{Name: effect + uuid.NewString()[:8], Subjects: agents, Selector: sel,
			Paths: globs, Access: "any", Effect: effect, Message: message})
		require.NoError(t, err)
	}
	lockID := uuid.New()
	custom := policy.SnapshotCustomPolicy{ID: uuid.New(), Cedar: `permit (principal is Werbot::Agent, action, resource);`}
	withCustom := func() func(*policy.Snapshot) {
		return func(s *policy.Snapshot) { s.CustomPolicies = append(s.CustomPolicies, custom) }
	}
	cases := []struct {
		name, command string
		action        string // "" is ssh.exec
		granted       bool
		setup         func() func(*policy.Snapshot) // a DB change, then a snapshot change
		allow         bool
		reason        string   // a reason the decision must carry
		obligations   []string // of a deny
		message       string
	}{
		{name: "read", command: "ls -l /tmp", allow: true, reason: "agent:baseline:read"},
		{name: "write", command: "touch /tmp/x"},
		{name: "destructive", command: "rm -rf /tmp/x", reason: "agent:baseline:dangerous"},
		{name: "terminal", action: "ssh.shell", reason: "agent:baseline:terminal"},
		{name: "frozen", command: "ls", reason: "agent:baseline:frozen",
			setup: func() func(*policy.Snapshot) { return func(s *policy.Snapshot) { s.Agents[0].Status = "frozen" } }},
		{name: "lock agent_run", command: "ls", reason: "lock:" + lockID.String(), setup: func() func(*policy.Snapshot) {
			return func(s *policy.Snapshot) {
				s.Locks = append(s.Locks, policy.SnapshotLock{ID: lockID, TargetType: "agent_run", TargetID: &p.run})
			}
		}},
		{name: "blocked run", command: "ls", reason: "agent:baseline:blocked_runs",
			setup: func() func(*policy.Snapshot) { return func(s *policy.Snapshot) { s.BlockedRuns = []uuid.UUID{p.run} } }},
		{name: "path deny", command: "cat /etc/hosts", message: "No /etc",
			setup: func() func(*policy.Snapshot) { path("deny", "No /etc", "/etc/**"); return nil }},
		{name: "path only outside", command: "cat /etc/hosts", message: policy.PathOutsideMessage,
			setup: func() func(*policy.Snapshot) { path("only", "", "~/app/**"); return nil }},
		{name: "path only inside", command: "cat ~/app/a", allow: true, setup: func() func(*policy.Snapshot) { path("only", "", "~/app/**"); return nil }},
		{name: "approval", command: "touch /tmp/x", obligations: []string{policy.ObligationApproval}, message: "Needs an OK",
			setup: func() func(*policy.Snapshot) { command("approval", "write"); return nil }},
		{name: "approval granted", command: "touch /tmp/x", granted: true, allow: true,
			setup: func() func(*policy.Snapshot) { command("approval", "write"); return nil }},
		{name: "approval and sponsor deny", command: "touch /tmp/x", message: "No writes", setup: func() func(*policy.Snapshot) {
			_, err := s.d.Policy.CreateCommandRule(ctx, org, policy.CommandRuleInput{Name: "deny" + uuid.NewString()[:8], Effect: "deny",
				Message: "No writes", Subjects: policy.Subjects{AllUsers: true}, Selector: sel, Classes: []string{"write"}})
			require.NoError(t, err)
			command("approval", "write")
			return nil
		}},
		{name: "custom permit", command: "touch /tmp/x", allow: true, reason: "custom:" + custom.ID.String() + ":0", setup: withCustom},
		{name: "custom permit destructive", command: "rm -rf /tmp/x", reason: "agent:baseline:dangerous", setup: withCustom},
		{name: "missing agent", command: "ls", reason: "agent:missing",
			setup: func() func(*policy.Snapshot) { return func(s *policy.Snapshot) { s.Agents = nil } }},
		{name: "foreign sponsor", command: "ls", reason: "agent:missing",
			setup: func() func(*policy.Snapshot) { return func(s *policy.Snapshot) { s.Agents[0].SponsorID = uuid.New() } }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for _, table := range []string{"command_rule", "path_rule"} {
				_, err := s.pool.Exec(ctx, "DELETE FROM "+table+" WHERE org_id = $1", org)
				require.NoError(t, err)
			}
			var edit func(*policy.Snapshot)
			if tc.setup != nil {
				edit = tc.setup()
			}
			ps, pol, teams := p.compiledWith(ctx, t, edit)
			action := cmp.Or(tc.action, "ssh.exec")
			server, gw := p.decideAgent(t, ps, pol, teams, action, tc.command, tc.granted, &p.agent)
			requireSame(t, server, gw, tc.name)
			assert.Equal(t, tc.allow, server.Allow, "%v", server.Reasons)
			assert.Equal(t, tc.obligations, server.Obligations)
			assert.Equal(t, tc.message, server.Message)
			if tc.reason != "" {
				assert.Contains(t, server.Reasons, tc.reason)
			}
			connS, connG := p.decideAgent(t, ps, pol, teams, "ssh.connect", "", false, &p.agent)
			requireSame(t, connS, connG, tc.name+" connect")

			human, humanGW := p.decideAgent(t, ps, pol, teams, "ssh.exec", "ls", false, nil)
			requireSame(t, human, humanGW, tc.name+" person")
			assert.True(t, human.Allow, "the agent cases never deny the sponsor: %v", human.Reasons)
		})
	}
}

// compiledWith is the server PolicySet and the gateway Policy of the
// organization with the agent of p active and edit applied to the snapshot,
// built as tunnelSnapshots.Org and Pool build it.
func (p agentParity) compiledWith(ctx context.Context, t *testing.T, edit func(*policy.Snapshot)) (*policy.PolicySet, *gateway.Policy, []uuid.UUID) {
	t.Helper()
	teams, err := p.s.d.Identity.UserTeams(ctx, p.org, p.user)
	require.NoError(t, err)
	snap, err := p.s.d.Policy.Snapshot(ctx, p.org)
	require.NoError(t, err)
	snap.Agents = []policy.SnapshotAgent{{ID: p.agent, SponsorID: p.user, Status: "active"}}
	if edit != nil {
		edit(&snap)
	}
	ps, text, err := policy.Compile(snap)
	require.NoError(t, err)
	base := &gatewayv1.Snapshot{CedarPolicies: text, PathRules: pathRules(snap), ClassRules: classRules(snap.ClassRules)}
	base.EntitiesJson, err = ps.EntitiesJSON()
	require.NoError(t, err)
	gsnap, err := p.snaps.Pool(ctx, base, p.org, p.gw)
	require.NoError(t, err)
	pol, err := gateway.CompilePolicy(gsnap)
	require.NoError(t, err)
	return ps, pol, teams
}

// decideAgent decides action on srv for agent (nil: the user) in run p.run
// on both sides; a command is analysed on the gateway and decided with the
// same classes, paths and path rule hits on the server. The gateway's agent
// authenticated an hour ago: past the grace, it is decided like the server.
func (p agentParity) decideAgent(t *testing.T, ps *policy.PolicySet, pol *gateway.Policy, teams []uuid.UUID, action, command string,
	granted bool, agent *uuid.UUID) (policy.Decision, gateway.Decision) {
	t.Helper()
	r, ok := pol.Resource("srv")
	require.True(t, ok)
	ip := netip.MustParseAddr("10.1.2.3")
	kind := gateway.KindExec
	if action == "ssh.shell" {
		kind = gateway.KindTerminal
	}
	req := gateway.AccessRequest{User: gateway.User{ID: p.user, TeamIDs: teams}, Resource: r, IP: ip, Time: monday10,
		Action: action, Kind: kind, ApprovalGranted: granted}
	sreq := policy.Request{UserID: p.user, TeamIDs: teams, ResourceID: r.ID, Action: action, IP: ip, Time: monday10,
		Kind: string(kind), ApprovalGranted: granted}
	if agent != nil {
		req.User.Agent = &gateway.Agent{ID: *agent, RunID: p.run, AuthAt: monday10.Add(-time.Hour)}
		sreq.AgentID, sreq.RunID = agent, &p.run
	}
	if command != "" {
		ep, _, err := r.Endpoint()
		require.NoError(t, err)
		a := pol.Analyze(command, ep.Host, "~")
		for _, c := range a.Classes {
			req.Classes = append(req.Classes, string(c))
		}
		for _, ref := range a.Paths {
			req.Paths = append(req.Paths, ref.String())
		}
		req.PathHits, req.PathOutside = shellclass.EvalPaths(pol.PathRules(), a.Paths, a.Unresolved)
		req.Command = command
		sreq.Command, sreq.Classes, sreq.Paths, sreq.PathHits, sreq.PathOutside = command, req.Classes, req.Paths, req.PathHits, req.PathOutside
	}
	return ps.Authorize(sreq), pol.Decide(req)
}
