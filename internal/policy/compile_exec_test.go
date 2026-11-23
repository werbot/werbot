package policy

import (
	"os"
	"testing"

	"github.com/cedar-policy/cedar-go"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	res2ID  = uuid.MustParse("019a0000-0000-7000-8000-000000000006")
	res3ID  = uuid.MustParse("019a0000-0000-7000-8000-000000000008")
	denyID  = uuid.MustParse("019a0000-0000-7000-8000-0000000000c1")
	apprID  = uuid.MustParse("019a0000-0000-7000-8000-0000000000c2")
	allowID = uuid.MustParse("019a0000-0000-7000-8000-0000000000c3")
)

// userRule is a command rule of userID on resID.
func userRule(id uuid.UUID, effect, message string, classes ...string) SnapshotCommandRule {
	return SnapshotCommandRule{ID: id, Subjects: Subjects{Users: []uuid.UUID{userID}}, ResourceIDs: []uuid.UUID{resID},
		Classes: classes, Effect: effect, Message: message}
}

// execSnap is snap() plus two more resources and the given command rules.
func execSnap(rules ...SnapshotCommandRule) Snapshot {
	s := snap()
	s.Resources = append(s.Resources,
		SnapshotResource{ID: res2ID, Name: "web", Type: "ssh", Labels: map[string]string{"env": "prod"}},
		SnapshotResource{ID: res3ID, Name: "dev", Type: "ssh", Labels: map[string]string{"env": "dev"}})
	s.CommandRules = rules
	return s
}

func TestCompileExecGolden(t *testing.T) {
	s := execSnap()
	s.CommandRules = []SnapshotCommandRule{
		{ID: allowID, Subjects: Subjects{Users: []uuid.UUID{userID}}, ResourceIDs: []uuid.UUID{resID}, Classes: []string{"read"}, Effect: "allow"},
		{ID: apprID, Subjects: Subjects{Users: []uuid.UUID{otherID, userID}}, ResourceIDs: []uuid.UUID{resID}, Classes: []string{"write"}, Effect: "approval"},
		{ID: denyID, Subjects: Subjects{Teams: []uuid.UUID{teamID}}, ResourceIDs: ExpandSelector(s.Resources, Selector{Labels: map[string]string{"env": "prod"}}),
			Classes: []string{"secrets", "destructive"}, Effect: "deny", Message: "No rm"},
	}
	_, text, err := Compile(s)
	require.NoError(t, err)
	want, err := os.ReadFile("testdata/compile_exec.golden.cedar")
	require.NoError(t, err)
	assert.Equal(t, string(want), text)

	// Reversed input order compiles to the same text.
	s.CommandRules[0], s.CommandRules[2] = s.CommandRules[2], s.CommandRules[0]
	s.CommandRules[1].Subjects.Users = []uuid.UUID{userID, otherID} // the approval rule
	_, text2, err := Compile(s)
	require.NoError(t, err)
	assert.Equal(t, text, text2)
}

func TestBindingCoversConnectAndExec(t *testing.T) {
	for _, action := range []string{"ssh.connect", "ssh.exec"} {
		r := req("10.1.2.3", "DE", "")
		r.Action = action
		assert.True(t, authorize(t, snap(), r).Allow, action)
	}
	r := req("10.1.2.3", "DE", "")
	r.Action = "ssh.remote_forward"
	assert.Equal(t, Decision{Reasons: []string{"invalid_action"}}, authorize(t, snap(), r))
}

func TestAllowRuleSkippedForHumans(t *testing.T) {
	agentsOnly := userRule(denyID, "deny", "", "write")
	agentsOnly.Subjects = Subjects{AllAgents: true}
	noResources := userRule(apprID, "approval", "", "write")
	noResources.ResourceIDs = nil
	_, text, err := Compile(execSnap(userRule(allowID, "allow", "", "read"), agentsOnly, noResources))
	require.NoError(t, err)
	assert.NotContains(t, text, "command:"+apprID.String(), "a rule without resources")
	assert.NotContains(t, text, "principal in [Werbot::User::", "allow and agent rules have no term for people")
}

func TestAllUsersRule(t *testing.T) {
	rule := userRule(denyID, "deny", "", "write")
	rule.Subjects = Subjects{AllUsers: true}
	_, text, err := Compile(execSnap(rule))
	require.NoError(t, err)
	assert.Contains(t, text, "when { principal is Werbot::User && ")

	r := execReq("write")
	r.UserID = otherID
	ps, _, err := Compile(execSnap(rule))
	require.NoError(t, err)
	assert.Equal(t, []string{"command:" + denyID.String()}, ps.Authorize(r).Reasons)
}

func TestCompileEscapesMessage(t *testing.T) {
	msg := `say "no" \ now — нет ✓`
	_, text, err := Compile(execSnap(userRule(denyID, "deny", msg, "write")))
	require.NoError(t, err)
	_, err = cedar.NewPolicyListFromBytes("werbot.cedar", []byte(text))
	require.NoError(t, err)
	d := authorize(t, execSnap(userRule(denyID, "deny", msg, "write")), execReq("write"))
	assert.False(t, d.Allow)
	assert.Equal(t, msg, d.Message)
}

func TestCompileRejectsBadCommandRule(t *testing.T) {
	bad := map[string]SnapshotCommandRule{
		"effect":     userRule(denyID, "maybe", "", "write"),
		"no classes": userRule(denyID, "deny", ""),
	}
	for name, rule := range bad {
		ps, _, err := Compile(execSnap(rule))
		assert.Error(t, err, name)
		assert.Nil(t, ps, name)
	}
}

func TestExpandSelector(t *testing.T) {
	rs := execSnap().Resources // resID, res2ID env=prod; res3ID env=dev
	assert.Equal(t, []uuid.UUID{resID, res2ID}, ExpandSelector(rs, Selector{Labels: map[string]string{"env": "prod"}}))
	assert.Equal(t, []uuid.UUID{resID, res3ID}, ExpandSelector(rs, Selector{ResourceIDs: []uuid.UUID{res3ID, otherID, resID}}),
		"ids outside the snapshot drop, the rest sort")
	assert.Empty(t, ExpandSelector(rs, Selector{}), "an empty selector matches nothing")
	assert.Empty(t, ExpandSelector(rs, Selector{Labels: map[string]string{"env": "qa"}}))
}
