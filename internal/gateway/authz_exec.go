package gateway

import (
	"errors"
	"fmt"
	"math"
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
	ObligationMFA      = "mfa"      // a fresh second factor of the user (mfa_rule)
)

// obligations are the known @obligation names and how a request meets each;
// they mirror internal/policy/obligations.go: change both together.
var obligations = map[string]func(*AccessRequest){
	ObligationApproval: func(r *AccessRequest) { r.ApprovalGranted = true },
	ObligationConsole:  func(r *AccessRequest) { r.Console = true },
	ObligationMFA:      func(r *AccessRequest) { r.MFAAgeSeconds, r.MFAMethod = 0, "webauthn" },
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

// Decide decides r: an agent's request in two passes (AuthorizeAgent), a
// person's in one. It mirrors policy.(*PolicySet).Authorize in
// internal/policy/policy.go, which the gateway may not import;
// TestPolicyParity, TestPolicyParityExec and TestPolicyParityAgents keep the
// two equal. An invalid IP, an action the resource type lacks or any policy
// evaluation error denies.
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
	if r.User.Agent != nil {
		return p.AuthorizeAgent(r)
	}
	return p.decidePass(r, nil)
}

// decidePass decides r with the user of r as principal, or with agent in an
// agent's pass. It mirrors policy.(*PolicySet).decidePass: change both together.
func (p *Policy) decidePass(r AccessRequest, agent *cedar.Entity) Decision {
	d, diag := p.evaluate(r, agent)
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
		out.Message, out.Obligations = p.explain(r, agent, out.Reasons)
	}
	return out
}

// explain returns the @message of the first deciding forbid by ascending id
// and the obligations of a deny decided by forbids, met for the principal of
// the pass (agent, or the user when nil). It mirrors
// policy.(*PolicySet).explain in internal/policy/obligations.go: change both
// together.
func (p *Policy) explain(r AccessRequest, agent *cedar.Entity, forbids []string) (string, []string) {
	var message string
	names := make([]string, 0, len(forbids))
	for _, id := range slices.Sorted(slices.Values(forbids)) {
		n := p.notes[cedar.PolicyID(id)]
		if message == "" {
			message = n.message
		}
		names = append(names, n.obligation)
	}
	return message, p.meet(r, agent, names)
}

// meet returns names, sorted and unique, when each is known and r with all of
// them met is allowed for the principal of the pass; nil otherwise.
func (p *Policy) meet(r AccessRequest, agent *cedar.Entity, names []string) []string {
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
	d, diag := p.evaluate(r, agent)
	if d != cedar.Allow || len(diag.Errors) > 0 {
		return nil
	}
	return slices.Compact(slices.Sorted(slices.Values(names)))
}

// evaluate runs the policies on r with the user of r as principal, or with
// agent in an agent's pass; r.IP must be valid and r.Action set.
func (p *Policy) evaluate(r AccessRequest, agent *cedar.Entity) (cedar.Decision, cedar.Diagnostic) {
	ip := r.IP.Unmap()
	t := r.at().UTC()
	teams := make([]cedar.EntityUID, len(r.User.TeamIDs))
	for i, id := range r.User.TeamIDs {
		teams[i] = cedar.NewEntityUID("Werbot::Team", cedar.String(id.String()))
	}
	user := cedar.Entity{
		UID:     cedar.NewEntityUID("Werbot::User", cedar.String(r.User.ID.String())),
		Parents: cedar.NewEntityUIDSet(teams...),
	}
	principal, getter := user.UID, cedar.EntityGetter(entityGetter{p.entities, user})
	if agent != nil {
		principal, getter = agent.UID, agentGetter{getter, *agent}
	}
	req := cedar.Request{
		Principal: principal,
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
			"run_id":           cedar.String(idString(r.User.runID())),
			"kind":             cedar.String(r.Kind),
			"mfa_age":          cedar.Long(r.mfaAge()),
			"mfa_method":       cedar.String(r.MFAMethod),
		}),
	}
	return cedar.Authorize(p.policies, getter, req)
}

// noMFA is context.mfa_age of a request without a second factor: no rule's
// max_age is met.
const noMFA int64 = math.MaxInt64

// mfaAge is r as context.mfa_age: noMFA without a method or with a negative
// age (fail-closed). It mirrors policy.mfaAge: change both together.
func (r AccessRequest) mfaAge() int64 {
	if r.MFAMethod == "" || r.MFAAgeSeconds < 0 {
		return noMFA
	}
	return r.MFAAgeSeconds
}

// MFAStrength is the factor that meets the mfa obligation of r: "any" when
// a TOTP code confirmed now would allow r, "webauthn" otherwise. It mirrors
// policy.MFAStrength: change both together.
func (p *Policy) MFAStrength(r AccessRequest) string {
	r.MFAAgeSeconds, r.MFAMethod = 0, "totp"
	if p.Decide(r).Allow {
		return "any"
	}
	return "webauthn"
}

// at is the time of r; zero is now.
func (r AccessRequest) at() time.Time {
	if r.Time.IsZero() {
		return time.Now()
	}
	return r.Time
}

// stringSet is values as a Cedar set of strings.
func stringSet(values []string) cedar.Set {
	out := make([]cedar.Value, len(values))
	for i, v := range values {
		out[i] = cedar.String(v)
	}
	return cedar.NewSet(out...)
}
