//go:build redteam

package redteam

import (
	"context"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
	"github.com/werbot/werbot/internal/clientauth"
	"github.com/werbot/werbot/internal/mcpserver"
	"github.com/werbot/werbot/test/cluster"
)

// Scenario 6 (12 §12 item 6): the kill switch. A frozen or locked agent loses
// its open sessions within revokeMax, its token exchange and its
// certificates; every lock target ends exactly its own scope (rulings C8, G18).

// sleeper is a long command of class read: it outlives every check.
const sleeper = "sleep 60"

// agentClientName is the client of the runs the tests exchange for (ruling G18).
const agentClientName = "redteam"

// healthyTries is how often healthySoon tries "id": fewer than
// denialsToPause, so a gateway still on the previous policy cannot pause the run.
const healthyTries = denialsToPause - 1

// startLong runs sleeper on target as a in the background and returns its
// open session and a channel that closes when the call returns.
func startLong(t *testing.T, a *cluster.Agent) (uuid.UUID, <-chan struct{}) {
	t.Helper()
	done := make(chan struct{})
	go func() {
		defer close(done)
		_, _ = a.Exec(context.Background(), mcpserver.ExecInput{Resource: target, Command: sleeper, Timeout: 90})
	}()
	return runSession(t, a.RunID, sleeper), done
}

// healthySoon requires "id" on target to run for a within healthyTries
// tries a second apart: a freeze, lock or binding change reaches both
// gateways within that time.
func healthySoon(t *testing.T, a *cluster.Agent) {
	t.Helper()
	var last mcpserver.ExecOutput
	for range healthyTries {
		out, err := a.Exec(t.Context(), execOn("id"))
		require.NoError(t, err, "health check")
		if out.Status == "ok" {
			return
		}
		last = out
		time.Sleep(time.Second)
	}
	t.Fatalf("health check of %s: %s %s", a.Name, last.Status, last.Message)
}

// exchange trades alice's CLI profile for a run token of agent (ruling G18);
// Cleanup ends that run.
func exchange(t *testing.T, agent string) *clientauth.AgentProfile {
	t.Helper()
	run, err := profile(t).ExchangeAgent(t.Context(), agent, agentClientName, "kill-switch")
	require.NoError(t, err)
	t.Cleanup(func() { _ = run.End(context.Background()) }) // after the unlock or unfreeze Cleanups registered later
	return run
}

// profile loads alice's CLI profile from home.
func profile(t *testing.T) *clientauth.Profile {
	t.Helper()
	p, err := clientauth.Current(home, func(string) string { return "" })
	require.NoError(t, err)
	return p
}

// setAgentStatus freezes ("frozen") or unfreezes ("active") an agent.
func setAgentStatus(t *testing.T, id uuid.UUID, status string) {
	t.Helper()
	_, err := st.Edge.Agents.UpdateAgent(context.Background(), connect.NewRequest(&werbotv1.UpdateAgentRequest{Id: id.String(), Status: status}))
	require.NoError(t, err, "agent status %s", status)
}

// waitReturned requires the call behind done to return within reportWait.
func waitReturned(t *testing.T, done <-chan struct{}, what string) {
	t.Helper()
	select {
	case <-done:
	case <-time.After(reportWait):
		t.Fatalf("%s did not return within %s", what, reportWait)
	}
}

// TestRedteamFrozenAgentLosesTokenAndSession: freezing an agent ends its open
// session within revokeMax (access_revoked), refuses its next call, a new
// token exchange (invalid_grant) and a certificate for a run token it
// already holds (PermissionDenied); unfreezing restores it.
func TestRedteamFrozenAgentLosesTokenAndSession(t *testing.T) {
	begin(t)
	const name = "rt-frozen"
	a := st.StartAgent(t, home, name, "redteam")
	requireAgentHealthy(t, a)
	run := exchange(t, name)
	id, done := startLong(t, a)
	updates := audits(t, "agent.update", a.AgentID)

	t.Cleanup(func() { setAgentStatus(t, a.AgentID, "active") })
	from := dbNow(t)
	setAgentStatus(t, a.AgentID, "frozen")
	endedWithin(t, id, from)
	waitReturned(t, done, "the frozen agent's command")
	assert.Equal(t, updates+1, audits(t, "agent.update", a.AgentID))

	_, err := a.Exec(t.Context(), execOn("id"))
	require.ErrorContains(t, err, "agent is frozen", "a frozen agent's next call")
	_, err = profile(t).ExchangeAgent(t.Context(), name, agentClientName, "retry")
	require.ErrorContains(t, err, "invalid_grant", "a frozen agent gets no run token")
	_, err = run.Signer(t.Context())
	require.Equal(t, connect.CodePermissionDenied, connect.CodeOf(err), "IssueUserCert with the frozen agent's run token: %v", err)
	require.ErrorContains(t, err, "agent is frozen")

	setAgentStatus(t, a.AgentID, "active")
	healthySoon(t, st.StartAgent(t, home, name, "redteam"))
}

// killCase is one lock target: the lock ends the first ends of the open
// sessions of (alice's agent, alice's other agent, bob's agent); the rest and
// alice's own shell stay. certDenied: the lock also refuses a certificate for
// a new run of alice's agent.
type killCase struct {
	typ        string
	target     func(own *cluster.Agent) *uuid.UUID
	ends       int
	certDenied bool
}

// TestRedteamKillSwitchTargets: each lock target ends exactly its sessions
// within revokeMax; a person's session stays under every agent lock.
func TestRedteamKillSwitchTargets(t *testing.T) {
	begin(t)
	bobHome := t.TempDir()
	st.Login(t, bob, bobHome, st.Edge.BaseURL)
	me, err := bob.Auth.GetSession(t.Context(), connect.NewRequest(&werbotv1.GetSessionRequest{}))
	require.NoError(t, err)
	bind(t, uuid.MustParse(me.Msg.GetUser().GetId()), st.Resources[target])
	sh := cluster.OpenShell(t, st.Dial(t, st.Addr(t, "nginx", 2222), "alice+"+target))
	sh.Mark(t, "human")
	human := shellSession(t)

	alice := st.AliceID
	for _, c := range []killCase{
		{"agent_run", func(own *cluster.Agent) *uuid.UUID { return &own.RunID }, 1, false},
		{"agent", func(own *cluster.Agent) *uuid.UUID { return &own.AgentID }, 1, true},
		{"agents_of_user", func(*cluster.Agent) *uuid.UUID { return &alice }, 2, true},
		{"all_agents", func(*cluster.Agent) *uuid.UUID { return nil }, 3, true},
	} {
		t.Run(c.typ, func(t *testing.T) {
			killTarget(t, c, bobHome, human)
			sh.Mark(t, "alive")
		})
	}
}

// killTarget runs one killCase.
func killTarget(t *testing.T, c killCase, bobHome string, human uuid.UUID) {
	own := st.StartAgent(t, home, "rt-kill", "redteam")
	agents := []*cluster.Agent{own, st.StartAgent(t, home, "rt-kill-other", "redteam"), st.StartAgent(t, bobHome, "rt-kill-bob", "redteam")}
	sessions := make([]uuid.UUID, len(agents))
	for i, a := range agents {
		healthySoon(t, a)
		sessions[i], _ = startLong(t, a)
	}
	run := exchange(t, own.Name)

	from := dbNow(t)
	id := lock(t, c.typ, c.target(own))
	for _, s := range sessions[:c.ends] {
		endedWithin(t, s, from)
	}
	stay := append([]uuid.UUID{human}, sessions[c.ends:]...)
	assert.False(t, within(revokeMax, func() bool {
		for _, s := range stay {
			if !open(t, s) {
				return true
			}
		}
		return false
	}), "a %s lock ends only its scope", c.typ)
	assert.Equal(t, 1, audits(t, "lock.create", id))
	_, err := own.Exec(t.Context(), execOn("id"))
	require.ErrorContains(t, err, "agent is locked", "the locked run's next call")

	_, err = run.Signer(t.Context())
	if c.certDenied {
		require.Equal(t, connect.CodePermissionDenied, connect.CodeOf(err), "IssueUserCert under a %s lock: %v", c.typ, err)
		require.ErrorContains(t, err, "agent is locked")
	} else {
		require.NoError(t, err, "a lock of another run leaves this run's certificate")
	}
	unlock(t, id)
	for _, s := range sessions[c.ends:] { // end the sleepers that stayed, so the agents close at once
		_, err := st.Edge.Sessions.TerminateSession(t.Context(), connect.NewRequest(&werbotv1.TerminateSessionRequest{Id: s.String(), Reason: "redteam"}))
		assert.NoError(t, err)
	}
}
