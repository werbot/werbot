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

// execBench compiles the latency budget snapshot (design §12): 1 000
// resources, 1 000 bindings, 100 command rules, 100 path rules of 4 globs and
// 10 custom policies; the request is an ssh.exec of a team member that an
// approval rule covers, so Decide also runs the trial evaluation of the obligation.
func execBench(tb testing.TB) (*Policy, AccessRequest) {
	tb.Helper()
	team, user := uuid.New(), uuid.New()
	ids := make([]string, 1000)
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
	for i := range 100 {
		refs := make([]string, 10)
		for j := range refs {
			refs[j] = fmt.Sprintf("Werbot::Resource::%q", ids[(i*10+j)%len(ids)])
		}
		fmt.Fprintf(&b, "@id(\"command:%03d\")\n@message(\"rule %d\")\n@obligation(\"approval\")\nforbid (principal, action in [Werbot::Action::\"ssh.exec\"], resource)\n"+
			"when { principal in [Werbot::Team::%q] && [%s].contains(resource) && context.classes.containsAny([\"read\", \"write\", \"opaque\"]) }\n"+
			"unless { context.approval_granted };\n\n", i, i, team.String(), strings.Join(refs, ", "))
	}
	for i := range 10 {
		fmt.Fprintf(&b, "@id(\"custom:%d:0\")\nforbid (principal, action == Werbot::Action::\"ssh.exec\", resource)\nwhen { context.command like \"*shutdown*\" && context.hour == %d };\n\n", i, i)
	}
	for i := range 100 {
		id := uuid.NewString()
		s.PathRules = append(s.PathRules, &gatewayv1.PathRule{Id: id, Access: "any", Effect: "deny",
			Paths: []string{fmt.Sprintf("/srv/app%d/**", i), fmt.Sprintf("/home/*/secret%d/**", i), fmt.Sprintf("~/keys%d/*.pem", i), fmt.Sprintf("/etc/conf%d/*", i)}})
		fmt.Fprintf(&b, "@id(\"path:%s\")\nforbid (principal, action in [Werbot::Action::\"ssh.exec\"], resource)\n"+
			"when { principal in [Werbot::Team::%q] && context.path_hits.contains(%q) };\n\n", id, team.String(), id)
	}
	s.CedarPolicies = b.String()
	var err error
	s.EntitiesJson, err = json.Marshal(ents)
	require.NoError(tb, err)
	p, err := CompilePolicy(s)
	require.NoError(tb, err)
	r, ok := p.Resource("r0")
	require.True(tb, ok)
	return p, AccessRequest{User: User{ID: user, TeamIDs: []uuid.UUID{team}}, Resource: r, IP: netip.MustParseAddr("192.0.2.1"),
		Action: "ssh.exec", Command: benchCommand}
}

// decideExec analyses the request command, matches its paths and decides it.
func decideExec(p *Policy, r AccessRequest) Decision {
	return p.Decide(p.withCommand(r, commandInput{Text: r.Command, Cwd: "~"}))
}

func BenchmarkExecDecision(b *testing.B) {
	p, r := execBench(b)
	require.Equal(b, []string{ObligationApproval}, decideExec(p, r).Obligations)
	for b.Loop() {
		decideExec(p, r)
	}
}
