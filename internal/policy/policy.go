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
}

// Binding is an access binding as stored: one resource or a label selector.
type Binding struct {
	ID            uuid.UUID
	SubjectType   string // "user" | "team"
	SubjectID     uuid.UUID
	ResourceID    *uuid.UUID
	LabelSelector map[string]string
}

// SnapshotBinding is a binding with its selector expanded to resource ids.
type SnapshotBinding struct {
	ID          uuid.UUID   `json:"id"`
	SubjectType string      `json:"subject_type"`
	SubjectID   uuid.UUID   `json:"subject_id"`
	ResourceIDs []uuid.UUID `json:"resource_ids"`
}

// Snapshot is everything the policy of one organization depends on.
type Snapshot struct {
	OrgID     uuid.UUID          `json:"org_id"`
	Resources []SnapshotResource `json:"resources"`
	Bindings  []SnapshotBinding  `json:"bindings"`
	Rules     []ResourceRules    `json:"rules"`
}

// Request asks whether a user may perform Action on a resource.
type Request struct {
	UserID     uuid.UUID
	TeamIDs    []uuid.UUID
	ResourceID uuid.UUID
	Action     string // "ssh.connect"
	IP         netip.Addr
	Country    string // ISO 3166-1 alpha-2 or ""
	Time       time.Time
	IPBlocked  bool
}

// Decision lists the ids of the policies that decided it; "error:<id>" marks
// a policy that failed to evaluate.
type Decision struct {
	Allow   bool
	Reasons []string
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
		sb := SnapshotBinding{ID: b.ID, SubjectType: b.SubjectType, SubjectID: b.SubjectID}
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
// r.Time means now.
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
	user := cedar.Entity{
		UID:     cedar.NewEntityUID("Werbot::User", cedar.String(r.UserID.String())),
		Parents: cedar.NewEntityUIDSet(teams...),
	}
	req := cedar.Request{
		Principal: user.UID,
		Action:    cedar.NewEntityUID("Werbot::Action", cedar.String(r.Action)),
		Resource:  cedar.NewEntityUID("Werbot::Resource", cedar.String(r.ResourceID.String())),
		Context: cedar.NewRecord(cedar.RecordMap{
			"ip":         cedar.IPAddr(netip.PrefixFrom(ip, ip.BitLen())),
			"country":    cedar.String(strings.ToUpper(r.Country)),
			"weekday":    cedar.Long(t.Weekday()),
			"hour":       cedar.Long(t.Hour()),
			"ip_blocked": cedar.Boolean(r.IPBlocked),
		}),
	}
	d, diag := cedar.Authorize(ps.policies, entityGetter{ps.entities, user}, req)
	// cedar-go skips a policy that fails to evaluate: a broken forbid would allow.
	out := Decision{Allow: d == cedar.Allow && len(diag.Errors) == 0}
	for _, rs := range diag.Reasons {
		out.Reasons = append(out.Reasons, string(rs.PolicyID))
	}
	for _, e := range diag.Errors {
		out.Reasons = append(out.Reasons, "error:"+string(e.PolicyID))
	}
	return out
}

// typeHasAction reports whether action is one of the actions of the registered
// type of resource id; a resource outside the snapshot has none.
func (ps *PolicySet) typeHasAction(id uuid.UUID, action string) bool {
	e, ok := ps.entities[cedar.NewEntityUID("Werbot::Resource", cedar.String(id.String()))]
	if !ok {
		return false
	}
	v, _ := e.Attributes.Get("type")
	typ, _ := v.(cedar.String)
	t, ok := connector.Lookup(string(typ))
	return ok && slices.Contains(t.Actions, action)
}

// entityGetter adds the request principal to the compiled entities without copying them.
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
