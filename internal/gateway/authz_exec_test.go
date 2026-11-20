package gateway

import (
	"log/slog"
	"net/netip"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	gatewayv1 "github.com/werbot/werbot/gen/gateway/v1"
)

// execPolicies is the compiled shape of a binding, a deny rule and an approval
// rule on ssh.exec (internal/policy/compile_exec.go).
const execPolicies = `
@id("binding:b")
permit (principal == Werbot::User::"u1", action, resource) when { [Werbot::Resource::"r1"].contains(resource) };
@id("command:a")
@message("No rm")
forbid (principal, action in [Werbot::Action::"ssh.exec"], resource) when { context.classes.containsAny(["destructive"]) };
@id("command:b")
@message("Needs an OK")
@obligation("approval")
forbid (principal, action in [Werbot::Action::"ssh.exec"], resource) when { context.classes.containsAny(["write"]) }
unless { context.approval_granted };`

// execRequest is an ssh.exec request of u1 to r1 with classes.
func execRequest(t *testing.T, p *Policy, m ids, classes ...string) AccessRequest {
	t.Helper()
	r1, ok := p.Resource("r1")
	require.True(t, ok)
	return AccessRequest{User: User{ID: m["u1"]}, Resource: r1, IP: netip.MustParseAddr("192.0.2.1"),
		Action: "ssh.exec", Command: "cmd", Classes: classes}
}

func TestDecideExecObligations(t *testing.T) {
	m := newIDs("u1", "r1")
	p := compileTest(t, m, execPolicies, nil, res{"r1", "r1", "ssh"})

	assert.Equal(t, Decision{Reasons: []string{"command:b"}, Obligations: []string{ObligationApproval}, Message: "Needs an OK"},
		p.Decide(execRequest(t, p, m, "write")))

	d := p.Decide(execRequest(t, p, m, "destructive", "write"))
	assert.False(t, d.Allow)
	assert.Nil(t, d.Obligations, "an approval never opens what another forbid denies")
	assert.Equal(t, "No rm", d.Message, "the message of the smallest @id")
	for range 20 { // cedar-go reports in map order; Decide sorts
		assert.Equal(t, []string{"command:a", "command:b"}, p.Decide(execRequest(t, p, m, "destructive", "write")).Reasons)
	}

	granted := execRequest(t, p, m, "write")
	granted.ApprovalGranted = true
	assert.Equal(t, Decision{Allow: true, Reasons: []string{"binding:b"}}, p.Decide(granted))
	assert.True(t, p.Decide(execRequest(t, p, m, "read")).Allow)

	connect := execRequest(t, p, m, "destructive")
	connect.Action = ""
	assert.True(t, p.Decide(connect).Allow, "an empty Action is <type>.connect; command rules apply to exec only")
	allow, reasons := p.Authorize(execRequest(t, p, m, "destructive"))
	assert.False(t, allow, "Authorize is Decide")
	assert.Equal(t, []string{"command:a"}, reasons)

	bad := execRequest(t, p, m, "read")
	bad.Action = "ssh.remote_forward"
	assert.Equal(t, Decision{Reasons: []string{"invalid_action"}}, p.Decide(bad))
}

// A deny asks for nothing when a deciding forbid has no known obligation, when
// meeting the obligations still denies, or when a policy fails to evaluate.
func TestDecideObligationsFailClosed(t *testing.T) {
	m := newIDs("u1", "r1")
	permit := `@id("binding:b") permit (principal, action, resource);` + "\n"
	for name, text := range map[string]string{
		"unknown obligation": permit + `@id("f") @obligation("mfa") forbid (principal, action, resource) unless { context.approval_granted };`,
		"no permit":          `@id("f") @obligation("approval") forbid (principal, action, resource) unless { context.approval_granted };`,
		"evaluation error": permit + `@id("f") @obligation("approval") forbid (principal, action, resource) unless { context.approval_granted };
@id("bad") forbid (principal, action, resource) when { context.missing };`,
	} {
		p := compileTest(t, m, text, nil, res{"r1", "r1", "ssh"})
		d := p.Decide(execRequest(t, p, m, "write"))
		assert.False(t, d.Allow, name)
		assert.Nil(t, d.Obligations, name)
	}
}

// Reasons name policies by @id, else by their position (policy<n>, as the
// snapshot text orders them); a duplicate name rejects the snapshot.
func TestDecideReasonsUseIDs(t *testing.T) {
	m := newIDs("u1", "r1")
	rule := "command:" + uuid.NewString()
	p := compileTest(t, m, `@id("binding:b") permit (principal, action, resource);
@id("`+rule+`") forbid (principal, action == Werbot::Action::"ssh.exec", resource) when { context.classes.contains("egress") };
forbid (principal, action == Werbot::Action::"ssh.exec", resource) when { context.classes.contains("secrets") };`,
		nil, res{"r1", "r1", "ssh"})
	assert.Equal(t, []string{rule}, p.Decide(execRequest(t, p, m, "egress")).Reasons)
	assert.Equal(t, []string{"policy2"}, p.Decide(execRequest(t, p, m, "secrets")).Reasons)

	for _, text := range []string{
		`@id("x") permit (principal, action, resource); @id("x") forbid (principal, action, resource);`,
		`@id("policy1") permit (principal, action, resource); permit (principal, action, resource);`,
	} {
		_, err := CompilePolicy(&gatewayv1.Snapshot{CedarPolicies: text})
		assert.Error(t, err, text)
	}
}

// A class rule the classifier refuses rejects the snapshot; the tunnel keeps
// the previous Policy.
func TestCompilePolicyRejectsBadClassRule(t *testing.T) {
	for name, cr := range map[string]*gatewayv1.ClassRule{
		"bad glob":      {Pattern: "[", Classes: []string{"read"}},
		"unknown class": {Pattern: "deploy", Classes: []string{"bogus"}},
		"no classes":    {Pattern: "deploy"},
	} {
		_, err := CompilePolicy(&gatewayv1.Snapshot{ClassRules: []*gatewayv1.ClassRule{cr}})
		assert.Error(t, err, name)
	}

	c := &tunnelClient{log: slog.New(slog.DiscardHandler), changed: make(chan struct{}, 1)}
	require.True(t, c.apply(&gatewayv1.Snapshot{Version: "v1"}))
	require.False(t, c.apply(&gatewayv1.Snapshot{Version: "v2", ClassRules: []*gatewayv1.ClassRule{{Pattern: "[", Classes: []string{"read"}}}}))
	assert.Equal(t, "v1", c.policy.Load().Version())
}

func TestClassifyUsesOverrides(t *testing.T) {
	p, err := CompilePolicy(&gatewayv1.Snapshot{ClassRules: []*gatewayv1.ClassRule{{Pattern: "./deploy.sh", Classes: []string{"write"}}}})
	require.NoError(t, err)
	assert.Equal(t, []string{"write"}, p.Classify("./deploy.sh", ""))
	assert.Equal(t, []string{"opaque"}, p.Classify("./other.sh", ""), "a path without an override is opaque")
	assert.Equal(t, []string{"read"}, p.Classify("curl http://db.internal/health", "db.internal"), "the resource host is not egress")
	assert.Equal(t, []string{"opaque"}, (*Policy)(nil).Classify("ls", ""), "no snapshot: opaque")
}

func TestMaskOutput(t *testing.T) {
	for in, want := range map[string]string{"": "agents", "agents": "agents", "all": "all", "none": "none"} {
		p, err := CompilePolicy(&gatewayv1.Snapshot{MaskOutput: in})
		require.NoError(t, err)
		assert.Equal(t, want, p.MaskOutput(), "an older server sends none: %q", in)
	}
}
