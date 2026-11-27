package policy

import (
	"net/netip"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	_ "github.com/werbot/werbot/internal/connector/postgres" // a type recorded as querylog
)

// agentRule is a command rule of every agent on r1.
func agentRule(id uuid.UUID, effect string, classes ...string) SnapshotCommandRule {
	r := userRule(id, effect, "", classes...)
	r.Subjects = Subjects{AllAgents: true}
	return r
}

func TestAgentTwoPass(t *testing.T) {
	d := authorize(t, agentSnap(), agentReq("read"))
	assert.True(t, d.Allow)
	assert.Subset(t, d.Reasons, []string{"binding:" + bindID.String(), "agent:baseline:read"})

	r := agentReq("read")
	r.ResourceID = res2ID
	assert.False(t, authorize(t, agentSnap(), r).Allow, "the sponsor has no binding on r2")

	s := agentSnap()
	s.Rules = []ResourceRules{{ResourceID: resID, Networks: []NetworkRule{{CIDR: netip.MustParsePrefix("10.0.0.0/8"), Effect: EffectDeny}}}}
	d = authorize(t, s, agentReq("read"))
	assert.False(t, d.Allow)
	assert.Contains(t, d.Reasons, "rule:network:"+resID.String())
}

func TestAgentMissingOrForeignDenied(t *testing.T) {
	r := agentReq("read")
	other := uuid.New()
	r.AgentID = &other
	assert.Equal(t, Decision{Reasons: []string{"agent:missing"}}, authorize(t, agentSnap(), r))

	r = asAgent(execReq("read"), agent2ID, userID)
	assert.Equal(t, Decision{Reasons: []string{"agent:missing"}}, authorize(t, agentSnap(), r), "sponsor is not the request user")
}

func TestAgentBaselineReadOnlyExactSet(t *testing.T) {
	s := agentSnap()
	assert.True(t, authorize(t, s, agentReq("read")).Allow)
	d := authorize(t, s, agentReq("read", "write"))
	assert.False(t, d.Allow)
	assert.Empty(t, d.Obligations)
	d = authorize(t, s, agentReq("destructive", "read"))
	assert.False(t, d.Allow)
	assert.Contains(t, d.Reasons, "agent:baseline:dangerous")
	for _, c := range []string{"opaque", "privilege", "secrets", "egress", "write"} {
		assert.False(t, authorize(t, s, agentReq(c)).Allow, c)
	}
}

func TestAgentAllowRuleIsSubset(t *testing.T) {
	s := agentSnap(agentRule(allowID, "allow", "write"))
	assert.True(t, authorize(t, s, agentReq("write")).Allow)
	assert.False(t, authorize(t, s, agentReq("read", "write")).Allow)
	assert.False(t, authorize(t, s, agentReq("privilege", "write")).Allow)

	wide := agentSnap(agentRule(allowID, "allow", "read", "write"))
	assert.True(t, authorize(t, wide, agentReq("read", "write")).Allow)

	human := execReq("privilege", "write")
	assert.Equal(t, authorize(t, agentSnap(), human), authorize(t, s, human), "the rule does not change a person's decision")
}

func TestAgentApprovalRule(t *testing.T) {
	s := agentSnap(agentRule(apprID, "approval", "write"))
	d := authorize(t, s, agentReq("write"))
	assert.False(t, d.Allow)
	assert.Equal(t, []string{ObligationApproval}, d.Obligations)
	granted := agentReq("write")
	granted.ApprovalGranted = true
	assert.True(t, authorize(t, s, granted).Allow)

	deny := userRule(denyID, "deny", "", "write")
	s = agentSnap(agentRule(apprID, "approval", "write"), deny)
	d = authorize(t, s, agentReq("write"))
	assert.False(t, d.Allow)
	assert.Empty(t, d.Obligations, "a deny of the sponsor's rule asks for nothing")
	assert.False(t, authorize(t, s, granted).Allow)
}

func TestAgentKindRules(t *testing.T) {
	tcpID := uuid.MustParse("019a0000-0000-7000-8000-0000000000b5")
	s := agentSnap()
	s.Resources = append(s.Resources, SnapshotResource{ID: tcpID, Name: "pg", Type: "tcp"})
	s.Bindings[0].ResourceIDs = []uuid.UUID{resID, tcpID}
	s.TerminalRules = []SnapshotTerminalRule{terminalRule(termConsoleID, "console", resID)}

	connect := agentReq()
	connect.Action = "ssh.connect"
	assert.True(t, authorize(t, s, connect).Allow, "connect of an exec")
	connect.Kind = "terminal"
	d := authorize(t, s, connect)
	assert.False(t, d.Allow)
	assert.Contains(t, d.Reasons, "agent:baseline:terminal")

	stream := agentReq()
	stream.ResourceID, stream.Action, stream.Kind, stream.Port = tcpID, "tcp.connect", "stream", 5432
	assert.False(t, authorize(t, s, stream).Allow, "no permit for streams")

	shell := agentReq()
	shell.Action, shell.Kind = "ssh.shell", "terminal"
	d = authorize(t, s, shell)
	assert.False(t, d.Allow)
	assert.Contains(t, d.Reasons, "agent:baseline:terminal")
	assert.Empty(t, d.Obligations, "the console is not offered to agents")
	human := shellReq("ssh.shell", resID)
	assert.Equal(t, []string{ObligationConsole}, authorize(t, s, human).Obligations)
}

func TestAgentLocks(t *testing.T) {
	r := agentReq("read")
	now := r.Time.Unix()
	a1, run, alice := agent1ID, run1ID, userID
	a2 := asAgent(execReq("read"), agent2ID, otherID)
	run2 := run2ID
	a2.RunID = &run2
	human := execReq("read")
	for name, tc := range map[string]struct {
		lock       SnapshotLock
		a1, a2, hu bool // allowed
	}{
		"agent":          {agentLock(agentLock1, "agent", &a1, now+60), false, true, true},
		"agents_of_user": {agentLock(agentLock1, "agents_of_user", &alice, 0), false, true, true},
		"all_agents":     {agentLock(agentLock1, "all_agents", nil, 0), false, false, true},
		"agent_run":      {agentLock(agentLock1, "agent_run", &run, 0), false, true, true},
		"expired":        {agentLock(agentLock1, "all_agents", nil, now), true, true, true},
	} {
		s := agentSnap()
		s.Locks = []SnapshotLock{tc.lock}
		assert.Equal(t, tc.a1, authorize(t, s, r).Allow, name+" a1")
		assert.Equal(t, tc.a2, authorize(t, s, a2).Allow, name+" a2")
		assert.Equal(t, tc.hu, authorize(t, s, human).Allow, name+" alice")
	}
	s := agentSnap()
	s.Locks = []SnapshotLock{agentLock(agentLock1, "agent_run", &run, 0)}
	other := agentReq("read")
	other.RunID = &run2
	assert.True(t, authorize(t, s, other).Allow, "another run")
}

func TestAgentFrozenAndBlockedRun(t *testing.T) {
	s := agentSnap()
	s.Agents[0].Status = "frozen"
	d := authorize(t, s, agentReq("read"))
	assert.False(t, d.Allow)
	assert.Contains(t, d.Reasons, "agent:baseline:frozen")

	s = agentSnap()
	s.BlockedRuns = []uuid.UUID{run1ID}
	d = authorize(t, s, agentReq("read"))
	assert.False(t, d.Allow)
	assert.Contains(t, d.Reasons, "agent:baseline:blocked_runs")
	assert.True(t, authorize(t, s, execReq("read")).Allow, "a person has no run")
}

func TestAgentSubjectUsersTeams(t *testing.T) {
	rule := userRule(allowID, "allow", "", "write") // users [alice]
	s := agentSnap(rule)
	assert.True(t, authorize(t, s, agentReq("write")).Allow)
	assert.False(t, authorize(t, s, asAgent(execReq("write"), agent2ID, otherID)).Allow, "bob's agent")

	rule.Subjects = Subjects{Teams: []uuid.UUID{teamID}}
	s = agentSnap(rule)
	r := agentReq("write")
	assert.False(t, authorize(t, s, r).Allow, "alice outside the team")
	r.TeamIDs = []uuid.UUID{teamID}
	assert.True(t, authorize(t, s, r).Allow)
}

func TestAgentPathRules(t *testing.T) {
	deny := pathRule(pathDenyID, "deny", "any", "", "/etc/**")
	deny.Subjects = Subjects{AllAgents: true}
	s := agentSnap()
	s.PathRules = []SnapshotPathRule{deny}
	hit := []string{pathDenyID.String()}
	d := authorize(t, s, withPaths(agentReq("read"), hit, nil))
	assert.False(t, d.Allow)
	assert.Contains(t, d.Reasons, "path:"+pathDenyID.String())
	assert.True(t, authorize(t, s, withPaths(execReq("read"), hit, nil)).Allow, "alice herself")

	only := pathRule(pathOnlyID, "only", "any", "", "~/app/**") // users [alice]
	s.PathRules = []SnapshotPathRule{only}
	assert.True(t, authorize(t, s, withPaths(agentReq("read"), nil, nil)).Allow)
	assert.False(t, authorize(t, s, withPaths(agentReq("read"), nil, []string{pathOnlyID.String()})).Allow)

	appr := pathRule(pathApprID, "approval", "any", "", "/srv/**")
	appr.Subjects = Subjects{AllAgents: true}
	s.PathRules = []SnapshotPathRule{appr}
	hit = []string{pathApprID.String()}
	d = authorize(t, s, withPaths(agentReq("read"), hit, nil))
	assert.Equal(t, []string{ObligationApproval}, d.Obligations)
	granted := withPaths(agentReq("read"), hit, nil)
	granted.ApprovalGranted = true
	assert.True(t, authorize(t, s, granted).Allow)
	granted.Classes = []string{"write"}
	assert.False(t, authorize(t, s, granted).Allow, "approval lifts the path, not the class")
}

func withPaths(r Request, hits, outside []string) Request {
	r.Paths, r.PathHits, r.PathOutside = []string{"/x"}, hits, outside
	return r
}

func TestMergeAgentObligations(t *testing.T) {
	allow := Decision{Allow: true, Reasons: []string{"b", "a"}}
	approval := Decision{Reasons: []string{"c", "a"}, Obligations: []string{ObligationApproval}, Message: "m"}
	plain := Decision{Reasons: []string{"d"}}
	assert.Equal(t, Decision{Allow: true, Reasons: []string{"a", "b"}}, mergeAgent(allow, allow))
	assert.Equal(t, Decision{Reasons: []string{"a", "b", "c"}, Obligations: []string{ObligationApproval}, Message: "m"}, mergeAgent(allow, approval))
	assert.Equal(t, Decision{Reasons: []string{"a", "c", "d"}, Message: "m"}, mergeAgent(plain, approval))
	assert.Equal(t, Decision{Reasons: []string{"a", "c"}, Obligations: []string{ObligationApproval}, Message: "m"}, mergeAgent(approval, approval))
}

func TestAgentContext(t *testing.T) {
	ps, _, err := Compile(agentSnap())
	require.NoError(t, err)
	human := execReq("read")
	human.Kind = "terminal" // the terminal baseline is for agents only
	assert.True(t, ps.Authorize(human).Allow)
}

// Input class "agent on a querylog type" (C10): until stage 18 checks a
// read-only database user, an agent never connects to a postgres resource,
// with only its sponsor's binding, with a custom permit on agents or with an
// untyped permit; its sponsor connects with the same binding.
func TestAgentPostgresConnectDenied(t *testing.T) {
	pgID := uuid.MustParse("019a0000-0000-7000-8000-0000000000b6")
	pgSnap := func(custom ...string) Snapshot {
		s := agentSnap()
		s.Resources = append(s.Resources, SnapshotResource{ID: pgID, Name: "pg", Type: "postgres"})
		s.Bindings[0].ResourceIDs = []uuid.UUID{resID, pgID}
		for i, text := range custom {
			s.CustomPolicies = append(s.CustomPolicies, SnapshotCustomPolicy{ID: uuid.NewSHA1(customID, []byte{byte(i)}), Cedar: text})
		}
		return s
	}
	r := agentReq()
	r.ResourceID, r.Action, r.Kind, r.Port = pgID, "postgres.connect", "stream", 5432

	for name, custom := range map[string][]string{
		"binding only": nil,
		"agent permit": {`permit (principal is Werbot::Agent, action == Werbot::Action::"postgres.connect", resource) when { context.kind == "stream" };`},
		"untyped":      {`permit (principal, action == Werbot::Action::"postgres.connect", resource);`},
		"everything":   {`permit (principal, action, resource);`},
	} {
		d := authorize(t, pgSnap(custom...), r)
		assert.False(t, d.Allow, name)
		assert.Contains(t, d.Reasons, "agent:baseline:db_readonly", name)
	}

	human := r
	human.AgentID, human.RunID = nil, nil
	d := authorize(t, pgSnap(), human)
	assert.True(t, d.Allow, "the sponsor connects")
	assert.Equal(t, []string{"binding:" + bindID.String()}, d.Reasons)
}
