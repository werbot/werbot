package policy

import (
	"bytes"
	"cmp"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/cedar-policy/cedar-go"
	"github.com/cedar-policy/cedar-go/types"
	"github.com/google/uuid"

	"github.com/werbot/werbot/internal/connector"
)

var countryRe = regexp.MustCompile(`^[A-Z]{2}$`)

const blocklistPolicy = `@id("blocklist")
forbid (principal, action, resource) when { context.ip_blocked };
`

// Compile turns s into a policy set and its Cedar text. The text is
// deterministic: bindings sort by ID, rules by ResourceID, list items by
// value, grants and locks by ID.
func Compile(s Snapshot) (*PolicySet, string, error) {
	var b strings.Builder
	for _, bd := range slices.SortedFunc(slices.Values(s.Bindings), func(x, y SnapshotBinding) int { return compareIDs(x.ID, y.ID) }) {
		if err := writeBinding(&b, bd); err != nil {
			return nil, "", err
		}
	}
	for _, r := range slices.SortedFunc(slices.Values(s.Rules), func(x, y ResourceRules) int { return compareIDs(x.ResourceID, y.ResourceID) }) {
		if err := writeRules(&b, r); err != nil {
			return nil, "", err
		}
	}
	writeGrants(&b, s.Grants, s.Locks)
	if err := writeLocks(&b, s.Locks); err != nil {
		return nil, "", err
	}
	b.WriteString(blocklistPolicy)
	text := b.String()

	policies, err := parse(text)
	if err != nil {
		return nil, "", err
	}
	entities := make(cedar.EntityMap, len(s.Resources))
	for _, r := range s.Resources {
		uid := cedar.NewEntityUID("Werbot::Resource", cedar.String(r.ID.String()))
		tags := make(cedar.RecordMap, len(r.Labels))
		for k, v := range r.Labels {
			tags[cedar.String(k)] = cedar.String(v)
		}
		entities[uid] = cedar.Entity{
			UID:        uid,
			Attributes: cedar.NewRecord(cedar.RecordMap{"name": cedar.String(r.Name), "type": cedar.String(r.Type)}),
			Tags:       cedar.NewRecord(tags),
		}
	}
	return &PolicySet{policies: policies, entities: entities}, text, nil
}

// parse indexes the policies of text by their @id; a duplicate id is an error,
// since PolicySet.Add would silently drop the second policy.
func parse(text string) (*cedar.PolicySet, error) {
	list, err := cedar.NewPolicyListFromBytes("werbot.cedar", []byte(text))
	if err != nil {
		return nil, fmt.Errorf("policy: parse: %w", err)
	}
	ps := cedar.NewPolicySet()
	for _, p := range list {
		id := p.Annotations()[types.AnnotationKey("id")]
		if !ps.Add(cedar.PolicyID(id), p) {
			return nil, fmt.Errorf("policy: duplicate policy id %q", id)
		}
	}
	return ps, nil
}

func writeBinding(b *strings.Builder, bd SnapshotBinding) error {
	var principal string
	switch bd.SubjectType {
	case "user":
		principal = fmt.Sprintf("principal == Werbot::User::%q", bd.SubjectID)
	case "team":
		principal = fmt.Sprintf("principal in Werbot::Team::%q", bd.SubjectID)
	default:
		return fmt.Errorf("policy: binding %s: unknown subject type %q", bd.ID, bd.SubjectType)
	}
	refs := make([]string, 0, len(bd.ResourceIDs))
	for _, id := range slices.SortedFunc(slices.Values(bd.ResourceIDs), compareIDs) {
		refs = append(refs, fmt.Sprintf("Werbot::Resource::%q", id))
	}
	fmt.Fprintf(b, "@id(\"binding:%s\")\npermit (%s, action, resource)\nwhen { [%s].contains(resource) }",
		bd.ID, principal, strings.Join(refs, ", "))
	if len(bd.Ports) > 0 {
		cond, err := portCondition(bd)
		if err != nil {
			return err
		}
		b.WriteString("\nwhen { " + cond + " }")
	}
	b.WriteString(";\n\n")
	return nil
}

// portCondition limits a binding to its ports, ranges by ascending port;
// port 0 (terminal, exec) has no target and passes.
func portCondition(bd SnapshotBinding) (string, error) {
	if err := connector.ValidatePorts(bd.Ports); err != nil {
		return "", fmt.Errorf("policy: binding %s: %w", bd.ID, err)
	}
	terms := []string{"context.port == 0"}
	for _, r := range slices.SortedFunc(slices.Values(bd.Ports), func(x, y connector.PortRange) int { return cmp.Compare(x.Port, y.Port) }) {
		last := cmp.Or(r.EndPort, r.Port)
		terms = append(terms, fmt.Sprintf("(context.port >= %d && context.port <= %d)", r.Port, last))
	}
	return strings.Join(terms, " || "), nil
}

// writeRules emits one forbid per non-empty rule list of r.
func writeRules(b *strings.Builder, r ResourceRules) error {
	forbid := func(kind string, cond []string) {
		fmt.Fprintf(b, "@id(\"rule:%s:%s\")\nforbid (principal, action, resource == Werbot::Resource::%q)\nwhen { %s };\n\n",
			kind, r.ResourceID, r.ResourceID, strings.Join(cond, " || "))
	}

	if len(r.Networks) > 0 {
		allow, deny, err := split(r.Networks, func(n NetworkRule) (string, Effect, error) {
			if !n.CIDR.IsValid() {
				return "", "", fmt.Errorf("policy: resource %s: invalid cidr", r.ResourceID)
			}
			return fmt.Sprintf("context.ip.isInRange(ip(%q))", n.CIDR), n.Effect, nil
		})
		if err != nil {
			return err
		}
		var cond []string
		if len(allow) > 0 {
			cond = append(cond, "!("+strings.Join(allow, " || ")+")")
		}
		forbid("network", append(cond, deny...))
	}

	if len(r.Countries) > 0 {
		allow, deny, err := split(r.Countries, func(c CountryRule) (string, Effect, error) {
			if !countryRe.MatchString(c.Country) {
				return "", "", fmt.Errorf("policy: resource %s: invalid country %q", r.ResourceID, c.Country)
			}
			return strconv.Quote(c.Country), c.Effect, nil
		})
		if err != nil {
			return err
		}
		// An unknown country ("") is always denied: fail-closed.
		cond := []string{`context.country == ""`}
		if len(allow) > 0 {
			cond = append(cond, "!(["+strings.Join(allow, ", ")+"].contains(context.country))")
		}
		if len(deny) > 0 {
			cond = append(cond, "["+strings.Join(deny, ", ")+"].contains(context.country)")
		}
		forbid("country", cond)
	}

	if len(r.Schedule) > 0 {
		hours := make([]int, len(r.Schedule))
		for i, s := range r.Schedule {
			if s.Weekday < 0 || s.Weekday > 6 || s.Hour < 0 || s.Hour > 23 {
				return fmt.Errorf("policy: resource %s: invalid slot %d/%d", r.ResourceID, s.Weekday, s.Hour)
			}
			hours[i] = s.Weekday*24 + s.Hour
		}
		slices.Sort(hours)
		slots := make([]string, len(hours))
		for i, h := range hours {
			slots[i] = strconv.Itoa(h)
		}
		forbid("schedule", []string{"!([" + strings.Join(slots, ", ") + "].contains(context.weekday * 24 + context.hour))"})
	}
	return nil
}

// split renders items into allow and deny terms, each sorted.
func split[T any](items []T, render func(T) (string, Effect, error)) (allow, deny []string, err error) {
	for _, it := range items {
		term, effect, err := render(it)
		if err != nil {
			return nil, nil, err
		}
		switch effect {
		case EffectAllow:
			allow = append(allow, term)
		case EffectDeny:
			deny = append(deny, term)
		default:
			return nil, nil, fmt.Errorf("policy: unknown effect %q", effect)
		}
	}
	slices.Sort(allow)
	slices.Sort(deny)
	return allow, deny, nil
}

func compareIDs(a, b uuid.UUID) int { return bytes.Compare(a[:], b[:]) }
