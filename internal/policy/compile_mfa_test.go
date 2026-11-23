package policy

import (
	"fmt"
	"net/netip"
	"os"
	"slices"
	"testing"

	"github.com/cedar-policy/cedar-go"
	xast "github.com/cedar-policy/cedar-go/x/exp/ast"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	mfaAnyID  = uuid.MustParse("019a0000-0000-7000-8000-000000000101")
	mfaWebID  = uuid.MustParse("019a0000-0000-7000-8000-000000000102")
	mfaNoneID = uuid.MustParse("019a0000-0000-7000-8000-000000000103")
)

// mfaRule is an MFA rule of userID on resources.
func mfaRule(id uuid.UUID, seconds int64, strength string, resources ...uuid.UUID) SnapshotMFARule {
	return SnapshotMFARule{ID: id, Subjects: Subjects{Users: []uuid.UUID{userID}}, ResourceIDs: resources, MaxAgeSeconds: seconds, Strength: strength}
}

// mfaSnap is snap() with the given MFA rules.
func mfaSnap(rules ...SnapshotMFARule) Snapshot {
	s := snap()
	s.MFARules = rules
	return s
}

// mfaReq is a connect request of userID to resID with a factor of method
// confirmed age seconds ago.
func mfaReq(age int64, method string) Request {
	r := req("10.1.2.3", "DE", "")
	r.MFAAgeSeconds, r.MFAMethod = age, method
	return r
}

func goldenMFASnap() Snapshot {
	team := mfaRule(mfaAnyID, 60, "any", resID, rawID)
	team.Subjects = Subjects{Teams: []uuid.UUID{teamID}}
	none := mfaRule(mfaNoneID, 60, "any")
	s := pathSnap(nil)
	s.MFARules = []SnapshotMFARule{mfaRule(mfaWebID, 300, "webauthn", resID), none, team}
	return s
}

// TestCompileMFARule: one obligation forbid per rule on the connect actions
// of the types of its resources, ascending; a rule without resources emits
// nothing; the text does not depend on the rule order.
func TestCompileMFARule(t *testing.T) {
	s := goldenMFASnap()
	_, text, err := Compile(s)
	require.NoError(t, err)
	want, err := os.ReadFile("testdata/compile_mfa.golden.cedar")
	require.NoError(t, err)
	assert.Equal(t, string(want), text)
	assert.NotContains(t, text, mfaNoneID.String())

	slices.Reverse(s.MFARules)
	_, text2, err := Compile(s)
	require.NoError(t, err)
	assert.Equal(t, text, text2, "rules compile in ID order")
}

// TestCompiledMFAPoliciesTypecheck validates every compiled MFA policy against the schema.
func TestCompiledMFAPoliciesTypecheck(t *testing.T) {
	_, text, err := Compile(goldenMFASnap())
	require.NoError(t, err)
	v, err := validator()
	require.NoError(t, err)
	list, err := cedar.NewPolicyListFromBytes("werbot.cedar", []byte(text))
	require.NoError(t, err)
	for i, p := range list {
		assert.NoError(t, v.Policy(fmt.Sprint(i), (*xast.Policy)(p.AST())), i)
	}
}

func TestCompileRejectsBadMFARule(t *testing.T) {
	for name, r := range map[string]SnapshotMFARule{
		"strength":  mfaRule(mfaAnyID, 60, "sms", resID),
		"short age": mfaRule(mfaAnyID, 29, "any", resID),
		"long age":  mfaRule(mfaAnyID, 86401, "any", resID),
	} {
		_, _, err := Compile(mfaSnap(r))
		assert.Error(t, err, name)
	}
}

func TestMFAObligation(t *testing.T) {
	reason := []string{"mfa:" + mfaAnyID.String()}
	s := mfaSnap(mfaRule(mfaAnyID, 60, "any", resID))
	assert.True(t, authorize(t, s, mfaReq(30, "totp")).Allow)
	assert.True(t, authorize(t, s, mfaReq(0, "webauthn")).Allow)
	for _, age := range []int64{60, 61} {
		assert.Equal(t, Decision{Reasons: reason, Obligations: []string{ObligationMFA}}, authorize(t, s, mfaReq(age, "totp")), age)
	}

	s = mfaSnap(mfaRule(mfaAnyID, 60, "webauthn", resID))
	assert.Equal(t, Decision{Reasons: reason, Obligations: []string{ObligationMFA}}, authorize(t, s, mfaReq(10, "totp")),
		"strength webauthn is not met by a TOTP code")
	assert.True(t, authorize(t, s, mfaReq(10, "webauthn")).Allow)

	exec := mfaReq(1000, "totp")
	exec.Action = "ssh.exec"
	assert.True(t, authorize(t, s, exec).Allow, "mfa rules apply to connect only")
}

// TestZeroRequestHasNoMFA: a request without a method, or with a negative
// age, never meets a rule, whatever its age says.
func TestZeroRequestHasNoMFA(t *testing.T) {
	s := mfaSnap(mfaRule(mfaAnyID, 86400, "any", resID))
	want := Decision{Reasons: []string{"mfa:" + mfaAnyID.String()}, Obligations: []string{ObligationMFA}}
	assert.Equal(t, want, authorize(t, s, req("10.1.2.3", "DE", "")))
	assert.Equal(t, want, authorize(t, s, mfaReq(0, "")))
	assert.Equal(t, want, authorize(t, s, mfaReq(5, "")))
	assert.Equal(t, want, authorize(t, s, mfaReq(-1, "webauthn")))
}

func TestMFAObligationWithOtherForbid(t *testing.T) {
	deny := ResourceRules{ResourceID: resID, Networks: []NetworkRule{{CIDR: netip.MustParsePrefix("10.0.0.0/8"), Effect: EffectDeny}}}
	s := mfaSnap(mfaRule(mfaAnyID, 60, "any", resID))
	s.Rules = []ResourceRules{deny}
	d := authorize(t, s, mfaReq(0, ""))
	assert.False(t, d.Allow)
	assert.Nil(t, d.Obligations, "a factor never opens what another forbid denies")

	s = mfaSnap(mfaRule(mfaAnyID, 60, "any", resID))
	s.Bindings = nil
	d = authorize(t, s, mfaReq(0, ""))
	assert.False(t, d.Allow)
	assert.Nil(t, d.Obligations, "no binding: a factor allows nothing")
}

func TestMFAStrength(t *testing.T) {
	strength := func(rules ...SnapshotMFARule) string {
		ps, _, err := Compile(mfaSnap(rules...))
		require.NoError(t, err)
		return MFAStrength(ps, mfaReq(0, ""))
	}
	assert.Equal(t, "any", strength(mfaRule(mfaAnyID, 60, "any", resID)))
	assert.Equal(t, "webauthn", strength(mfaRule(mfaWebID, 60, "webauthn", resID)))
	assert.Equal(t, "webauthn", strength(mfaRule(mfaAnyID, 60, "any", resID), mfaRule(mfaWebID, 600, "webauthn", resID)))
}

func TestCustomPolicyMFAObligation(t *testing.T) {
	text := `@obligation("mfa") forbid (principal, action, resource) unless { context.mfa_age < 300 };`
	require.NoError(t, ValidateCustom(text))
	s := snap()
	s.CustomPolicies = []SnapshotCustomPolicy{{ID: mfaAnyID, Cedar: text}}
	d := authorize(t, s, mfaReq(0, ""))
	assert.False(t, d.Allow)
	assert.Equal(t, []string{ObligationMFA}, d.Obligations)
	assert.True(t, authorize(t, s, mfaReq(10, "totp")).Allow)
}

// TestAgentMFAObligationPlainDeny: an agent of a sponsor under an MFA rule
// is denied plainly, even when its request carries a factor; the sponsor
// alone gets the obligation.
func TestAgentMFAObligationPlainDeny(t *testing.T) {
	s := agentSnap()
	rule := mfaRule(mfaAnyID, 60, "any", resID)
	rule.Subjects = Subjects{AllUsers: true}
	s.MFARules = []SnapshotMFARule{rule}
	for _, r := range []Request{mfaReq(0, ""), mfaReq(0, "webauthn")} {
		d := authorize(t, s, asAgent(r, agent1ID, userID))
		assert.False(t, d.Allow, r.MFAMethod)
		assert.Nil(t, d.Obligations, r.MFAMethod)
		assert.Contains(t, d.Reasons, "mfa:"+mfaAnyID.String(), r.MFAMethod)
	}
	assert.Equal(t, []string{ObligationMFA}, authorize(t, s, mfaReq(0, "")).Obligations, "the sponsor")
}
