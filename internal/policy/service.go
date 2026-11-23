package policy

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/netip"
	"slices"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/werbot/werbot/internal/connector"
	"github.com/werbot/werbot/internal/platform/apperr"
	"github.com/werbot/werbot/internal/platform/dbx"
	"github.com/werbot/werbot/internal/platform/principal"
)

// ResourceSource lists the resources of an organization.
type ResourceSource interface {
	PolicyResources(ctx context.Context, orgID uuid.UUID) ([]SnapshotResource, error)
}

// BindingSource lists the access bindings of an organization.
type BindingSource interface {
	PolicyBindings(ctx context.Context, orgID uuid.UUID) ([]Binding, error)
}

// TeamSource lists the teams of a user in an organization.
type TeamSource interface {
	UserTeams(ctx context.Context, orgID, userID uuid.UUID) ([]uuid.UUID, error)
}

// Deps are the dependencies of a Service.
type Deps struct {
	Pool      *pgxpool.Pool
	Resources ResourceSource
	Bindings  BindingSource
	Teams     TeamSource
	Grants    GrantSource // nil: no grants
	Locks     LockSource  // nil: no locks
	Agents    AgentSource // nil: no agents
	Logger    *slog.Logger
}

// Service owns resource rules and decides access requests.
type Service struct {
	deps Deps
	geo  atomic.Pointer[loadedGeoIP] // nil until ReloadGeoIP finds a database
}

// loadedGeoIP is a database and the month of the geoip_db row it came from.
type loadedGeoIP struct {
	GeoIP
	month time.Time
}

// New creates a Service. It reads nothing: the GeoIP database stays empty
// (every country is "", so country rules deny) until ReloadGeoIP.
func New(d Deps) *Service {
	d.Logger = cmp.Or(d.Logger, slog.Default())
	return &Service{deps: d}
}

// actor is the principal's user, or uuid.Nil for a call without one.
func actor(ctx context.Context) uuid.UUID {
	p, _ := principal.From(ctx)
	return p.UserID
}

// ReloadGeoIP replaces the GeoIP database with the geoip_db row. No row keeps
// the current database, and so does a row of the month already loaded (the
// blob is not read). A broken one keeps the current database too and returns
// the error; the next call retries.
func (s *Service) ReloadGeoIP(ctx context.Context) error {
	var (
		month pgtype.Date
		data  []byte
		same  bool // the stored month is the loaded one: no blob read
	)
	err := dbx.System(ctx, s.deps.Pool, dbx.ReasonMaintenance, func(tx pgx.Tx) (err error) {
		st := newStore(tx)
		if month, err = st.GetGeoIPMonth(ctx); err != nil {
			return err
		}
		if cur := s.geo.Load(); cur != nil && cur.month.Equal(month.Time) {
			same = true
			return nil
		}
		data, err = st.GetGeoIPData(ctx)
		return err
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("policy: reload geoip: %w", err)
	}
	if same {
		return nil
	}
	g, err := OpenGeoIP(data)
	if err != nil {
		return fmt.Errorf("policy: reload geoip: %w", err)
	}
	s.geo.Store(&loadedGeoIP{GeoIP: g, month: month.Time})
	return nil
}

// Country resolves ip with the current GeoIP database; "" without one.
func (s *Service) Country(ip netip.Addr) string {
	if g := s.geo.Load(); g != nil {
		return g.Country(ip)
	}
	return ""
}

// Blocklist returns the blocked networks.
func (s *Service) Blocklist(ctx context.Context) ([]netip.Prefix, error) {
	var out []netip.Prefix
	err := dbx.System(ctx, s.deps.Pool, dbx.ReasonMaintenance, func(tx pgx.Tx) (err error) {
		out, err = newStore(tx).ListBlocklist(ctx)
		return err
	})
	return out, err
}

// Snapshot collects everything the policy of orgID depends on.
func (s *Service) Snapshot(ctx context.Context, orgID uuid.UUID) (Snapshot, error) {
	snap, _, err := s.load(ctx, orgID, netip.Addr{})
	return snap, err
}

// load builds the snapshot of orgID; a valid ip is checked against the
// blocklist in the same transaction.
func (s *Service) load(ctx context.Context, orgID uuid.UUID, ip netip.Addr) (snap Snapshot, blocked bool, err error) {
	resources, err := s.deps.Resources.PolicyResources(ctx, orgID)
	if err != nil {
		return Snapshot{}, false, err
	}
	bindings, err := s.deps.Bindings.PolicyBindings(ctx, orgID)
	if err != nil {
		return Snapshot{}, false, err
	}
	snap = Snapshot{OrgID: orgID, Resources: resources, Bindings: ExpandBindings(resources, bindings)}
	if err = s.loadSources(ctx, orgID, &snap); err != nil {
		return Snapshot{}, false, err
	}
	err = dbx.InOrg(ctx, s.deps.Pool, orgID, actor(ctx), func(tx pgx.Tx) (err error) {
		blocked, err = readOrg(ctx, newStore(tx), ip, &snap)
		return err
	})
	if err != nil {
		return Snapshot{}, false, err
	}
	return snap, blocked, nil
}

// loadSources reads the grants, locks and agents of orgID into snap from
// the sources that are set.
func (s *Service) loadSources(ctx context.Context, orgID uuid.UUID, snap *Snapshot) (err error) {
	if s.deps.Grants != nil {
		if snap.Grants, err = s.deps.Grants.PolicyGrants(ctx, orgID); err != nil {
			return err
		}
	}
	if s.deps.Locks != nil {
		if snap.Locks, err = s.deps.Locks.PolicyLocks(ctx, orgID); err != nil {
			return err
		}
	}
	if s.deps.Agents != nil {
		snap.Agents, snap.BlockedRuns, err = s.deps.Agents.PolicyAgents(ctx, orgID)
	}
	return err
}

// readOrg reads the resource, exec, path and terminal rules of snap.OrgID into snap and
// checks a valid ip against the blocklist.
func readOrg(ctx context.Context, st Store, ip netip.Addr, snap *Snapshot) (blocked bool, err error) {
	if snap.Rules, err = listRules(ctx, st, snap.OrgID, nil); err != nil {
		return false, err
	}
	if err = errors.Join(loadExec(ctx, st, snap), loadPaths(ctx, st, snap)); err != nil {
		return false, err
	}
	if ip.IsValid() {
		return st.IPBlocked(ctx, ip.Unmap())
	}
	return false, nil
}

// loadExec reads the command rules (selectors expanded over snap.Resources,
// by id), the class rules (by pattern) and the enabled custom policies.
func loadExec(ctx context.Context, st Store, snap *Snapshot) error {
	rules, err := st.ListCommandRules(ctx, snap.OrgID)
	if err != nil {
		return err
	}
	if snap.CommandRules, err = snapshotCommandRules(rules, snap.Resources); err != nil {
		return err
	}
	classes, err := st.ListClassRules(ctx, snap.OrgID)
	if err != nil {
		return err
	}
	for _, c := range classes {
		snap.ClassRules = append(snap.ClassRules, SnapshotClassRule{Pattern: c.Pattern, Classes: c.Classes})
	}
	custom, err := st.ListCustomPolicies(ctx, snap.OrgID)
	for _, c := range custom {
		if c.Enabled {
			snap.CustomPolicies = append(snap.CustomPolicies, SnapshotCustomPolicy{ID: c.ID, Cedar: c.Cedar})
		}
	}
	return err
}

// check is a request on a resource of a loaded snapshot before its action is set.
type check struct {
	snap Snapshot
	res  SnapshotResource
	typ  connector.Type
	ps   *PolicySet // nil: the snapshot failed to compile, Authorize denies
	req  Request
}

// prepare loads and compiles the snapshot of orgID for a request of sub on
// resourceID from ip at the time at (zero: now) with the first action of the
// resource type.
func (s *Service) prepare(ctx context.Context, orgID uuid.UUID, sub subject, resourceID uuid.UUID, ip netip.Addr, at time.Time) (check, error) {
	snap, blocked, err := s.load(ctx, orgID, ip)
	if err != nil {
		return check{}, err
	}
	user, err := sub.sponsor(snap)
	if err != nil {
		return check{}, err
	}
	c, err := locate(snap, resourceID)
	if err != nil {
		return check{}, err
	}
	teams, err := s.deps.Teams.UserTeams(ctx, orgID, user)
	if err != nil {
		return check{}, err
	}
	if c.ps, _, err = Compile(snap); err != nil {
		s.deps.Logger.ErrorContext(ctx, "policy: compile snapshot", "org_id", orgID, "err", err)
	}
	c.req = Request{UserID: user, AgentID: sub.agent, TeamIDs: teams, ResourceID: resourceID, Action: c.typ.Actions[0],
		IP: ip, Country: s.Country(ip), Time: at, IPBlocked: blocked}
	return c, nil
}

// locate is the check of resourceID in snap, its request not set yet.
func locate(snap Snapshot, resourceID uuid.UUID) (check, error) {
	i := slices.IndexFunc(snap.Resources, func(r SnapshotResource) bool { return r.ID == resourceID })
	if i < 0 {
		return check{}, fmt.Errorf("resource not found: %w", apperr.ErrNotFound)
	}
	c := check{snap: snap, res: snap.Resources[i]}
	var ok bool
	if c.typ, ok = connector.Lookup(c.res.Type); !ok {
		return check{}, fmt.Errorf("resource type %q is not registered: %w", c.res.Type, apperr.ErrFailedPrecondition)
	}
	return c, nil
}

// CheckAccess decides whether userID may use resourceID from ip at the time at
// (zero: now) on the target port (0: terminal or exec) with the first action of
// the resource type. A snapshot that fails to compile denies.
func (s *Service) CheckAccess(ctx context.Context, orgID, userID, resourceID uuid.UUID, ip netip.Addr, at time.Time, port uint16) (Decision, error) {
	c, err := s.prepare(ctx, orgID, subject{user: userID}, resourceID, ip, at)
	if err != nil {
		return Decision{}, err
	}
	c.req.Port = port
	return c.ps.Authorize(c.req), nil
}
