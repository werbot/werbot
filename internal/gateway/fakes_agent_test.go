package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/cedar-policy/cedar-go"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	gatewayv1 "github.com/werbot/werbot/gen/gateway/v1"
)

// outy serves an exec like an SSH target that prints "<command>
// secret=<secret>": to stdout and the recording; it exits with 3.
func outy(_ context.Context, s *Session) error {
	out := fmt.Sprintf("%s secret=%s\n", s.Command, s.Secret.Value)
	_, _ = io.WriteString(s.Exec.Stdout, out)
	s.Recording.Output([]byte(out))
	s.Exec.SetExitStatus(3)
	return nil
}

// agentBaseline is the Cedar the server writes for agents
// (internal/policy/compile_agents.go writeBaseline) on the types typs, which
// have an exec action, with the blocked runs.
func agentBaseline(blockedRuns []string, typs ...string) string {
	var connect, exec []string
	for _, t := range typs {
		connect = append(connect, fmt.Sprintf("Werbot::Action::%q", t+".connect"))
		exec = append(exec, fmt.Sprintf("Werbot::Action::%q", t+".exec"))
	}
	text := `
@id("agent:baseline:connect")
permit (principal is Werbot::Agent, action in [` + strings.Join(connect, ", ") + `], resource)
when { context.kind == "exec" };
@id("agent:baseline:read")
permit (principal is Werbot::Agent, action in [` + strings.Join(exec, ", ") + `], resource)
when { ["read"].containsAll(context.classes) };
@id("agent:baseline:dangerous")
forbid (principal is Werbot::Agent, action, resource)
when { context.classes.containsAny(["destructive", "secrets", "egress"]) };
@id("agent:baseline:terminal")
forbid (principal is Werbot::Agent, action, resource)
when { context.kind == "terminal" };
@id("agent:baseline:frozen")
forbid (principal is Werbot::Agent, action, resource)
when { principal.status == "frozen" };
`
	if len(blockedRuns) > 0 {
		text += `@id("agent:baseline:blocked_runs")
forbid (principal, action, resource)
when { ["` + strings.Join(blockedRuns, `", "`) + `"].contains(context.run_id) };
`
	}
	return text
}

// testAgent is a Werbot::Agent entity of a snapshot: short names of the
// agent and its sponsor, and its status.
type testAgent struct{ id, sponsor, status string }

// withAgents adds the entities of agents to s, as the server compile does.
func withAgents(t *testing.T, s *gatewayv1.Snapshot, m ids, agents ...testAgent) {
	t.Helper()
	ents := cedar.EntityMap{}
	require.NoError(t, json.Unmarshal(s.GetEntitiesJson(), &ents))
	for _, a := range agents {
		uid := cedar.NewEntityUID("Werbot::Agent", cedar.String(m[a.id].String()))
		ents[uid] = cedar.Entity{UID: uid, Attributes: cedar.NewRecord(cedar.RecordMap{
			"sponsor": cedar.NewEntityUID("Werbot::User", cedar.String(m[a.sponsor].String())), "status": cedar.String(a.status)})}
	}
	var err error
	s.EntitiesJson, err = json.Marshal(ents)
	require.NoError(t, err)
}

// agentPolicy compiles policies and the agent baseline over "x" (execy,
// host 10.0.0.5) and "fake", with the agents and blocked runs.
func agentPolicy(t *testing.T, m ids, policies string, blockedRuns []string, agents ...testAgent) *Policy {
	t.Helper()
	for _, n := range []string{"rx", "rfake"} {
		if _, ok := m[n]; !ok {
			m[n] = uuid.New()
		}
	}
	s := testSnapshot(t, m, policies+agentBaseline(blockedRuns, "execy"), nil, res{"rx", "x", "execy"}, res{"rfake", "fake", "fake"})
	s.Resources[0].ConfigJson = []byte(`{"host":"10.0.0.5"}`)
	withAgents(t, s, m, agents...)
	p, err := CompilePolicy(s)
	require.NoError(t, err)
	return p
}

// recordedDenial is one RecordDenial call.
type recordedDenial struct {
	run               uuid.UUID
	resource, command string
	classes, reasons  []string
}

// agentControl is fakeControl with RecordDenial recorded: denyErr fails it.
type agentControl struct {
	*fakeControl
	mu      sync.Mutex
	denyErr error
	denials []recordedDenial
}

func (a *agentControl) RecordDenial(_ context.Context, run uuid.UUID, resource, command string, classes, reasons []string) (bool, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.denials = append(a.denials, recordedDenial{run, resource, command, classes, reasons})
	return false, a.denyErr
}

func (a *agentControl) recorded() []recordedDenial {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.denials
}

// RecordDenial of the plain fake: a server without it.
func (*fakeControl) RecordDenial(context.Context, uuid.UUID, string, string, []string, []string) (bool, error) {
	return false, connect.NewError(connect.CodeUnimplemented, nil)
}

// newAgentEnv is a pipeline env on agentPolicy of execTestPolicies whose user u1 is the sponsor
// of agent a1 in run run1, authenticated now.
func newAgentEnv(t *testing.T) (*pipeEnv, *agentControl) {
	t.Helper()
	e := newPipeEnv(t, 10)
	e.m["a1"], e.m["run1"] = uuid.New(), uuid.New()
	e.f.setPolicy(agentPolicy(t, e.m, execTestPolicies, nil, testAgent{"a1", "u1", "active"}))
	e.f.with(func(f *fakeControl) { f.user.Agent = &Agent{ID: e.m["a1"], RunID: e.m["run1"], AuthAt: time.Now()} })
	a := &agentControl{fakeControl: e.f}
	e.p.ctl = a
	return e, a
}

// startAgents keeps the agent run and tool call StartSession received, by session id.
var startAgents sync.Map

// startedAs is started that also keeps the agent of the session.
func startedAs(paths []string, a startArgs) uuid.UUID {
	id := started(paths)
	startAgents.Store(id, a)
	return id
}

// sessionAgent is the agent run and tool call StartSession received for session id.
func sessionAgent(id uuid.UUID) startArgs {
	a, _ := startAgents.Load(id)
	out, _ := a.(startArgs)
	return out
}
