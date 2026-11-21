package gateway

import (
	"errors"
	"fmt"
	"net/netip"
	"slices"
	"strings"
	"time"

	"github.com/cedar-policy/cedar-go"
	"github.com/cedar-policy/cedar-go/types"
	"github.com/google/uuid"

	gatewayv1 "github.com/werbot/werbot/gen/gateway/v1"
	"github.com/werbot/werbot/internal/platform/shellclass"
)

// Obligations a deny can carry.
const (
	ObligationApproval = "approval" // an approval of the command
	ObligationConsole  = "console"  // the shell is served by the command console
)

// obligations are the known @obligation names and how a request meets each;
// they mirror internal/policy/obligations.go: change both together.
var obligations = map[string]func(*AccessRequest){
	ObligationApproval: func(r *AccessRequest) { r.ApprovalGranted = true },
	ObligationConsole:  func(r *AccessRequest) { r.Console = true },
}

// Decision lists the ids of the policies that decided it (@id, else
// "policy<n>" by position); "error:<id>" marks a policy that failed to
// evaluate. A deny carries Obligations when meeting them (e.g. an approval)
// would allow, and the @message of the first deciding forbid.
type Decision struct {
	Allow       bool
	Reasons     []string
	Obligations []string
	Message     string
}

// note is what Decide reads from the annotations of a policy.
type note struct{ message, obligation string }

// compileCedar parses the snapshot policies and keys each by its @id, else by
// "policy<n>" (its position); a duplicate key is an error, since
// PolicySet.Add would drop the second policy. It also collects the notes.
func compileCedar(text string) (*cedar.PolicySet, map[cedar.PolicyID]note, error) {
	list, err := cedar.NewPolicyListFromBytes("snapshot.cedar", []byte(text))
	if err != nil {
		return nil, nil, fmt.Errorf("snapshot policies: %w", err)
	}
	ps := cedar.NewPolicySet()
	notes := map[cedar.PolicyID]note{}
	for i, p := range list {
		a := p.Annotations()
		id := cedar.PolicyID(a[types.AnnotationKey("id")])
		if id == "" {
			id = cedar.PolicyID(fmt.Sprint("policy", i))
		}
		if !ps.Add(id, p) {
			return nil, nil, fmt.Errorf("snapshot policies: duplicate policy id %q", id)
		}
		if n := (note{string(a["message"]), string(a["obligation"])}); n != (note{}) {
			notes[id] = n
		}
	}
	return ps, notes, nil
}

// compileCommandRules builds the classifier of the snapshot class rules and
// the path rules; a rule the classifier refuses, or a path rule with an
// invalid id, glob, access or effect, rejects the snapshot.
func compileCommandRules(s *gatewayv1.Snapshot) (*shellclass.Classifier, []shellclass.PathRule, error) {
	overrides := make([]shellclass.Override, len(s.GetClassRules()))
	for i, r := range s.GetClassRules() {
		classes := make([]shellclass.Class, len(r.GetClasses()))
		for j, c := range r.GetClasses() {
			classes[j] = shellclass.Class(c)
		}
		overrides[i] = shellclass.Override{Pattern: r.GetPattern(), Classes: classes}
	}
	c, err := shellclass.New(overrides)
	if err != nil {
		return nil, nil, fmt.Errorf("snapshot class rules: %w", err)
	}
	rules := make([]shellclass.PathRule, len(s.GetPathRules()))
	for i, r := range s.GetPathRules() {
		if rules[i], err = compilePathRule(r); err != nil {
			return nil, nil, err
		}
	}
	return c, rules, nil
}

// compilePathRule checks a snapshot path rule as the server compile does
// (internal/policy/compile_paths.go), plus its uuid id.
func compilePathRule(r *gatewayv1.PathRule) (shellclass.PathRule, error) {
	id, err := uuid.Parse(r.GetId())
	if err != nil || id.String() != r.GetId() {
		return shellclass.PathRule{}, errors.New("snapshot path rule: invalid id")
	}
	rule := shellclass.PathRule{ID: r.GetId(), Globs: slices.Clone(r.GetPaths()), Access: shellclass.Access(r.GetAccess()), Effect: r.GetEffect()}
	if err := rule.Validate(); err != nil {
		return shellclass.PathRule{}, fmt.Errorf("snapshot path rule %s: %w", id, err)
	}
	return rule, nil
}

// Analyze classifies command like the server's TestCommand, with the snapshot
// class rules and the host of the target resource, and lists the paths it
// touches from cwd ("~", "~/…" or absolute). No snapshot: opaque and unresolved.
func (p *Policy) Analyze(command, resourceHost, cwd string) shellclass.Analysis {
	if p == nil {
		return shellclass.Analysis{Classes: []shellclass.Class{shellclass.Opaque}, Unresolved: true}
	}
	return p.classifier.Analyze(command, resourceHost, cwd)
}

// PathRules are the snapshot path rules, by ID.
func (p *Policy) PathRules() []shellclass.PathRule {
	if p == nil {
		return nil
	}
	return p.pathRules
}

// pathSet is the compiled path rules of p, with "~" globs resolved from home
// when it is set.
func (p *Policy) pathSet(home string) shellclass.PathSet {
	switch {
	case p == nil:
		return shellclass.PathSet{}
	case home == "":
		return p.paths
	}
	return shellclass.CompilePaths(shellclass.ExpandHome(p.pathRules, home))
}

// MaskOutput is whose exec output the gateway masks: "agents", "all" or
// "none"; an older server sends none, which means "agents".
func (p *Policy) MaskOutput() string {
	if p == nil || p.maskOutput == "" {
		return "agents"
	}
	return p.maskOutput
}

// Decide decides r. It mirrors policy.(*PolicySet).Authorize in
// internal/policy/policy.go, which the gateway may not import;
// TestPolicyParity and TestPolicyParityExec keep the two equal. An invalid IP,
// an action the resource type lacks or any policy evaluation error denies.
func (p *Policy) Decide(r AccessRequest) Decision {
	if p == nil {
		return Decision{Reasons: []string{"error:compile"}}
	}
	if !r.IP.IsValid() {
		return Decision{Reasons: []string{"invalid_ip"}}
	}
	if r.Action == "" {
		r.Action = r.Resource.Type + ".connect"
	}
	if !p.typeHasAction(r.Resource.ID, r.Action) {
		return Decision{Reasons: []string{"invalid_action"}}
	}
	d, diag := p.evaluate(r)
	// cedar-go skips a policy that fails to evaluate: a broken forbid would allow.
	out := Decision{Allow: d == cedar.Allow && len(diag.Errors) == 0}
	for _, rs := range diag.Reasons {
		out.Reasons = append(out.Reasons, string(rs.PolicyID))
	}
	slices.Sort(out.Reasons) // cedar-go reports in map order
	n := len(out.Reasons)
	for _, e := range diag.Errors {
		out.Reasons = append(out.Reasons, "error:"+string(e.PolicyID))
	}
	slices.Sort(out.Reasons[n:])
	if d == cedar.Deny && len(diag.Errors) == 0 {
		out.Message, out.Obligations = p.explain(r, out.Reasons)
	}
	return out
}

// explain returns the @message of the first deciding forbid by ascending id
// and the obligations of a deny decided by forbids. It mirrors
// policy.(*PolicySet).explain in internal/policy/obligations.go: change both
// together.
func (p *Policy) explain(r AccessRequest, forbids []string) (string, []string) {
	var message string
	names := make([]string, 0, len(forbids))
	for _, id := range slices.Sorted(slices.Values(forbids)) {
		n := p.notes[cedar.PolicyID(id)]
		if message == "" {
			message = n.message
		}
		names = append(names, n.obligation)
	}
	return message, p.meet(r, names)
}

// meet returns names, sorted and unique, when each is known and r with all of
// them met is allowed; nil otherwise.
func (p *Policy) meet(r AccessRequest, names []string) []string {
	if len(names) == 0 {
		return nil
	}
	for _, n := range names {
		fulfil, ok := obligations[n]
		if !ok {
			return nil
		}
		fulfil(&r)
	}
	d, diag := p.evaluate(r)
	if d != cedar.Allow || len(diag.Errors) > 0 {
		return nil
	}
	return slices.Compact(slices.Sorted(slices.Values(names)))
}

// evaluate runs the policies on r; r.IP must be valid and r.Action set.
func (p *Policy) evaluate(r AccessRequest) (cedar.Decision, cedar.Diagnostic) {
	ip := r.IP.Unmap()
	t := r.Time
	if t.IsZero() {
		t = time.Now()
	}
	t = t.UTC()
	teams := make([]cedar.EntityUID, len(r.User.TeamIDs))
	for i, id := range r.User.TeamIDs {
		teams[i] = cedar.NewEntityUID("Werbot::Team", cedar.String(id.String()))
	}
	user := cedar.Entity{
		UID:     cedar.NewEntityUID("Werbot::User", cedar.String(r.User.ID.String())),
		Parents: cedar.NewEntityUIDSet(teams...),
	}
	req := cedar.Request{
		Principal: user.UID,
		Action:    cedar.NewEntityUID("Werbot::Action", cedar.String(r.Action)),
		Resource:  cedar.NewEntityUID("Werbot::Resource", cedar.String(r.Resource.ID.String())),
		Context: cedar.NewRecord(cedar.RecordMap{
			"ip":               cedar.IPAddr(netip.PrefixFrom(ip, ip.BitLen())),
			"country":          cedar.String(strings.ToUpper(r.User.Country)),
			"weekday":          cedar.Long(t.Weekday()),
			"hour":             cedar.Long(t.Hour()),
			"ip_blocked":       cedar.Boolean(slices.ContainsFunc(p.blocklist, func(b netip.Prefix) bool { return b.Contains(ip) })),
			"port":             cedar.Long(r.Port),
			"now":              cedar.Long(t.Unix()),
			"cli_session_id":   cedar.String(cliSession(r.CLISessionID)),
			"command":          cedar.String(r.Command),
			"classes":          stringSet(r.Classes),
			"approval_granted": cedar.Boolean(r.ApprovalGranted),
			"paths":            stringSet(r.Paths),
			"path_hits":        stringSet(r.PathHits),
			"path_outside":     stringSet(r.PathOutside),
			"console":          cedar.Boolean(r.Console),
		}),
	}
	return cedar.Authorize(p.policies, entityGetter{p.entities, user}, req)
}

// stringSet is values as a Cedar set of strings.
func stringSet(values []string) cedar.Set {
	out := make([]cedar.Value, len(values))
	for i, v := range values {
		out[i] = cedar.String(v)
	}
	return cedar.NewSet(out...)
}
