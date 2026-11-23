package gateway

import (
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mfaPolicies is the compiled shape of a binding and an MFA rule of 60 s
// with strength any on r1 (internal/policy/compile_mfa.go).
const mfaPolicies = `
@id("binding:b")
permit (principal == Werbot::User::"u1", action, resource) when { [Werbot::Resource::"r1", Werbot::Resource::"r2"].contains(resource) };
@id("mfa:a")
@obligation("mfa")
forbid (principal, action in [Werbot::Action::"ssh.connect"], resource)
when { principal in [Werbot::User::"u1"] && [Werbot::Resource::"r1"].contains(resource) }
unless { context.mfa_age < 60 && (true || context.mfa_method == "webauthn") };`

// webauthnPolicy is the same rule with strength webauthn on r2.
const webauthnPolicy = `
@id("mfa:w")
@obligation("mfa")
forbid (principal, action in [Werbot::Action::"ssh.connect"], resource)
when { principal in [Werbot::User::"u1"] && [Werbot::Resource::"r2"].contains(resource) }
unless { context.mfa_age < 600 && (false || context.mfa_method == "webauthn") };`

// mfaRequest is a connect request of u1 to resource with a factor of method
// confirmed age seconds ago.
func mfaRequest(t *testing.T, p *Policy, m ids, resource string, age int64, method string) AccessRequest {
	t.Helper()
	r, ok := p.Resource(resource)
	require.True(t, ok)
	return AccessRequest{User: User{ID: m["u1"]}, Resource: r, IP: netip.MustParseAddr("192.0.2.1"), MFAAgeSeconds: age, MFAMethod: method}
}

func TestDecideMFAObligation(t *testing.T) {
	m := newIDs("u1", "r1", "r2")
	p := compileTest(t, m, mfaPolicies+webauthnPolicy, nil, res{"r1", "r1", "ssh"}, res{"r2", "r2", "ssh"})
	need := func(id string) Decision { return Decision{Reasons: []string{id}, Obligations: []string{ObligationMFA}} }

	assert.True(t, p.Decide(mfaRequest(t, p, m, "r1", 30, "totp")).Allow)
	assert.Equal(t, need("mfa:a"), p.Decide(mfaRequest(t, p, m, "r1", 60, "totp")))
	assert.Equal(t, need("mfa:w"), p.Decide(mfaRequest(t, p, m, "r2", 10, "totp")), "strength webauthn is not met by a TOTP code")
	assert.True(t, p.Decide(mfaRequest(t, p, m, "r2", 10, "webauthn")).Allow)

	exec := mfaRequest(t, p, m, "r1", 0, "")
	exec.Action = "ssh.exec"
	assert.True(t, p.Decide(exec).Allow, "mfa rules apply to connect only")
}

// TestGatewayZeroRequestHasNoMFA mirrors TestZeroRequestHasNoMFA.
func TestGatewayZeroRequestHasNoMFA(t *testing.T) {
	m := newIDs("u1", "r1", "r2")
	p := compileTest(t, m, mfaPolicies, nil, res{"r1", "r1", "ssh"}, res{"r2", "r2", "ssh"})
	for _, r := range []AccessRequest{mfaRequest(t, p, m, "r1", 0, ""), mfaRequest(t, p, m, "r1", 5, ""), mfaRequest(t, p, m, "r1", -1, "webauthn")} {
		assert.Equal(t, Decision{Reasons: []string{"mfa:a"}, Obligations: []string{ObligationMFA}}, p.Decide(r), r)
	}
}

func TestGatewayMFAStrength(t *testing.T) {
	m := newIDs("u1", "r1", "r2")
	p := compileTest(t, m, mfaPolicies+webauthnPolicy, nil, res{"r1", "r1", "ssh"}, res{"r2", "r2", "ssh"})
	assert.Equal(t, "any", p.MFAStrength(mfaRequest(t, p, m, "r1", 0, "")))
	assert.Equal(t, "webauthn", p.MFAStrength(mfaRequest(t, p, m, "r2", 0, "")))
}

// TestGatewayPermittedIncludesMFA: the menu lists a resource a person reaches
// after an MFA check, not one another forbid denies.
func TestGatewayPermittedIncludesMFA(t *testing.T) {
	m := newIDs("u1", "r1", "r2")
	deny := "\n" + `@id("rule") forbid (principal, action, resource == Werbot::Resource::"r2");`
	p := compileTest(t, m, mfaPolicies+webauthnPolicy+deny, nil, res{"r1", "r1", "ssh"}, res{"r2", "r2", "ssh"})
	got := p.Permitted(User{ID: m["u1"]}, netip.MustParseAddr("192.0.2.1"), time.Now())
	require.Len(t, got, 1)
	assert.Equal(t, "r1", got[0].Name)
}

// TestGatewayAgentMFAPlainDeny mirrors TestAgentMFAObligationPlainDeny: an
// agent of a sponsor under an MFA rule is denied plainly, even with a factor.
func TestGatewayAgentMFAPlainDeny(t *testing.T) {
	m := newIDs("u1", "u2", "a1", "run1")
	rule := `@id("binding:b") permit (principal == Werbot::User::"u1", action, resource);
@id("mfa:a") @obligation("mfa")
forbid (principal, action in [Werbot::Action::"execy.connect"], resource) when { principal is Werbot::User }
unless { context.mfa_age < 60 && (true || context.mfa_method == "webauthn") };
`
	p := agentPolicy(t, m, rule, nil, testAgent{"a1", "u1", "active"})
	for _, method := range []string{"", "webauthn"} {
		r := agentReq(t, p, m, "x")
		r.Action, r.MFAMethod = "execy.connect", method
		d := p.Decide(r)
		assert.False(t, d.Allow, method)
		assert.Nil(t, d.Obligations, method)
		assert.Contains(t, d.Reasons, "mfa:a", method)
		assert.Empty(t, p.Permitted(r.User, r.IP, agentNow), method)
	}
	person := agentReq(t, p, m, "x")
	person.User.Agent, person.Action = nil, "execy.connect"
	assert.Equal(t, []string{ObligationMFA}, p.Decide(person).Obligations, "the sponsor")
}
