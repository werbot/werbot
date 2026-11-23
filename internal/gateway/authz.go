package gateway

import (
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"slices"
	"time"

	"github.com/cedar-policy/cedar-go"
	"github.com/google/uuid"

	gatewayv1 "github.com/werbot/werbot/gen/gateway/v1"
	"github.com/werbot/werbot/internal/connector"
	"github.com/werbot/werbot/internal/platform/shellclass"
)

// User is an organization member resolved by the control plane.
type User struct {
	ID      uuid.UUID
	Login   string
	TeamIDs []uuid.UUID
	Country string
	Agent   *Agent // the agent run the user sponsors on this request; nil: the user
}

// Resource is a resource of the pool as the snapshot lists it.
type Resource struct {
	ID                       uuid.UUID
	Name, Type               string
	Config                   []byte        // protojson of the connector config
	IdleTimeout, MaxDuration time.Duration // 0: the gateway's Config values
	HostKeySHA256            string        // fingerprint of the pinned target host key; empty: none
}

// Endpoint is where a stream or datagram of the resource may connect, from its
// config; false means the type has none (or is unknown here).
func (r Resource) Endpoint() (connector.Endpoint, bool, error) {
	t, ok := connector.Lookup(r.Type)
	if !ok {
		return connector.Endpoint{}, false, nil
	}
	return t.EndpointOf(r.Config)
}

// AccessRequest asks whether User may perform Action on Resource; a zero Time
// means now, Port is the target port of a stream or datagram (0: terminal or exec).
type AccessRequest struct {
	User     User
	Resource Resource
	IP       netip.Addr
	Time     time.Time
	Port     uint16
	// CLISessionID is the CLI session of the user's certificate; uuid.Nil: none.
	CLISessionID uuid.UUID
	// Action is the Cedar action ("ssh.exec"); "" means "<type>.connect".
	Action string
	// Command, Classes, Paths (shellclass.PathRef texts), PathHits and
	// PathOutside (path rule ids) and ApprovalGranted describe an exec
	// request; a connect request leaves them empty.
	Command                      string
	Classes                      []string
	Paths, PathHits, PathOutside []string
	ApprovalGranted              bool
	// Console marks a shell served by the command console.
	Console bool
	// Kind is the kind of the session ("terminal" for its shell decision); "": none.
	Kind Kind
	// MFAAgeSeconds is the age of the user's last second factor of
	// MFAMethod ("totp" | "webauthn"); MFAMethod "" means none.
	MFAAgeSeconds int64
	MFAMethod     string
}

// Policy is a compiled snapshot: the local policy decision point.
type Policy struct {
	version     string
	policies    *cedar.PolicySet
	entities    cedar.EntityMap
	blocklist   []netip.Prefix
	resources   []Resource                // by Name
	members     map[uuid.UUID][]uuid.UUID // org members and their teams; empty: the server sends none
	userCAs     [][]byte                  // wire form of the organization's user CA keys; empty: no certificate
	expirations []int64                   // unix seconds of grant and lock expiries, ascending
	lockMode    string                    // "": best_effort
	notes       map[cedar.PolicyID]note   // @message and @obligation of the policies that have one
	classifier  *shellclass.Classifier    // built-in classes and the snapshot's class rules
	pathRules   []shellclass.PathRule     // the snapshot's path rules, by ID
	paths       shellclass.PathSet        // pathRules compiled
	maskOutput  string                    // "": an older server, agents
}

// CompilePolicy parses a snapshot; any malformed part is an error.
func CompilePolicy(s *gatewayv1.Snapshot) (*Policy, error) {
	ps, notes, err := compileCedar(s.GetCedarPolicies())
	if err != nil {
		return nil, err
	}
	p := &Policy{version: s.GetVersion(), policies: ps, notes: notes, entities: cedar.EntityMap{},
		expirations: slices.Sorted(slices.Values(s.GetExpirations())), lockMode: s.GetLockMode(), maskOutput: s.GetMaskOutput()}
	// empty entities_json is the server's deny-all snapshot
	if b := s.GetEntitiesJson(); len(b) > 0 {
		if err := json.Unmarshal(b, &p.entities); err != nil {
			return nil, fmt.Errorf("snapshot entities: %w", err)
		}
	}
	if p.blocklist, err = compileBlocklist(s.GetBlocklist()); err != nil {
		return nil, err
	}
	if p.classifier, p.pathRules, err = compileCommandRules(s); err != nil {
		return nil, err
	}
	p.paths = shellclass.CompilePaths(p.pathRules)
	for _, r := range s.GetResources() {
		id, err := uuid.Parse(r.GetId())
		if err != nil {
			return nil, fmt.Errorf("snapshot resource %q: invalid id", r.GetName())
		}
		p.resources = append(p.resources, Resource{ID: id, Name: r.GetName(), Type: r.GetType(), Config: r.GetConfigJson(),
			IdleTimeout: max(r.GetIdleTimeout().AsDuration(), 0), MaxDuration: max(r.GetMaxDuration().AsDuration(), 0),
			HostKeySHA256: r.GetHostKeySha256()})
	}
	if p.members, p.userCAs, err = compileSubjects(s); err != nil {
		return nil, err
	}
	slices.SortFunc(p.resources, func(a, b Resource) int { return cmp.Compare(a.Name, b.Name) })
	for i := 1; i < len(p.resources); i++ {
		if p.resources[i].Name == p.resources[i-1].Name {
			return nil, fmt.Errorf("snapshot resource %q: duplicate name", p.resources[i].Name)
		}
	}
	return p, nil
}

func compileBlocklist(cidrs []string) ([]netip.Prefix, error) {
	out := make([]netip.Prefix, 0, len(cidrs))
	for _, c := range cidrs {
		pfx, err := netip.ParsePrefix(c)
		if err != nil {
			return nil, fmt.Errorf("snapshot blocklist: %w", err)
		}
		out = append(out, pfx)
	}
	return out, nil
}

func compileMembers(ms []*gatewayv1.Member) (map[uuid.UUID][]uuid.UUID, error) {
	out := make(map[uuid.UUID][]uuid.UUID, len(ms))
	for _, m := range ms {
		uid, err := uuid.Parse(m.GetUserId())
		if err != nil {
			return nil, errors.New("snapshot member: invalid user_id")
		}
		teams := make([]uuid.UUID, len(m.GetTeamIds()))
		for i, t := range m.GetTeamIds() {
			if teams[i], err = uuid.Parse(t); err != nil {
				return nil, errors.New("snapshot member: invalid team_id")
			}
		}
		out[uid] = teams
	}
	return out, nil
}

// withTeams returns u with the teams the snapshot lists for them; false when u
// is not an organization member. A snapshot without members (an older server)
// knows no one: u comes back as it is.
func (p *Policy) withTeams(u User) (User, bool) {
	if p == nil || len(p.members) == 0 {
		return u, true
	}
	teams, ok := p.members[u.ID]
	u.TeamIDs = teams
	return u, ok
}

// resourceByID finds a resource of the pool by id.
// ponytail: linear scan per re-evaluated session; index by id if pools reach thousands of resources.
func (p *Policy) resourceByID(id uuid.UUID) (Resource, bool) {
	if p == nil {
		return Resource{}, false
	}
	i := slices.IndexFunc(p.resources, func(r Resource) bool { return r.ID == id })
	if i < 0 {
		return Resource{}, false
	}
	return p.resources[i], true
}

// Version is the snapshot version.
func (p *Policy) Version() string {
	if p == nil {
		return ""
	}
	return p.version
}

// Resource finds a resource of the pool by name.
func (p *Policy) Resource(name string) (Resource, bool) {
	if p == nil {
		return Resource{}, false
	}
	i, ok := slices.BinarySearchFunc(p.resources, name, func(r Resource, n string) int { return cmp.Compare(r.Name, n) })
	if !ok {
		return Resource{}, false
	}
	return p.resources[i], true
}

// Permitted lists the resources u may connect to, by ascending Name; a
// resource that needs a fresh second factor first is listed too. An agent's
// decision never carries mfa (mergeAgent). It mirrors
// policy.(*Service).AccessibleResources: change both together.
func (p *Policy) Permitted(u User, ip netip.Addr, now time.Time) []Resource {
	if p == nil {
		return nil
	}
	var out []Resource
	for _, r := range p.resources {
		if d := p.Decide(AccessRequest{User: u, Resource: r, IP: ip, Time: now}); d.Allow || slices.Equal(d.Obligations, []string{ObligationMFA}) {
			out = append(out, r)
		}
	}
	return out
}

// Authorize is Decide(r).Allow and its Reasons.
func (p *Policy) Authorize(r AccessRequest) (allow bool, reasons []string) {
	d := p.Decide(r)
	return d.Allow, d.Reasons
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

// agentGetter adds the agent of an agent's pass, which the snapshot may not
// list yet (AuthorizeAgent), to an entityGetter.
type agentGetter struct {
	cedar.EntityGetter
	agent cedar.Entity
}

func (g agentGetter) Get(uid cedar.EntityUID) (cedar.Entity, bool) {
	if uid == g.agent.UID {
		return g.agent, true
	}
	return g.EntityGetter.Get(uid)
}

// cliSession is id as context.cli_session_id: uuid.Nil (no CLI session) is "",
// which no cli_session lock matches.
func cliSession(id uuid.UUID) string {
	if id == uuid.Nil {
		return ""
	}
	return id.String()
}
