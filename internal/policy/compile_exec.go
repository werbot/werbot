package policy

import (
	"fmt"
	"slices"
	"strings"

	"github.com/cedar-policy/cedar-go"
	"github.com/google/uuid"

	"github.com/werbot/werbot/internal/connector"
)

// commandEffects are the effects of a command rule.
var commandEffects = []string{"allow", "deny", "approval"}

// Subjects are the people and agents a command rule applies to.
type Subjects struct {
	Users     []uuid.UUID `json:"users"`
	Teams     []uuid.UUID `json:"teams"`
	AllUsers  bool        `json:"all_users"`
	AllAgents bool        `json:"all_agents"`
}

// Selector picks resources by id or by labels.
type Selector struct {
	ResourceIDs []uuid.UUID       `json:"resource_ids"`
	Labels      map[string]string `json:"labels"`
}

// SnapshotCommandRule is a command rule with its selector expanded.
type SnapshotCommandRule struct {
	ID          uuid.UUID   `json:"id"`
	Subjects    Subjects    `json:"subjects"`
	ResourceIDs []uuid.UUID `json:"resource_ids"`
	Classes     []string    `json:"classes"`
	Effect      string      `json:"effect"` // allow | deny | approval
	Message     string      `json:"message"`
}

// SnapshotClassRule overrides the classes of commands matching Pattern.
type SnapshotClassRule struct {
	Pattern string   `json:"pattern"`
	Classes []string `json:"classes"`
}

// SnapshotCustomPolicy is an enabled custom Cedar policy.
type SnapshotCustomPolicy struct {
	ID    uuid.UUID `json:"id"`
	Cedar string    `json:"cedar"`
}

// ExpandSelector returns the snapshot resources sel picks, ascending.
func ExpandSelector(resources []SnapshotResource, sel Selector) []uuid.UUID {
	var out []uuid.UUID
	for _, r := range resources {
		if selects(sel, r) {
			out = append(out, r.ID)
		}
	}
	slices.SortFunc(out, compareIDs)
	return out
}

// selects reports whether sel picks r: by id when it lists ids, else by
// every label pair; an empty selector picks nothing.
func selects(sel Selector, r SnapshotResource) bool {
	if len(sel.ResourceIDs) > 0 {
		return slices.Contains(sel.ResourceIDs, r.ID)
	}
	return len(sel.Labels) > 0 && matches(r.Labels, sel.Labels)
}

// writeCommandRules emits one forbid per deny or approval rule, sorted by ID.
// Allow rules emit nothing (bindings already allow exec; agents: stage 12),
// nor do rules without people, without resources or without exec types.
func writeCommandRules(b *strings.Builder, rules []SnapshotCommandRule) error {
	actions := execActions()
	for _, r := range slices.SortedFunc(slices.Values(rules), func(x, y SnapshotCommandRule) int { return compareIDs(x.ID, y.ID) }) {
		if !slices.Contains(commandEffects, r.Effect) || len(r.Classes) == 0 {
			return fmt.Errorf("policy: command rule %s: invalid effect or classes", r.ID)
		}
		t := target(r.Subjects, r.ResourceIDs)
		if r.Effect == "allow" || t == "" {
			continue
		}
		f := forbid{id: "command:" + r.ID.String(), message: r.Message, actions: actions,
			when: fmt.Sprintf("%s && context.classes.containsAny([%s])", t, strings.Join(cedarStrings(r.Classes), ", "))}
		if r.Effect == "approval" {
			f.obligation, f.unless = ObligationApproval, "context.approval_granted"
		}
		writeForbid(b, f)
	}
	return nil
}

// writeAnnotation writes @key("value") with Cedar escaping of value.
func writeAnnotation(b *strings.Builder, key, value string) {
	fmt.Fprintf(b, "@%s(%s)\n", key, cedar.String(value).MarshalCedar())
}

// subjectCondition is the principal term of s; "" when s has no people.
// Teams and users are separate sets: the validator rejects a mixed set.
func subjectCondition(s Subjects) string {
	if s.AllUsers {
		return "principal is Werbot::User"
	}
	terms := slices.DeleteFunc([]string{principalIn("Werbot::Team", s.Teams), principalIn("Werbot::User", s.Users)},
		func(t string) bool { return t == "" })
	if len(terms) > 1 {
		return "(" + strings.Join(terms, " || ") + ")"
	}
	return strings.Join(terms, "")
}

// principalIn is "principal in [<ids>]"; "" without ids.
func principalIn(typ string, ids []uuid.UUID) string {
	if len(ids) == 0 {
		return ""
	}
	return "principal in [" + strings.Join(entityRefs(typ, ids), ", ") + "]"
}

// execActions are the exec actions of the registered types, by type name.
func execActions() []string {
	var out []string
	for _, t := range connector.Types() {
		if a, ok := t.ExecAction(); ok {
			out = append(out, fmt.Sprintf("Werbot::Action::%q", a))
		}
	}
	return out
}

// entityRefs are the Cedar references of ids, ascending.
func entityRefs(typ string, ids []uuid.UUID) []string {
	refs := make([]string, 0, len(ids))
	for _, id := range slices.SortedFunc(slices.Values(ids), compareIDs) {
		refs = append(refs, fmt.Sprintf("%s::%q", typ, id))
	}
	return refs
}

// cedarStrings are the Cedar string literals of values, sorted and unique.
func cedarStrings(values []string) []string {
	sorted := slices.Compact(slices.Sorted(slices.Values(values)))
	out := make([]string, len(sorted))
	for i, v := range sorted {
		out[i] = string(cedar.String(v).MarshalCedar())
	}
	return out
}
