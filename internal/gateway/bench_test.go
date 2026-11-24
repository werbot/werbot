package gateway

import (
	"encoding/json"
	"fmt"
	"net/netip"
	"strings"
	"testing"

	"github.com/cedar-policy/cedar-go"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	gatewayv1 "github.com/werbot/werbot/gen/gateway/v1"
	"github.com/werbot/werbot/internal/platform/shellclass"
)

const benchCommand = "cat /var/log/syslog | grep -i error | tail -n 50"

// worstCommand touches shellclass.MaxPaths paths (Ruling G6).
var worstCommand = func() string {
	paths := make([]string, shellclass.MaxPaths)
	for i := range paths {
		paths[i] = fmt.Sprintf("/srv/d%d/f", i)
	}
	return "cat " + strings.Join(paths, " ")
}()

// execScale sizes the snapshot of execBench; every resource has one binding.
// halfApproval turns the odd command rules into plain forbids (no obligation).
type execScale struct {
	resources, commandRules, customPolicies, grants, locks, agents int
	halfApproval                                                   bool
}

var (
	// scaleBudget is the design §12 budget snapshot, the gate of TestExecDecisionP99.
	scaleBudget = execScale{resources: 1000, commandRules: 100, customPolicies: 10}
	// scaleL2 is the L2 scale of the load test (10 §8.3): a known limit, measured only (O1).
	scaleL2 = execScale{resources: 10000, commandRules: 200, customPolicies: 5, grants: 50, locks: 10, agents: 100, halfApproval: true}
)

// execBench compiles the latency snapshot of scale (design §12). The budget
// scale: 1 000 resources and bindings, 100 command rules, 100 path rules of 4
// globs, 10 custom policies and the agent baseline with 100 blocked runs; the request
// is an ssh.exec of a team member that an approval rule covers, so Decide
// also runs the trial evaluation of the obligation. agent is the same request
// of the member's agent: two passes.
func execBench(tb testing.TB, sc execScale) (p *Policy, r, agent AccessRequest) {
	tb.Helper()
	team, user, agentID := uuid.New(), uuid.New(), uuid.New()
	ids := make([]string, sc.resources)
	ents := cedar.EntityMap{}
	s := &gatewayv1.Snapshot{Version: "bench"}
	var b strings.Builder
	for i := range ids {
		ids[i] = uuid.NewString()
		uid := cedar.NewEntityUID("Werbot::Resource", cedar.String(ids[i]))
		ents[uid] = cedar.Entity{UID: uid, Attributes: cedar.NewRecord(cedar.RecordMap{"name": cedar.String(fmt.Sprint("r", i)), "type": cedar.String("ssh")})}
		s.Resources = append(s.Resources, &gatewayv1.Resource{Id: ids[i], Name: fmt.Sprint("r", i), Type: "ssh", ConfigJson: []byte(`{}`)})
		fmt.Fprintf(&b, "@id(\"binding:%d\")\npermit (principal in Werbot::Team::%q, action, resource)\nwhen { [Werbot::Resource::%q].contains(resource) };\n\n",
			i, team.String(), ids[i])
	}
	writeCommandRules(&b, sc, team, ids)
	for i := range sc.customPolicies {
		fmt.Fprintf(&b, "@id(\"custom:%d:0\")\nforbid (principal, action == Werbot::Action::\"ssh.exec\", resource)\nwhen { context.command like \"*shutdown*\" && context.hour == %d };\n\n", i, i)
	}
	for i := range 100 {
		id := uuid.NewString()
		s.PathRules = append(s.PathRules, &gatewayv1.PathRule{Id: id, Access: "any", Effect: "deny",
			Paths: []string{fmt.Sprintf("/srv/app%d/**", i), fmt.Sprintf("/home/*/secret%d/**", i), fmt.Sprintf("~/keys%d/*.pem", i), fmt.Sprintf("/etc/conf%d/*", i)}})
		fmt.Fprintf(&b, "@id(\"path:%s\")\nforbid (principal, action in [Werbot::Action::\"ssh.exec\"], resource)\n"+
			"when { principal in [Werbot::Team::%q] && context.path_hits.contains(%q) };\n\n", id, team.String(), id)
	}
	writeGrantsAndLocks(&b, sc, ids)
	runs := make([]string, 100)
	for i := range runs {
		runs[i] = uuid.NewString()
	}
	b.WriteString(agentBaseline(runs, "ssh"))
	s.CedarPolicies = b.String()
	uid := cedar.NewEntityUID("Werbot::Agent", cedar.String(agentID.String()))
	ents[uid] = cedar.Entity{UID: uid, Attributes: cedar.NewRecord(cedar.RecordMap{
		"sponsor": cedar.NewEntityUID("Werbot::User", cedar.String(user.String())), "status": cedar.String("active")})}
	addOtherAgents(ents, sc, user)
	var err error
	s.EntitiesJson, err = json.Marshal(ents)
	require.NoError(tb, err)
	p, err = CompilePolicy(s)
	require.NoError(tb, err)
	res, ok := p.Resource("r0")
	require.True(tb, ok)
	r = AccessRequest{User: User{ID: user, TeamIDs: []uuid.UUID{team}}, Resource: res, IP: netip.MustParseAddr("192.0.2.1"),
		Action: "ssh.exec", Kind: KindExec, Command: benchCommand}
	agent = r
	agent.User.Agent = &Agent{ID: agentID, RunID: uuid.New()}
	return p, r, agent
}

// writeCommandRules writes the command rules; each covers 10 resources. With
// halfApproval the odd rules forbid without an obligation.
func writeCommandRules(b *strings.Builder, sc execScale, team uuid.UUID, ids []string) {
	for i := range sc.commandRules {
		refs := make([]string, 10)
		for j := range refs {
			refs[j] = fmt.Sprintf("Werbot::Resource::%q", ids[(i*10+j)%len(ids)])
		}
		obligation := "@obligation(\"approval\")\n"
		if sc.halfApproval && i%2 == 1 {
			obligation = ""
		}
		fmt.Fprintf(b, "@id(\"command:%03d\")\n@message(\"rule %d\")\n%sforbid (principal, action in [Werbot::Action::\"ssh.exec\"], resource)\n"+
			"when { principal in [Werbot::Team::%q] && [%s].contains(resource) && context.classes.containsAny([\"read\", \"write\", \"opaque\"]) }\n"+
			"unless { context.approval_granted };\n\n", i, i, obligation, team.String(), strings.Join(refs, ", "))
	}
}

// writeGrantsAndLocks writes grants of other users and locks on other
// resources: they are evaluated and never match the request.
func writeGrantsAndLocks(b *strings.Builder, sc execScale, ids []string) {
	for i := range sc.grants {
		fmt.Fprintf(b, "@id(\"grant:%d\")\npermit (principal == Werbot::User::%q, action, resource == Werbot::Resource::%q)\nwhen { context.now < 4102444800 };\n\n",
			i, uuid.NewString(), ids[(i+1)%len(ids)])
	}
	for i := range sc.locks {
		fmt.Fprintf(b, "@id(\"lock:%d\")\nforbid (principal, action, resource == Werbot::Resource::%q)\nwhen { context.now < 4102444800 };\n\n", i, ids[len(ids)-1-i])
	}
}

// addOtherAgents adds sc.agents more active agents of the sponsor user.
func addOtherAgents(ents cedar.EntityMap, sc execScale, user uuid.UUID) {
	for range sc.agents {
		other := cedar.NewEntityUID("Werbot::Agent", cedar.String(uuid.NewString()))
		ents[other] = cedar.Entity{UID: other, Attributes: cedar.NewRecord(cedar.RecordMap{
			"sponsor": cedar.NewEntityUID("Werbot::User", cedar.String(user.String())), "status": cedar.String("active")})}
	}
}

// decideExec analyses the request command, matches its paths and decides it.
func decideExec(p *Policy, r AccessRequest) Decision {
	return p.Decide(p.withCommand(r, commandInput{Text: r.Command, Cwd: "~"}))
}

func BenchmarkExecDecision(b *testing.B) {
	p, human, agent := execBench(b, scaleBudget)
	require.Equal(b, []string{ObligationApproval}, decideExec(p, human).Obligations)
	require.Equal(b, []string{ObligationApproval}, decideExec(p, agent).Obligations, "the agent pass allows a read")
	for who, r := range map[string]AccessRequest{"human": human, "agent": agent} {
		for name, command := range map[string]string{"bench": benchCommand, "worst": worstCommand} {
			r.Command = command
			b.Run(who+"/"+name, func(b *testing.B) {
				for b.Loop() {
					decideExec(p, r)
				}
			})
		}
	}
}
