package policy

import (
	"fmt"
	"os"
	"slices"
	"testing"

	"github.com/cedar-policy/cedar-go"
	xast "github.com/cedar-policy/cedar-go/x/exp/ast"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/werbot/werbot/internal/connector"
	_ "github.com/werbot/werbot/internal/connector/ssh"
	"github.com/werbot/werbot/internal/platform/shellclass"
)

// rawterm is a terminal type without an exec action, like telnet (which
// TestCheckAccessUnregisteredType needs unregistered here).
func init() {
	connector.Register(connector.Type{Name: "rawterm", NewConfig: func() proto.Message { return &emptypb.Empty{} },
		Validate: func(proto.Message) error { return nil }, Actions: []string{"rawterm.connect", "rawterm.shell"},
		Kinds: []connector.Kind{connector.KindTerminal}})
}

var (
	rawID         = uuid.MustParse("019a0000-0000-7000-8000-00000000000a")
	pathDenyID    = uuid.MustParse("019a0000-0000-7000-8000-0000000000e1")
	pathApprID    = uuid.MustParse("019a0000-0000-7000-8000-0000000000e2")
	pathOnlyID    = uuid.MustParse("019a0000-0000-7000-8000-0000000000e3")
	termConsoleID = uuid.MustParse("019a0000-0000-7000-8000-0000000000f1")
	termDenyID    = uuid.MustParse("019a0000-0000-7000-8000-0000000000f2")
)

// pathSnap is snap() plus a rawterm resource bound to userID and the given rules.
func pathSnap(paths []SnapshotPathRule, terminals ...SnapshotTerminalRule) Snapshot {
	s := snap()
	s.Resources = append(s.Resources, SnapshotResource{ID: rawID, Name: "switch", Type: "rawterm"})
	s.Bindings[0].ResourceIDs = []uuid.UUID{resID, rawID}
	s.PathRules, s.TerminalRules = paths, terminals
	return s
}

func pathRule(id uuid.UUID, effect, access, message string, globs ...string) SnapshotPathRule {
	return SnapshotPathRule{ID: id, Subjects: Subjects{Users: []uuid.UUID{userID}}, ResourceIDs: []uuid.UUID{resID},
		Paths: globs, Access: access, Effect: effect, Message: message}
}

func terminalRule(id uuid.UUID, mode string, resources ...uuid.UUID) SnapshotTerminalRule {
	return SnapshotTerminalRule{ID: id, Subjects: Subjects{Users: []uuid.UUID{userID}}, ResourceIDs: resources, Mode: mode}
}

// pathReq is an ssh.exec request of userID to resID with the path context.
func pathReq(hits, outside []string) Request {
	r := execReq("write")
	r.Paths, r.PathHits, r.PathOutside = []string{"/etc/hosts"}, hits, outside
	return r
}

// shellReq is a <typ>.shell request of userID to resource id.
func shellReq(action string, id uuid.UUID) Request {
	r := req("10.1.2.3", "DE", "")
	r.Action, r.ResourceID = action, id
	return r
}

func goldenPathSnap() Snapshot {
	team := pathRule(pathDenyID, "deny", "any", "No /etc", "/etc/**")
	team.Subjects = Subjects{Teams: []uuid.UUID{teamID}}
	appr := pathRule(pathApprID, "approval", "write", "", "~/deploy/*.sh", "/srv/**")
	appr.Subjects = Subjects{Users: []uuid.UUID{otherID, userID}}
	only := pathRule(pathOnlyID, "only", "write", "", "~/app/**")
	only.Subjects = Subjects{AllUsers: true}
	deny := terminalRule(termDenyID, "deny", rawID, resID)
	deny.Subjects = Subjects{Teams: []uuid.UUID{teamID}}
	return pathSnap([]SnapshotPathRule{only, appr, team}, deny, terminalRule(termConsoleID, "console", resID, rawID))
}

func TestCompilePathsGolden(t *testing.T) {
	s := goldenPathSnap()
	_, text, err := Compile(s)
	require.NoError(t, err)
	want, err := os.ReadFile("testdata/compile_paths.golden.cedar")
	require.NoError(t, err)
	assert.Equal(t, string(want), text)

	slices.Reverse(s.PathRules)
	slices.Reverse(s.TerminalRules)
	_, text2, err := Compile(s)
	require.NoError(t, err)
	assert.Equal(t, text, text2, "rules compile in ID order")
}

// TestCompiledPathPoliciesTypecheck validates every compiled path and
// terminal policy against the schema.
func TestCompiledPathPoliciesTypecheck(t *testing.T) {
	_, text, err := Compile(goldenPathSnap())
	require.NoError(t, err)
	v, err := validator()
	require.NoError(t, err)
	list, err := cedar.NewPolicyListFromBytes("werbot.cedar", []byte(text))
	require.NoError(t, err)
	require.Len(t, list, 16) // binding, three path rules, three terminal policies, blocklist, eight agent baseline policies
	for i, p := range list {
		assert.NoError(t, v.Policy(fmt.Sprint(i), (*xast.Policy)(p.AST())), i)
	}
}

func TestCompileRejectsBadPathRule(t *testing.T) {
	for name, r := range map[string]SnapshotPathRule{
		"empty glob":     pathRule(pathDenyID, "deny", "any", "", ""),
		"relative glob":  pathRule(pathDenyID, "deny", "any", "", "etc/**"),
		"unclean glob":   pathRule(pathDenyID, "deny", "any", "", "/srv/../etc"),
		"bad pattern":    pathRule(pathDenyID, "deny", "any", "", "/srv/["),
		"no paths":       pathRule(pathDenyID, "deny", "any", ""),
		"unknown access": pathRule(pathDenyID, "deny", "exec", "", "/etc/**"),
		"unknown effect": pathRule(pathDenyID, "allow", "any", "", "/etc/**"),
	} {
		_, _, err := Compile(pathSnap([]SnapshotPathRule{r}))
		assert.Error(t, err, name)
	}
	_, _, err := Compile(pathSnap(nil, terminalRule(termDenyID, "shell", resID)))
	assert.Error(t, err, "unknown terminal mode")
}

func TestPathRulesSkipWithoutPeopleOrResources(t *testing.T) {
	agents := pathRule(pathDenyID, "deny", "any", "", "/etc/**")
	agents.Subjects = Subjects{AllAgents: true}
	none := pathRule(pathApprID, "deny", "any", "", "/etc/**")
	none.ResourceIDs = nil
	term := terminalRule(termDenyID, "deny")
	_, text, err := Compile(pathSnap([]SnapshotPathRule{agents, none}, term))
	require.NoError(t, err)
	assert.NotContains(t, text, "path:"+pathApprID.String(), "a rule without resources")
	assert.NotContains(t, text, "principal in [Werbot::User::", "the agent rule has no term for people")
	assert.NotContains(t, text, "terminal:")
}

func TestBindingCoversShell(t *testing.T) {
	s := pathSnap(nil)
	assert.True(t, authorize(t, s, shellReq("ssh.shell", resID)).Allow)
	assert.True(t, authorize(t, s, shellReq("rawterm.shell", rawID)).Allow)
	s.Bindings = nil
	assert.False(t, authorize(t, s, shellReq("ssh.shell", resID)).Allow)
	assert.False(t, authorize(t, s, shellReq("rawterm.shell", rawID)).Allow)
}

func TestPathOnlyRuleDeniesOutside(t *testing.T) {
	s := pathSnap([]SnapshotPathRule{pathRule(pathOnlyID, "only", "write", "", "~/app/**")})
	id := pathOnlyID.String()
	assert.Equal(t, Decision{Reasons: []string{"path:" + id}, Message: PathOutsideMessage}, authorize(t, s, pathReq(nil, []string{id})))
	assert.True(t, authorize(t, s, pathReq(nil, nil)).Allow)

	bob := pathReq(nil, []string{id})
	bob.UserID = otherID
	s.Bindings[0].SubjectID = otherID
	assert.True(t, authorize(t, s, bob).Allow, "rule of another user")

	s.Bindings[0].SubjectID = userID
	connect := pathReq(nil, []string{id})
	connect.Action = "ssh.connect"
	assert.True(t, authorize(t, s, connect).Allow, "path rules apply to exec only")
}

func TestPathDenyHit(t *testing.T) {
	s := pathSnap([]SnapshotPathRule{pathRule(pathDenyID, "deny", "any", "No /etc", "/etc/**")})
	id := pathDenyID.String()
	assert.Equal(t, Decision{Reasons: []string{"path:" + id}, Message: "No /etc"}, authorize(t, s, pathReq([]string{id}, nil)))
	assert.True(t, authorize(t, s, pathReq(nil, nil)).Allow)
}

func TestPathApprovalObligation(t *testing.T) {
	appr := pathRule(pathApprID, "approval", "write", "", "/srv/**")
	s := pathSnap([]SnapshotPathRule{appr})
	id := pathApprID.String()
	assert.Equal(t, Decision{Reasons: []string{"path:" + id}, Obligations: []string{ObligationApproval}},
		authorize(t, s, pathReq([]string{id}, nil)))
	granted := pathReq([]string{id}, nil)
	granted.ApprovalGranted = true
	assert.True(t, authorize(t, s, granted).Allow)

	s.PathRules = append(s.PathRules, pathRule(pathOnlyID, "only", "write", "", "~/app/**"))
	d := authorize(t, s, pathReq([]string{id}, []string{pathOnlyID.String()}))
	assert.False(t, d.Allow)
	assert.Nil(t, d.Obligations, "an approval cannot lift an only rule")
}

func TestTerminalConsoleObligation(t *testing.T) {
	s := pathSnap(nil, terminalRule(termConsoleID, "console", resID))
	want := Decision{Reasons: []string{"terminal:" + termConsoleID.String()}, Obligations: []string{ObligationConsole}}
	assert.Equal(t, want, authorize(t, s, shellReq("ssh.shell", resID)))

	pty := shellReq("ssh.shell", resID)
	pty.Command = "top"
	assert.Equal(t, want, authorize(t, s, pty), "a terminal with a command gets the same shell decision")

	console := shellReq("ssh.shell", resID)
	console.Console = true
	assert.True(t, authorize(t, s, console).Allow)
	assert.True(t, authorize(t, s, execReq("write")).Allow, "exec is not a shell")

	s.Bindings = nil
	d := authorize(t, s, shellReq("ssh.shell", resID))
	assert.False(t, d.Allow)
	assert.Nil(t, d.Obligations, "a console without a binding asks for nothing")
}

// TestTerminalConsoleOnTelnetIsDeny: rawterm stands in for telnet (a shell
// action, no exec action).
func TestTerminalConsoleOnTelnetIsDeny(t *testing.T) {
	s := pathSnap(nil, terminalRule(termConsoleID, "console", rawID))
	r := shellReq("rawterm.shell", rawID)
	assert.Equal(t, Decision{Reasons: []string{"terminal:" + termConsoleID.String() + ":deny"}}, authorize(t, s, r))
	r.Console = true
	assert.False(t, authorize(t, s, r).Allow, "a type without exec has no console")
}

func TestTerminalDenyMode(t *testing.T) {
	s := pathSnap(nil, terminalRule(termDenyID, "deny", resID, rawID))
	assert.Equal(t, Decision{Reasons: []string{"terminal:" + termDenyID.String()}}, authorize(t, s, shellReq("ssh.shell", resID)))
	console := shellReq("rawterm.shell", rawID)
	console.Console = true
	assert.Equal(t, Decision{Reasons: []string{"terminal:" + termDenyID.String()}}, authorize(t, s, console))
	assert.True(t, authorize(t, s, execReq("write")).Allow)
}

func TestPathEvalRules(t *testing.T) {
	s := goldenPathSnap()
	assert.Equal(t, []shellclass.PathRule{
		{ID: pathDenyID.String(), Globs: []string{"/etc/**"}, Access: shellclass.AccessAny, Effect: "deny"},
		{ID: pathApprID.String(), Globs: []string{"~/deploy/*.sh", "/srv/**"}, Access: shellclass.AccessWrite, Effect: "approval"},
		{ID: pathOnlyID.String(), Globs: []string{"~/app/**"}, Access: shellclass.AccessWrite, Effect: "only"},
	}, s.PathEvalRules())
	assert.Empty(t, Snapshot{}.PathEvalRules())
}
