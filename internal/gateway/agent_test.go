package gateway

import (
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var agentNow = time.Date(2026, 11, 23, 10, 0, 0, 0, time.UTC)

// agentReq is an execy.exec request on resource of u1's agent a1 in run1,
// authenticated at agentNow, with classes.
func agentReq(t *testing.T, p *Policy, m ids, resource string, classes ...string) AccessRequest {
	t.Helper()
	r, ok := p.Resource(resource)
	require.True(t, ok, resource)
	return AccessRequest{User: User{ID: m["u1"], Agent: &Agent{ID: m["a1"], RunID: m["run1"], AuthAt: agentNow}},
		Resource: r, IP: netip.MustParseAddr("192.0.2.1"), Time: agentNow, Action: "execy.exec", Kind: KindExec, Classes: classes}
}

// The gateway mirrors TestAgentTwoPass and TestAgentBaselineReadOnlyExactSet
// (internal/policy/agents_test.go): an agent's request passes as its sponsor
// and as the agent; only reads pass the baseline; the baseline leaves people alone.
func TestAuthorizeAgentTwoPass(t *testing.T) {
	m := newIDs("u1", "u2", "a1", "run1", "ry")
	p := agentPolicy(t, m, execTestPolicies, nil, testAgent{"a1", "u1", "active"})
	read := agentReq(t, p, m, "x", "read")
	d := p.Decide(read)
	require.True(t, d.Allow, d.Reasons)
	assert.Subset(t, d.Reasons, []string{"binding:b", "agent:baseline:read"})
	assert.Equal(t, d, p.AuthorizeAgent(read), "Decide dispatches an agent to AuthorizeAgent")

	withOwn := strings.Replace(execTestPolicies, `principal == Werbot::User::"u1", action, resource`,
		`principal == Werbot::User::"u1", action, resource == Werbot::Resource::"rx"`, 1)
	q := compileTest(t, m, withOwn+agentBaseline(nil, "execy"), nil, res{"rx", "x", "execy"}, res{"ry", "y", "execy"})
	assert.True(t, q.Decide(agentReq(t, q, m, "x", "read")).Allow, "the substituted agent of the sponsor")
	assert.False(t, q.Decide(agentReq(t, q, m, "y", "read")).Allow, "the sponsor has no binding on y")

	d = p.Decide(agentReq(t, p, m, "x", "read", "write"))
	assert.False(t, d.Allow)
	assert.Empty(t, d.Obligations)
	d = p.Decide(agentReq(t, p, m, "x", "destructive", "read"))
	assert.False(t, d.Allow)
	assert.Contains(t, d.Reasons, "agent:baseline:dangerous")
	assert.Equal(t, "No rm", d.Message, "the sponsor's message")
	for _, c := range []string{"opaque", "privilege", "secrets", "egress", "write"} {
		assert.False(t, p.Decide(agentReq(t, p, m, "x", c)).Allow, c)
	}

	human := agentReq(t, p, m, "x", "privilege", "write")
	human.User.Agent = nil
	assert.True(t, p.Decide(human).Allow, "the baseline does not apply to a person")
}

func TestAuthorizeAgentForeignSponsorDenied(t *testing.T) {
	m := newIDs("u1", "u2", "a1", "run1")
	p := agentPolicy(t, m, execTestPolicies, nil, testAgent{"a1", "u2", "active"})
	assert.Equal(t, Decision{Reasons: []string{"agent:missing"}}, p.Decide(agentReq(t, p, m, "x", "read")),
		"an agent the snapshot lists for another sponsor")
}

func TestAgentKinds(t *testing.T) {
	m := newIDs("u1", "a1", "run1")
	p := agentPolicy(t, m, execTestPolicies, nil, testAgent{"a1", "u1", "active"})
	connect := agentReq(t, p, m, "x")
	connect.Action = ""
	assert.True(t, p.Decide(connect).Allow, "connect of an exec")
	connect.Kind = KindTerminal
	d := p.Decide(connect)
	assert.False(t, d.Allow)
	assert.Contains(t, d.Reasons, "agent:baseline:terminal")

	shell := agentReq(t, p, m, "x")
	shell.Action, shell.Kind = "execy.shell", KindTerminal
	d = p.Decide(shell)
	assert.False(t, d.Allow)
	assert.Contains(t, d.Reasons, "agent:baseline:terminal")
	assert.Empty(t, d.Obligations)
}

func TestAgentEntityMissingFromSnapshot(t *testing.T) {
	m := newIDs("u1", "a1", "run1")
	p := agentPolicy(t, m, execTestPolicies, nil)
	d := p.Decide(agentReq(t, p, m, "x", "read"))
	require.True(t, d.Allow, "an agent the snapshot does not list yet is an active agent of its sponsor: %v", d.Reasons)
	for _, r := range d.Reasons {
		assert.False(t, strings.HasPrefix(r, "error:"), r)
	}

	frozen := agentPolicy(t, m, execTestPolicies, nil, testAgent{"a1", "u1", "frozen"})
	d = frozen.Decide(agentReq(t, frozen, m, "x", "read"))
	assert.False(t, d.Allow)
	assert.Contains(t, d.Reasons, "agent:baseline:frozen")
}

func TestAgentEntityMissingAfterGraceDenied(t *testing.T) {
	m := newIDs("u1", "a1", "run1")
	p := agentPolicy(t, m, execTestPolicies, nil)
	r := agentReq(t, p, m, "x", "read")
	r.Time = agentNow.Add(agentSnapshotGrace - time.Second)
	assert.True(t, p.Decide(r).Allow, "within the grace")
	r.Time = agentNow.Add(agentSnapshotGrace)
	assert.Equal(t, Decision{Reasons: []string{"agent:missing"}}, p.Decide(r), "past the grace a missing agent denies")
}

// Every request carries context.run_id and context.kind: a blocked run
// denies its agent only, and a person is decided without evaluation errors.
func TestAgentBlockedRun(t *testing.T) {
	m := newIDs("u1", "a1", "run1", "run2")
	p := agentPolicy(t, m, execTestPolicies, []string{m["run1"].String()}, testAgent{"a1", "u1", "active"})
	d := p.Decide(agentReq(t, p, m, "x", "read"))
	assert.False(t, d.Allow)
	assert.Contains(t, d.Reasons, "agent:baseline:blocked_runs")

	other := agentReq(t, p, m, "x", "read")
	other.User.Agent.RunID = m["run2"]
	assert.True(t, p.Decide(other).Allow, "another run of the agent")

	human := agentReq(t, p, m, "x", "read")
	human.User.Agent = nil
	d = p.Decide(human)
	assert.True(t, d.Allow, d.Reasons)
	assert.Equal(t, []string{"binding:b"}, d.Reasons)
	human.Action, human.Kind = "", ""
	assert.True(t, p.Decide(human).Allow, "a connect without a kind")
}

func TestMergeAgent(t *testing.T) {
	appr := Decision{Reasons: []string{"command:a"}, Obligations: []string{ObligationApproval}, Message: "OK?"}
	assert.Equal(t, Decision{Allow: true, Reasons: []string{"a", "b"}},
		mergeAgent(Decision{Allow: true, Reasons: []string{"b"}}, Decision{Allow: true, Reasons: []string{"a", "b"}}))
	assert.Equal(t, Decision{Reasons: []string{"b", "command:a"}, Obligations: []string{ObligationApproval}, Message: "OK?"},
		mergeAgent(Decision{Allow: true, Reasons: []string{"b"}}, appr), "one pass asks for an approval")
	assert.Equal(t, Decision{Reasons: []string{"command:a", "x"}, Message: "no"},
		mergeAgent(Decision{Reasons: []string{"x"}, Message: "no"}, appr), "a deny without obligations asks for nothing")
	assert.Equal(t, uuid.Nil, User{}.runID())
}
