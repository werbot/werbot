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
	Logger    *slog.Logger
}

// Service owns resource rules and decides access requests.
type Service struct {
	deps Deps
	geo  atomic.Pointer[GeoIP] // nil until ReloadGeoIP finds a database
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
// the current database; a broken one keeps it too and returns the error.
func (s *Service) ReloadGeoIP(ctx context.Context) error {
	var data []byte
	err := dbx.System(ctx, s.deps.Pool, dbx.ReasonMaintenance, func(tx pgx.Tx) (err error) {
		data, err = newStore(tx).GetGeoIPData(ctx)
		return err
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("policy: reload geoip: %w", err)
	}
	g, err := OpenGeoIP(data)
	if err != nil {
		return fmt.Errorf("policy: reload geoip: %w", err)
	}
	s.geo.Store(&g)
	return nil
}

// Country resolves ip with the current GeoIP database; "" without one.
func (s *Service) Country(ip netip.Addr) string {
	if g := s.geo.Load(); g != nil {
		return (*g).Country(ip)
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
	var rules []ResourceRules
	err = dbx.InOrg(ctx, s.deps.Pool, orgID, actor(ctx), func(tx pgx.Tx) error {
		st := newStore(tx)
		if rules, err = listRules(ctx, st, orgID, nil); err != nil {
			return err
		}
		if ip.IsValid() {
			blocked, err = st.IPBlocked(ctx, ip.Unmap())
		}
		return err
	})
	if err != nil {
		return Snapshot{}, false, err
	}
	return Snapshot{OrgID: orgID, Resources: resources, Bindings: ExpandBindings(resources, bindings), Rules: rules}, blocked, nil
}

// CheckAccess decides whether userID may use resourceID from ip at the time at
// (zero: now) with the first action of the resource type. A snapshot that
// fails to compile denies.
func (s *Service) CheckAccess(ctx context.Context, orgID, userID, resourceID uuid.UUID, ip netip.Addr, at time.Time) (Decision, error) {
	snap, blocked, err := s.load(ctx, orgID, ip)
	if err != nil {
		return Decision{}, err
	}
	i := slices.IndexFunc(snap.Resources, func(r SnapshotResource) bool { return r.ID == resourceID })
	if i < 0 {
		return Decision{}, fmt.Errorf("resource not found: %w", apperr.ErrNotFound)
	}
	typ, ok := connector.Lookup(snap.Resources[i].Type)
	if !ok {
		return Decision{}, fmt.Errorf("resource type %q is not registered: %w", snap.Resources[i].Type, apperr.ErrFailedPrecondition)
	}
	teams, err := s.deps.Teams.UserTeams(ctx, orgID, userID)
	if err != nil {
		return Decision{}, err
	}
	ps, _, err := Compile(snap)
	if err != nil {
		s.deps.Logger.ErrorContext(ctx, "policy: compile snapshot", "org_id", orgID, "err", err)
		return Decision{Reasons: []string{"error:compile"}}, nil
	}
	return ps.Authorize(Request{UserID: userID, TeamIDs: teams, ResourceID: resourceID, Action: typ.Actions[0],
		IP: ip, Country: s.Country(ip), Time: at, IPBlocked: blocked}), nil
}
