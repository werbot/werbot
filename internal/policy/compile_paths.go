package policy

import (
	"cmp"
	"fmt"
	"slices"
	"strings"

	"github.com/cedar-policy/cedar-go"
	"github.com/google/uuid"

	"github.com/werbot/werbot/internal/connector"
	"github.com/werbot/werbot/internal/platform/shellclass"
)

// PathOutsideMessage is the @message of an "only" path rule without its own.
const PathOutsideMessage = "path outside allowed folders"

// terminalModes are the modes of a terminal rule.
var terminalModes = []string{"console", "deny"}

// SnapshotPathRule is a folder rule with its selector expanded.
type SnapshotPathRule struct {
	ID          uuid.UUID   `json:"id"`
	Subjects    Subjects    `json:"subjects"`
	ResourceIDs []uuid.UUID `json:"resource_ids"`
	Paths       []string    `json:"paths"`   // globs
	Access      string      `json:"access"`  // read | write | any
	Effect      string      `json:"effect"`  // only | deny | approval
	Message     string      `json:"message"` // "" for only: PathOutsideMessage
}

// SnapshotTerminalRule is a terminal rule with its selector expanded.
type SnapshotTerminalRule struct {
	ID          uuid.UUID   `json:"id"`
	Subjects    Subjects    `json:"subjects"`
	ResourceIDs []uuid.UUID `json:"resource_ids"`
	Mode        string      `json:"mode"` // console | deny
}

// PathEvalRules are the path rules of s by ID for shellclass.EvalPaths.
func (s Snapshot) PathEvalRules() []shellclass.PathRule {
	out := make([]shellclass.PathRule, 0, len(s.PathRules))
	for _, r := range slices.SortedFunc(slices.Values(s.PathRules), func(x, y SnapshotPathRule) int { return compareIDs(x.ID, y.ID) }) {
		out = append(out, shellclass.PathRule{ID: r.ID.String(), Globs: slices.Clone(r.Paths), Access: shellclass.Access(r.Access), Effect: r.Effect})
	}
	return out
}

// forbid is one compiled forbid policy; one without actions is not written.
type forbid struct {
	id, message, obligation string
	actions                 []string
	when, unless            string
}

func writeForbid(b *strings.Builder, f forbid) {
	if len(f.actions) == 0 {
		return
	}
	fmt.Fprintf(b, "@id(%q)\n", f.id)
	if f.message != "" {
		writeAnnotation(b, "message", f.message)
	}
	if f.obligation != "" {
		writeAnnotation(b, "obligation", f.obligation)
	}
	fmt.Fprintf(b, "forbid (principal, action in [%s], resource)\nwhen { %s }", strings.Join(f.actions, ", "), f.when)
	if f.unless != "" {
		fmt.Fprintf(b, "\nunless { %s }", f.unless)
	}
	b.WriteString(";\n\n")
}

// target is the principal term subj and the resource term of a rule; ""
// when the rule has no subjects or no resources.
func target(subj string, resources []uuid.UUID) string {
	if subj == "" || len(resources) == 0 {
		return ""
	}
	return fmt.Sprintf("%s && [%s].contains(resource)", subj, strings.Join(entityRefs("Werbot::Resource", resources), ", "))
}

// writePathRules emits one forbid on the exec actions per path rule of people
// or agents, sorted by ID; rules without subjects or resources emit nothing.
// An invalid glob, access or effect fails the compile.
func writePathRules(b *strings.Builder, rules []SnapshotPathRule) error {
	actions := execActions()
	for _, r := range slices.SortedFunc(slices.Values(rules), func(x, y SnapshotPathRule) int { return compareIDs(x.ID, y.ID) }) {
		if err := r.validate(); err != nil {
			return err
		}
		if t := target(ruleSubject(r.Subjects), r.ResourceIDs); t != "" {
			writeForbid(b, r.forbid(t, actions))
		}
	}
	return nil
}

func (r SnapshotPathRule) validate() error {
	if err := (shellclass.PathRule{Globs: r.Paths, Access: shellclass.Access(r.Access), Effect: r.Effect}).Validate(); err != nil {
		return fmt.Errorf("policy: path rule %s: %w", r.ID, err)
	}
	return nil
}

// forbid is the policy of r: deny and approval forbid on a hit, only on a
// path outside its globs.
func (r SnapshotPathRule) forbid(target string, actions []string) forbid {
	f := forbid{id: "path:" + r.ID.String(), message: r.Message, actions: actions,
		when: fmt.Sprintf("%s && context.path_hits.contains(%q)", target, r.ID)}
	switch r.Effect {
	case shellclass.EffectOnly:
		f.message = cmp.Or(r.Message, PathOutsideMessage)
		f.when = fmt.Sprintf("%s && context.path_outside.contains(%q)", target, r.ID)
	case shellclass.EffectApproval:
		f.obligation, f.unless = ObligationApproval, "context.approval_granted"
	}
	return f
}

// writeTerminalRules emits the forbids of the terminal rules on the shell
// actions, sorted by ID. A console rule forbids the shell of a type with an
// exec action unless the console serves it, and plainly forbids the shell of
// every other type (":deny"); a deny rule forbids every shell. Rules without
// people or resources emit nothing: agents never get a terminal
// (agent:baseline:terminal). An unknown mode fails the compile.
func writeTerminalRules(b *strings.Builder, rules []SnapshotTerminalRule) error {
	console, plain := shellActions()
	all := slices.Sorted(slices.Values(slices.Concat(console, plain)))
	for _, r := range slices.SortedFunc(slices.Values(rules), func(x, y SnapshotTerminalRule) int { return compareIDs(x.ID, y.ID) }) {
		if !slices.Contains(terminalModes, r.Mode) {
			return fmt.Errorf("policy: terminal rule %s: invalid mode", r.ID)
		}
		t := target(subjectCondition(r.Subjects), r.ResourceIDs)
		id := "terminal:" + r.ID.String()
		switch {
		case t == "":
		case r.Mode == "deny":
			writeForbid(b, forbid{id: id, actions: all, when: t})
		default:
			writeForbid(b, forbid{id: id, obligation: ObligationConsole, actions: console, when: t, unless: "context.console"})
			writeForbid(b, forbid{id: id + ":deny", actions: plain, when: t})
		}
	}
	return nil
}

// shellActions are the shell actions of the registered types, by type name:
// console of the types that also have an exec action, plain of the others.
func shellActions() (console, plain []string) {
	for _, t := range connector.Types() {
		a, ok := t.ShellAction()
		if !ok {
			continue
		}
		ref := fmt.Sprintf("Werbot::Action::%q", a)
		if _, exec := t.ExecAction(); exec {
			console = append(console, ref)
		} else {
			plain = append(plain, ref)
		}
	}
	return console, plain
}

// stringSet is values as a Cedar set of strings.
func stringSet(values []string) cedar.Set {
	out := make([]cedar.Value, len(values))
	for i, v := range values {
		out[i] = cedar.String(v)
	}
	return cedar.NewSet(out...)
}
