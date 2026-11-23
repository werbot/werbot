package policy

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

// agentTCPSnap is agentSnap with a tcp resource "pg" bound to alice and
// custom Cedar policies; agentStream is agent a1's stream request to it.
func agentTCPSnap(custom ...string) (Snapshot, Request) {
	tcpID := uuid.MustParse("019a0000-0000-7000-8000-0000000000b5")
	s := agentSnap()
	s.Resources = append(s.Resources, SnapshotResource{ID: tcpID, Name: "pg", Type: "tcp"})
	s.Bindings[0].ResourceIDs = []uuid.UUID{resID, tcpID}
	for i, text := range custom {
		s.CustomPolicies = append(s.CustomPolicies, SnapshotCustomPolicy{ID: uuid.NewSHA1(customID, []byte{byte(i)}), Cedar: text})
	}
	r := agentReq()
	r.ResourceID, r.Action, r.Kind, r.Port = tcpID, "tcp.connect", "stream", 5432
	return s, r
}

// Input class "custom permit on agents" (O3): a permit scoped to
// Werbot::Agent and an untyped permit both open tcp.connect of kind stream
// for an agent; without one the agent has no stream.
func TestAgentCustomPermitTCP(t *testing.T) {
	s, r := agentTCPSnap()
	assert.False(t, authorize(t, s, r).Allow, "no custom permit: no stream")

	for name, text := range map[string]string{
		"agent":   `permit (principal is Werbot::Agent, action == Werbot::Action::"tcp.connect", resource) when { context.kind == "stream" };`,
		"untyped": `permit (principal, action == Werbot::Action::"tcp.connect", resource);`,
	} {
		s, r := agentTCPSnap(text)
		d := authorize(t, s, r)
		assert.True(t, d.Allow, name)
		assert.Contains(t, d.Reasons, "custom:"+uuid.NewSHA1(customID, []byte{0}).String()+":0", name)
	}

	s, r = agentTCPSnap(`permit (principal is Werbot::User, action == Werbot::Action::"tcp.connect", resource);`)
	assert.False(t, authorize(t, s, r).Allow, "a permit of people does not reach agents")
}

// Input class "custom permit of everything for agents": it never lifts a
// baseline forbid: destructive, secrets and egress commands, a terminal, a
// frozen agent and a blocked run stay denied with the baseline reason.
func TestAgentBaselineForbidNotLifted(t *testing.T) {
	wide := []string{`permit (principal, action, resource);`, `permit (principal is Werbot::Agent, action, resource);`}
	s, _ := agentTCPSnap(wide...)
	assert.True(t, authorize(t, s, agentReq("write")).Allow, "the custom permit widens the agent")

	for _, class := range []string{"destructive", "secrets", "egress"} {
		d := authorize(t, s, agentReq(class))
		assert.False(t, d.Allow, class)
		assert.Contains(t, d.Reasons, "agent:baseline:dangerous", class)
	}
	terminal := agentReq()
	terminal.Action, terminal.Kind = "ssh.shell", "terminal"
	d := authorize(t, s, terminal)
	assert.False(t, d.Allow)
	assert.Contains(t, d.Reasons, "agent:baseline:terminal")

	frozen, _ := agentTCPSnap(wide...)
	frozen.Agents[0].Status = "frozen"
	d = authorize(t, frozen, agentReq("read"))
	assert.False(t, d.Allow)
	assert.Contains(t, d.Reasons, "agent:baseline:frozen")

	blocked, _ := agentTCPSnap(wide...)
	blocked.BlockedRuns = []uuid.UUID{run1ID}
	d = authorize(t, blocked, agentReq("read"))
	assert.False(t, d.Allow)
	assert.Contains(t, d.Reasons, "agent:baseline:blocked_runs")
}
