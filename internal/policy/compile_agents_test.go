package policy

import (
	"fmt"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/cedar-policy/cedar-go"
	xast "github.com/cedar-policy/cedar-go/x/exp/ast"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/werbot/werbot/internal/connector"
)

// sftpy is a type with an sftp action, which no registered type declares
// before stage 23b.
func init() {
	connector.Register(connector.Type{Name: "sftpy", NewConfig: func() proto.Message { return &emptypb.Empty{} },
		Validate: func(proto.Message) error { return nil }, Actions: []string{"sftpy.connect", "sftpy.sftp"},
		Kinds: []connector.Kind{connector.KindExec}})
}

var (
	agent1ID   = uuid.MustParse("019a0000-0000-7000-8000-0000000000a1") // alice's (userID)
	agent2ID   = uuid.MustParse("019a0000-0000-7000-8000-0000000000a2") // bob's (otherID)
	run1ID     = uuid.MustParse("019a0000-0000-7000-8000-0000000000b1")
	run2ID     = uuid.MustParse("019a0000-0000-7000-8000-0000000000b2")
	bobBindID  = uuid.MustParse("019a0000-0000-7000-8000-0000000000b3")
	agentLock1 = uuid.MustParse("019a0000-0000-7000-8000-0000000000d1")
	agentLock2 = uuid.MustParse("019a0000-0000-7000-8000-0000000000d2")
	agentLock3 = uuid.MustParse("019a0000-0000-7000-8000-0000000000d3")
	agentLock4 = uuid.MustParse("019a0000-0000-7000-8000-0000000000d4")
)

// agentSnap is execSnap() with bob bound to r1 too, agent a1 of alice and
// agent a2 of bob, and the given command rules.
func agentSnap(rules ...SnapshotCommandRule) Snapshot {
	s := execSnap(rules...)
	s.Bindings = append(s.Bindings, SnapshotBinding{ID: bobBindID, SubjectType: "user", SubjectID: otherID, ResourceIDs: []uuid.UUID{resID}})
	s.Agents = []SnapshotAgent{{ID: agent1ID, SponsorID: userID, Status: "active"}, {ID: agent2ID, SponsorID: otherID, Status: "active"}}
	return s
}

// agentReq is an ssh.exec request of agent a1 (sponsor alice) in run 1 on r1.
func agentReq(classes ...string) Request {
	return asAgent(execReq(classes...), agent1ID, userID)
}

// asAgent is r made by agent in run 1 with sponsor as the user.
func asAgent(r Request, agent, sponsor uuid.UUID) Request {
	run := run1ID
	r.UserID, r.AgentID, r.RunID, r.Kind = sponsor, &agent, &run, "exec"
	return r
}

// baselineText is the agent baseline Compile writes with the registered types.
func baselineText(blockedRuns ...uuid.UUID) string {
	var b strings.Builder
	writeBaseline(&b, connector.Types(), blockedRuns)
	return b.String()
}

func agentLock(id uuid.UUID, typ string, target *uuid.UUID, until int64) SnapshotLock {
	return SnapshotLock{ID: id, TargetType: typ, TargetID: target, ExpiresUnix: until}
}

func goldenAgentSnap() Snapshot {
	allow := userRule(allowID, "allow", "", "write")
	allow.Subjects = Subjects{AllAgents: true}
	appr := userRule(apprID, "approval", "", "write")
	appr.Subjects = Subjects{Users: []uuid.UUID{userID}, Teams: []uuid.UUID{teamID}}
	deny := userRule(denyID, "deny", "No secrets", "secrets")
	deny.Subjects = Subjects{AllUsers: true, AllAgents: true}
	s := agentSnap(deny, appr, allow)
	path := pathRule(pathDenyID, "deny", "any", "", "/etc/**")
	path.Subjects = Subjects{AllAgents: true}
	s.PathRules = []SnapshotPathRule{path}
	a1, run, alice := agent1ID, run1ID, userID
	s.Locks = []SnapshotLock{agentLock(agentLock4, "all_agents", nil, 2000), agentLock(agentLock3, "agents_of_user", &alice, 0),
		agentLock(agentLock2, "agent_run", &run, 1000), agentLock(agentLock1, "agent", &a1, 0)}
	s.BlockedRuns = []uuid.UUID{run2ID, run1ID}
	return s
}

func TestCompileAgentGolden(t *testing.T) {
	s := goldenAgentSnap()
	ps, text, err := Compile(s)
	require.NoError(t, err)
	want, err := os.ReadFile("testdata/compile_agents.golden.cedar")
	require.NoError(t, err)
	assert.Equal(t, string(want), text)

	slices.Reverse(s.Locks)
	slices.Reverse(s.CommandRules)
	slices.Reverse(s.BlockedRuns)
	slices.Reverse(s.Agents)
	ps2, text2, err := Compile(s)
	require.NoError(t, err)
	assert.Equal(t, text, text2, "input order does not change the text")

	ents, err := ps.EntitiesJSON()
	require.NoError(t, err)
	ents2, err := ps2.EntitiesJSON()
	require.NoError(t, err)
	assert.Equal(t, string(ents), string(ents2))
	assert.Contains(t, string(ents), fmt.Sprintf(`{"uid":{"type":"Werbot::Agent","id":%q},"parents":[],"attrs":{"sponsor":{"__entity":{"type":"Werbot::User","id":%q}},"status":"active"}`, agent1ID, userID))
}

func TestCompileEmitsBaselineWithoutAgents(t *testing.T) {
	text := compileText(t, snap())
	for _, id := range []string{"connect", "read", "dangerous", "terminal", "sftp", "remote_forward", "frozen"} {
		assert.Contains(t, text, `@id("agent:baseline:`+id+`")`, id)
	}
	assert.NotContains(t, text, "agent:baseline:blocked_runs")
	assert.True(t, strings.HasSuffix(text, "when { principal.status == \"frozen\" };\n\n"), "the baseline ends the text")
}

func TestAgentBaselineSFTP(t *testing.T) {
	sftpID := uuid.MustParse("019a0000-0000-7000-8000-0000000000b4")
	s := agentSnap()
	s.Resources = append(s.Resources, SnapshotResource{ID: sftpID, Name: "files", Type: "sftpy"})
	s.Bindings[0].ResourceIDs = []uuid.UUID{resID, sftpID}
	r := agentReq()
	r.ResourceID, r.Action, r.Classes = sftpID, "sftpy.sftp", nil
	d := authorize(t, s, r)
	assert.False(t, d.Allow)
	assert.Contains(t, d.Reasons, "agent:baseline:sftp")

	human := r
	human.AgentID, human.RunID, human.Kind = nil, nil, ""
	assert.True(t, authorize(t, s, human).Allow, "the sponsor keeps the type's actions")

	assert.Equal(t, 1, strings.Count(compileText(t, s), `@id("agent:baseline:sftp")`))
	ssh, ok := connector.Lookup("ssh")
	require.True(t, ok)
	var b strings.Builder
	writeBaseline(&b, []connector.Type{ssh}, nil)
	assert.NotContains(t, b.String(), "agent:baseline:sftp", "no type declares sftp")
	assert.Contains(t, b.String(), `@id("agent:baseline:read")`)
}

// TestCompiledAgentPoliciesTypecheck validates every compiled agent policy
// (subjects, locks, baseline) against the schema: each principal.sponsor and
// principal.status term sits under principal is Werbot::Agent.
func TestCompiledAgentPoliciesTypecheck(t *testing.T) {
	_, text, err := Compile(goldenAgentSnap())
	require.NoError(t, err)
	v, err := validator()
	require.NoError(t, err)
	list, err := cedar.NewPolicyListFromBytes("werbot.cedar", []byte(text))
	require.NoError(t, err)
	// two bindings, two command forbids, two agent permits, a path rule,
	// four locks, blocklist, eight baseline policies
	require.Len(t, list, 20)
	for i, p := range list {
		assert.NoError(t, v.Policy(fmt.Sprint(i), (*xast.Policy)(p.AST())), i)
	}
}

func TestCompileRejectsBadAgentLock(t *testing.T) {
	for _, typ := range []string{"agent", "agent_run", "agents_of_user"} {
		_, _, err := Compile(jitSnap(nil, agentLock(agentLock1, typ, nil, 0)))
		assert.ErrorContains(t, err, "no target", typ)
	}
}
