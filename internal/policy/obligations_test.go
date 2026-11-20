package policy

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// execReq is an ssh.exec request of userID to resID with the given classes.
func execReq(classes ...string) Request {
	r := req("10.1.2.3", "DE", "")
	r.Action, r.Command, r.Classes = "ssh.exec", "cmd", classes
	return r
}

func TestCommandDenyRule(t *testing.T) {
	rule := userRule(denyID, "deny", "No rm", "destructive")
	rule.Subjects = Subjects{Teams: []uuid.UUID{teamID}}
	s := execSnap(rule)
	inTeam := func(r Request) Request { r.TeamIDs = []uuid.UUID{teamID}; return r }

	assert.Equal(t, Decision{Reasons: []string{"command:" + denyID.String()}, Message: "No rm"},
		authorize(t, s, inTeam(execReq("destructive", "read"))))
	assert.True(t, authorize(t, s, inTeam(execReq("read"))).Allow)
	assert.True(t, authorize(t, s, execReq("destructive")).Allow, "user outside the team")

	connect := inTeam(execReq("destructive"))
	connect.Action = "ssh.connect"
	assert.True(t, authorize(t, s, connect).Allow, "rules apply to exec only")
}

func TestApprovalObligation(t *testing.T) {
	s := execSnap(userRule(apprID, "approval", "Needs an OK", "write"))
	d := authorize(t, s, execReq("write"))
	assert.Equal(t, Decision{Reasons: []string{"command:" + apprID.String()}, Obligations: []string{ObligationApproval}, Message: "Needs an OK"}, d)

	r := execReq("write")
	r.ApprovalGranted = true
	assert.True(t, authorize(t, s, r).Allow)
	assert.True(t, authorize(t, s, execReq("read")).Allow)
}

func TestObligationsOnlyWhenAllForbidsCarryThem(t *testing.T) {
	s := execSnap(userRule(apprID, "approval", "", "write"), userRule(denyID, "deny", "blocked", "write"))
	d := authorize(t, s, execReq("write"))
	assert.False(t, d.Allow)
	assert.Nil(t, d.Obligations)
	assert.Equal(t, "blocked", d.Message)
	for range 20 { // cedar-go reports in map order; Authorize sorts
		assert.Equal(t, []string{"command:" + denyID.String(), "command:" + apprID.String()}, authorize(t, s, execReq("write")).Reasons)
	}

	r := execReq("write")
	r.ApprovalGranted = true
	d = authorize(t, s, r)
	assert.False(t, d.Allow, "approval never opens what another forbid denies")
	assert.Nil(t, d.Obligations)

	// The message of the forbid with the smallest @id wins (denyID < apprID).
	s = execSnap(userRule(apprID, "approval", "second", "write"), userRule(denyID, "deny", "first", "write"))
	assert.Equal(t, "first", authorize(t, s, execReq("write")).Message)
}

func TestUnknownObligationOrErrorDenies(t *testing.T) {
	cases := map[string]string{
		"unknown obligation": `@id("ok") permit (principal, action, resource);
@id("f") @obligation("mfa") forbid (principal, action, resource) unless { context.approval_granted };`,
		"evaluation error": `@id("ok") permit (principal, action, resource);
@id("f") @obligation("approval") forbid (principal, action, resource) unless { context.approval_granted };
@id("bad") forbid (principal, action, resource) when { context.missing };`,
	}
	for name, text := range cases {
		ps, _, err := Compile(snap())
		require.NoError(t, err)
		ps.policies, err = parse(text)
		require.NoError(t, err)
		d := ps.Authorize(execReq("write"))
		assert.False(t, d.Allow, name)
		assert.Nil(t, d.Obligations, name)
	}
}

func TestNoObligationWithoutPermit(t *testing.T) {
	s := execSnap(userRule(apprID, "approval", "", "write"))
	s.Bindings = nil
	d := authorize(t, s, execReq("write"))
	assert.False(t, d.Allow)
	assert.Nil(t, d.Obligations)
	assert.Equal(t, []string{"command:" + apprID.String()}, d.Reasons)
}

func TestConnectContextHasEmptyExecFields(t *testing.T) {
	ps, _, err := Compile(snap())
	require.NoError(t, err)
	ps.policies, err = parse(`@id("ok") permit (principal, action, resource)
when { context.command == "" && context.classes == [] && !context.approval_granted };`)
	require.NoError(t, err)
	assert.True(t, ps.Authorize(req("10.1.2.3", "DE", "")).Allow)
}
