package gateway

import (
	"cmp"
	"encoding/json"
	"fmt"
	"net/netip"
	"slices"
	"strings"
	"time"

	"github.com/cedar-policy/cedar-go"
	"github.com/google/uuid"

	gatewayv1 "github.com/werbot/werbot/gen/gateway/v1"
	"github.com/werbot/werbot/internal/connector"
)

// User is an organization member resolved by the control plane.
type User struct {
	ID      uuid.UUID
	Login   string
	TeamIDs []uuid.UUID
	Country string
}

// Resource is a resource of the pool as the snapshot lists it.
type Resource struct {
	ID         uuid.UUID
	Name, Type string
	Config     []byte // protojson of the connector config
}

// AccessRequest asks whether User may connect to Resource; a zero Time means now.
type AccessRequest struct {
	User     User
	Resource Resource
	IP       netip.Addr
	Time     time.Time
}

// Policy is a compiled snapshot: the local policy decision point.
type Policy struct {
	version   string
	policies  *cedar.PolicySet
	entities  cedar.EntityMap
	blocklist []netip.Prefix
	resources []Resource // by Name
}

// CompilePolicy parses a snapshot; any malformed part is an error.
func CompilePolicy(s *gatewayv1.Snapshot) (*Policy, error) {
	ps, err := cedar.NewPolicySetFromBytes("snapshot.cedar", []byte(s.GetCedarPolicies()))
	if err != nil {
		return nil, fmt.Errorf("snapshot policies: %w", err)
	}
	p := &Policy{version: s.GetVersion(), policies: ps, entities: cedar.EntityMap{}}
	// empty entities_json is the server's deny-all snapshot
	if b := s.GetEntitiesJson(); len(b) > 0 {
		if err := json.Unmarshal(b, &p.entities); err != nil {
			return nil, fmt.Errorf("snapshot entities: %w", err)
		}
	}
	for _, c := range s.GetBlocklist() {
		pfx, err := netip.ParsePrefix(c)
		if err != nil {
			return nil, fmt.Errorf("snapshot blocklist: %w", err)
		}
		p.blocklist = append(p.blocklist, pfx)
	}
	for _, r := range s.GetResources() {
		id, err := uuid.Parse(r.GetId())
		if err != nil {
			return nil, fmt.Errorf("snapshot resource %q: invalid id", r.GetName())
		}
		p.resources = append(p.resources, Resource{ID: id, Name: r.GetName(), Type: r.GetType(), Config: r.GetConfigJson()})
	}
	slices.SortFunc(p.resources, func(a, b Resource) int { return cmp.Compare(a.Name, b.Name) })
	for i := 1; i < len(p.resources); i++ {
		if p.resources[i].Name == p.resources[i-1].Name {
			return nil, fmt.Errorf("snapshot resource %q: duplicate name", p.resources[i].Name)
		}
	}
	return p, nil
}

// Version is the snapshot version.
func (p *Policy) Version() string { return p.version }

// Resource finds a resource of the pool by name.
func (p *Policy) Resource(name string) (Resource, bool) {
	i, ok := slices.BinarySearchFunc(p.resources, name, func(r Resource, n string) int { return cmp.Compare(r.Name, n) })
	if !ok {
		return Resource{}, false
	}
	return p.resources[i], true
}

// Permitted lists the resources u may connect to, by ascending Name.
func (p *Policy) Permitted(u User, ip netip.Addr, now time.Time) []Resource {
	var out []Resource
	for _, r := range p.resources {
		if ok, _ := p.Authorize(AccessRequest{User: u, Resource: r, IP: ip, Time: now}); ok {
			out = append(out, r)
		}
	}
	return out
}

// Authorize decides r with the action "<type>.connect". It mirrors
// policy.(*PolicySet).Authorize in internal/policy/policy.go, which the gateway
// may not import; TestPolicyParity keeps the two equal. An invalid IP, an action
// the resource type lacks or any policy evaluation error denies.
func (p *Policy) Authorize(r AccessRequest) (allow bool, reasons []string) {
	if p == nil {
		return false, []string{"error:compile"}
	}
	if !r.IP.IsValid() {
		return false, []string{"invalid_ip"}
	}
	action := r.Resource.Type + ".connect"
	if !p.typeHasAction(r.Resource.ID, action) {
		return false, []string{"invalid_action"}
	}
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
		Action:    cedar.NewEntityUID("Werbot::Action", cedar.String(action)),
		Resource:  cedar.NewEntityUID("Werbot::Resource", cedar.String(r.Resource.ID.String())),
		Context: cedar.NewRecord(cedar.RecordMap{
			"ip":         cedar.IPAddr(netip.PrefixFrom(ip, ip.BitLen())),
			"country":    cedar.String(strings.ToUpper(r.User.Country)),
			"weekday":    cedar.Long(t.Weekday()),
			"hour":       cedar.Long(t.Hour()),
			"ip_blocked": cedar.Boolean(slices.ContainsFunc(p.blocklist, func(b netip.Prefix) bool { return b.Contains(ip) })),
		}),
	}
	d, diag := cedar.Authorize(p.policies, entityGetter{p.entities, user}, req)
	// cedar-go skips a policy that fails to evaluate: a broken forbid would allow.
	allow = d == cedar.Allow && len(diag.Errors) == 0
	for _, rs := range diag.Reasons {
		reasons = append(reasons, string(rs.PolicyID))
	}
	for _, e := range diag.Errors {
		reasons = append(reasons, "error:"+string(e.PolicyID))
	}
	return allow, reasons
}

// typeHasAction reports whether action is one of the actions of the registered
// type of resource id in the snapshot entities; an unknown resource has none.
func (p *Policy) typeHasAction(id uuid.UUID, action string) bool {
	e, ok := p.entities[cedar.NewEntityUID("Werbot::Resource", cedar.String(id.String()))]
	if !ok {
		return false
	}
	v, _ := e.Attributes.Get("type")
	typ, _ := v.(cedar.String)
	t, ok := connector.Lookup(string(typ))
	return ok && slices.Contains(t.Actions, action)
}

// entityGetter adds the request principal to the snapshot entities without copying them.
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
