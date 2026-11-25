// Package policy compiles an organization's access snapshot to Cedar and
// authorizes requests against it. Every error denies (fail-closed).
package policy

import (
	"encoding/json"
	"net/netip"
	"slices"
	"strings"
	"time"

	"github.com/cedar-policy/cedar-go"
	"github.com/google/uuid"

	"github.com/werbot/werbot/internal/connector"
	"github.com/werbot/werbot/internal/platform/rpcid"
)

// Effect of a network or country rule.
type Effect string

// Rule effects.
const (
	EffectAllow Effect = "allow"
	EffectDeny  Effect = "deny"
)

// NetworkRule allows or denies a client CIDR.
type NetworkRule struct {
	CIDR   netip.Prefix `json:"cidr"`
	Effect Effect       `json:"effect"`
}

// CountryRule allows or denies an ISO 3166-1 alpha-2 country.
type CountryRule struct {
	Country string `json:"country"`
	Effect  Effect `json:"effect"`
}

// Slot is one allowed hour of the week in UTC; Weekday 0 is Sunday.
type Slot struct {
	Weekday int `json:"weekday"`
	Hour    int `json:"hour"`
}

// ResourceRules are the network, country and schedule rules of one resource.
// An empty list imposes no restriction.
type ResourceRules struct {
	ResourceID uuid.UUID     `json:"resource_id"`
	Networks   []NetworkRule `json:"networks"`
	Countries  []CountryRule `json:"countries"`
	Schedule   []Slot        `json:"schedule"`
}

// SnapshotResource is a resource as the policy sees it.
type SnapshotResource struct {
	ID     uuid.UUID         `json:"id"`
	Name   string            `json:"name"`
	Type   string            `json:"type"`
	Labels map[string]string `json:"labels"`
	// Host is the endpoint host: the resource host of the egress class.
	Host string `json:"host,omitempty"`
}

// Binding is an access binding as stored: one resource or a label selector.
type Binding struct {
	ID            uuid.UUID
	SubjectType   string // "user" | "team"
	SubjectID     uuid.UUID
	ResourceID    *uuid.UUID
	LabelSelector map[string]string
	Ports         []connector.PortRange // empty: every port
}

// SnapshotBinding is a binding with its selector expanded to resource ids.
type SnapshotBinding struct {
	ID          uuid.UUID   `json:"id"`
	SubjectType string      `json:"subject_type"`
	SubjectID   uuid.UUID   `json:"subject_id"`
	ResourceIDs []uuid.UUID `json:"resource_ids"`
	// Ports restrict stream and datagram sessions; empty: every port.
	Ports []connector.PortRange `json:"ports,omitempty"`
}

// Snapshot is everything the policy of one organization depends on.
type Snapshot struct {
	OrgID     uuid.UUID          `json:"org_id"`
	Resources []SnapshotResource `json:"resources"`
	Bindings  []SnapshotBinding  `json:"bindings"`
	Rules     []ResourceRules    `json:"rules"`
	Grants    []SnapshotGrant    `json:"grants"`
	Locks     []SnapshotLock     `json:"locks"`
	// CommandRules have their selectors expanded.
	CommandRules   []SnapshotCommandRule  `json:"command_rules"`
	ClassRules     []SnapshotClassRule    `json:"class_rules"`
	CustomPolicies []SnapshotCustomPolicy `json:"custom_policies"` // enabled only
	PathRules      []SnapshotPathRule     `json:"path_rules"`
	TerminalRules  []SnapshotTerminalRule `json:"terminal_rules"`
	MFARules       []SnapshotMFARule      `json:"mfa_rules"`
	Agents         []SnapshotAgent        `json:"agents"`
	BlockedRuns    []uuid.UUID            `json:"blocked_runs"` // paused or recently ended runs
}

// Request asks whether a user or an agent of the user may perform Action.
type Request struct {
	UserID     uuid.UUID // the sponsor of AgentID
	AgentID    *uuid.UUID
	RunID      *uuid.UUID
	Kind       string // connector.Kind of the session; "" when none
	TeamIDs    []uuid.UUID
	ResourceID uuid.UUID
	Action     string // "ssh.connect"
	IP         netip.Addr
	Country    string // ISO 3166-1 alpha-2 or ""
	Time       time.Time
	IPBlocked  bool
	Port       uint16 // target port of a stream or datagram; 0: terminal or exec
	// CLISessionID is the CLI session of the user certificate; uuid.Nil: none.
	CLISessionID uuid.UUID
	// Command, Classes and ApprovalGranted describe an exec request; a
	// connect request leaves them empty.
	Command         string
	Classes         []string
	ApprovalGranted bool
	// Paths are the paths of the command, PathHits and PathOutside the ids
	// of the path rules it hits or leaves (shellclass.EvalPaths); Console
	// marks a shell the command console serves.
	Paths, PathHits, PathOutside []string
	Console                      bool
	// MFAAgeSeconds is the age of the user's last second factor of
	// MFAMethod ("totp" | "webauthn"); MFAMethod "" means none (mfaAge).
	MFAAgeSeconds int64
	MFAMethod     string
}

// Decision lists the ids of the policies that decided it; "error:<id>" marks
// a policy that failed to evaluate. A deny carries Obligations when meeting
// them (e.g. an approval) would allow, and the @message of the first
// deciding forbid.
type Decision struct {
	Allow       bool
	Reasons     []string
	Obligations []string
	Message     string
}

// PolicySet is a compiled snapshot.
type PolicySet struct { //nolint:revive // name fixed by contract §3.2
	policies *cedar.PolicySet
	entities cedar.EntityMap
}

// EntitiesJSON is the Cedar JSON of the compiled entities; cedar sorts them
// by UID, so equal snapshots encode to equal bytes.
func (ps *PolicySet) EntitiesJSON() ([]byte, error) {
	return json.Marshal(ps.entities)
}

// ExpandBindings resolves label selectors: a resource matches when it has every pair.
func ExpandBindings(resources []SnapshotResource, bindings []Binding) []SnapshotBinding {
	out := make([]SnapshotBinding, 0, len(bindings))
	for _, b := range bindings {
		sb := SnapshotBinding{ID: b.ID, SubjectType: b.SubjectType, SubjectID: b.SubjectID, Ports: b.Ports}
		if b.ResourceID != nil {
			sb.ResourceIDs = []uuid.UUID{*b.ResourceID}
		} else if len(b.LabelSelector) > 0 { // an empty selector matches nothing, not everything
			for _, r := range resources {
				if matches(r.Labels, b.LabelSelector) {
					sb.ResourceIDs = append(sb.ResourceIDs, r.ID)
				}
			}
		}
		out = append(out, sb)
	}
	return out
}

func matches(labels, selector map[string]string) bool {
	for k, v := range selector {
		if got, ok := labels[k]; !ok || got != v {
			return false
		}
	}
	return true
}

// Authorize decides r. An invalid IP, an action that is not one of the
// resource type's actions or any policy evaluation error denies. A zero
// r.Time means now. A deny also carries the @message and the obligations of
// the forbids that decided it. An agent's request passes twice (authorizeAgent).
func (ps *PolicySet) Authorize(r Request) Decision {
	if ps == nil {
		return Decision{Reasons: []string{"error:compile"}}
	}
	if !r.IP.IsValid() {
		return Decision{Reasons: []string{"invalid_ip"}}
	}
	if !ps.typeHasAction(r.ResourceID, r.Action) {
		return Decision{Reasons: []string{"invalid_action"}}
	}
	if r.AgentID != nil {
		return ps.authorizeAgent(r)
	}
	return ps.decidePass(r, userUID(r.UserID))
}

// decidePass decides r with principal p: the user of r or its agent.
func (ps *PolicySet) decidePass(r Request, p cedar.EntityUID) Decision {
	d, diag := ps.evaluate(r, p)
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
		out.Message, out.Obligations = ps.explain(r, p, out.Reasons)
	}
	return out
}

// evaluate runs the policies on r with principal p; r.IP must be valid.
func (ps *PolicySet) evaluate(r Request, p cedar.EntityUID) (cedar.Decision, cedar.Diagnostic) {
	ip := r.IP.Unmap()
	t := r.Time
	if t.IsZero() {
		t = time.Now()
	}
	t = t.UTC()
	teams := make([]cedar.EntityUID, len(r.TeamIDs))
	for i, id := range r.TeamIDs {
		teams[i] = cedar.NewEntityUID("Werbot::Team", cedar.String(id.String()))
	}
	user := cedar.Entity{UID: userUID(r.UserID), Parents: cedar.NewEntityUIDSet(teams...)}
	req := cedar.Request{
		Principal: p,
		Action:    cedar.NewEntityUID("Werbot::Action", cedar.String(r.Action)),
		Resource:  cedar.NewEntityUID("Werbot::Resource", cedar.String(r.ResourceID.String())),
		Context: cedar.NewRecord(cedar.RecordMap{
			"ip":               cedar.IPAddr(netip.PrefixFrom(ip, ip.BitLen())),
			"country":          cedar.String(strings.ToUpper(r.Country)),
			"weekday":          cedar.Long(t.Weekday()),
			"hour":             cedar.Long(t.Hour()),
			"ip_blocked":       cedar.Boolean(r.IPBlocked),
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
			"run_id":           cedar.String(rpcid.String(r.RunID)), // no run is "", which no run id matches
			"kind":             cedar.String(r.Kind),
			"mfa_age":          cedar.Long(mfaAge(r)),
			"mfa_method":       cedar.String(r.MFAMethod),
		}),
	}
	return cedar.Authorize(ps.policies, entityGetter{ps.entities, user}, req)
}

// entityGetter adds the request user to the compiled entities without copying them.
type entityGetter struct {
	entities cedar.EntityMap
	user     cedar.Entity
}

func (g entityGetter) Get(uid cedar.EntityUID) (cedar.Entity, bool) {
	if uid == g.user.UID {
		return g.user, true
	}
	e, ok := g.entities[uid]
	return e, ok
}
