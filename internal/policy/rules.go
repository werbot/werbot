package policy

import (
	"context"
	"fmt"
	"net/netip"
	"slices"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/werbot/werbot/internal/platform/apperr"
	"github.com/werbot/werbot/internal/platform/auditlog"
	"github.com/werbot/werbot/internal/platform/clientip"
	"github.com/werbot/werbot/internal/platform/dbx"
	"github.com/werbot/werbot/internal/platform/principal"
	"github.com/werbot/werbot/internal/policy/dbq"
)

func invalid(format string, a ...any) error {
	return fmt.Errorf("%w: %s", apperr.ErrInvalidArgument, fmt.Sprintf(format, a...))
}

// requireResource returns ErrNotFound unless resourceID is a resource of orgID.
func (s *Service) requireResource(ctx context.Context, orgID, resourceID uuid.UUID) error {
	rs, err := s.deps.Resources.PolicyResources(ctx, orgID)
	if err != nil {
		return err
	}
	if !slices.ContainsFunc(rs, func(r SnapshotResource) bool { return r.ID == resourceID }) {
		return fmt.Errorf("resource not found: %w", apperr.ErrNotFound)
	}
	return nil
}

// GetRules returns the rules of a resource of orgID.
func (s *Service) GetRules(ctx context.Context, orgID, resourceID uuid.UUID) (ResourceRules, error) {
	if err := s.requireResource(ctx, orgID, resourceID); err != nil {
		return ResourceRules{}, err
	}
	var rules []ResourceRules
	err := dbx.InOrg(ctx, s.deps.Pool, orgID, actor(ctx), func(tx pgx.Tx) (err error) {
		rules, err = listRules(ctx, newStore(tx), orgID, &resourceID)
		return err
	})
	if err != nil || len(rules) == 0 {
		return ResourceRules{ResourceID: resourceID}, err
	}
	return rules[0], nil
}

// SetRules replaces the three rule sets of r.ResourceID in one transaction.
func (s *Service) SetRules(ctx context.Context, orgID uuid.UUID, r ResourceRules) error {
	p, err := principal.Require(ctx)
	if err != nil {
		return err
	}
	if r, err = normalize(r); err != nil {
		return err
	}
	if err := s.requireResource(ctx, orgID, r.ResourceID); err != nil {
		return err
	}
	err = dbx.InOrg(ctx, s.deps.Pool, orgID, p.UserID, func(tx pgx.Tx) error {
		if err := replaceRules(ctx, newStore(tx), orgID, r); err != nil {
			return err
		}
		return auditlog.Write(ctx, tx, auditlog.Event{OrgID: &orgID, ActorUserID: &p.UserID, ActorKind: "user",
			Action: "policy.rules.set", TargetType: "resource", TargetID: r.ResourceID.String(), IP: clientip.From(ctx),
			Data: map[string]any{"networks": r.Networks, "countries": r.Countries, "schedule": r.Schedule}})
	})
	if isForeignKeyViolation(err) { // the resource left the organization meanwhile
		return fmt.Errorf("resource not found: %w", apperr.ErrNotFound)
	}
	return err
}

// normalize masks the CIDRs of r and validates it as Compile does, plus what
// Compile tolerates but a stored rule set must not have: IPv4-mapped CIDRs
// (they never match the unmapped client address) and duplicates.
func normalize(r ResourceRules) (ResourceRules, error) {
	r.Networks = slices.Clone(r.Networks)
	for i, n := range r.Networks {
		if n.CIDR.Addr().Is4In6() {
			return r, invalid("cidr %s: use the IPv4 form", n.CIDR)
		}
		r.Networks[i].CIDR = n.CIDR.Masked()
	}
	if err := writeRules(&strings.Builder{}, r); err != nil {
		return r, fmt.Errorf("%w: %w", apperr.ErrInvalidArgument, err)
	}
	switch {
	case hasDup(r.Networks, func(n NetworkRule) netip.Prefix { return n.CIDR }):
		return r, invalid("duplicate cidr")
	case hasDup(r.Countries, func(c CountryRule) string { return c.Country }):
		return r, invalid("duplicate country")
	case hasDup(r.Schedule, func(s Slot) Slot { return s }):
		return r, invalid("duplicate schedule slot")
	}
	return r, nil
}

func hasDup[T any, K comparable](items []T, key func(T) K) bool {
	seen := make(map[K]bool, len(items))
	for _, it := range items {
		k := key(it)
		if seen[k] {
			return true
		}
		seen[k] = true
	}
	return false
}

func replaceRules(ctx context.Context, st Store, orgID uuid.UUID, r ResourceRules) error {
	id := r.ResourceID
	if err := st.DeleteNetworkRules(ctx, dbq.DeleteNetworkRulesParams{OrgID: orgID, ResourceID: id}); err != nil {
		return err
	}
	if err := st.DeleteCountryRules(ctx, dbq.DeleteCountryRulesParams{OrgID: orgID, ResourceID: id}); err != nil {
		return err
	}
	if err := st.DeleteSchedule(ctx, dbq.DeleteScheduleParams{OrgID: orgID, ResourceID: id}); err != nil {
		return err
	}
	for _, n := range r.Networks {
		if err := st.InsertNetworkRule(ctx, dbq.InsertNetworkRuleParams{OrgID: orgID, ResourceID: id, Cidr: n.CIDR, Effect: string(n.Effect)}); err != nil {
			return err
		}
	}
	for _, c := range r.Countries {
		if err := st.InsertCountryRule(ctx, dbq.InsertCountryRuleParams{OrgID: orgID, ResourceID: id, Country: c.Country, Effect: string(c.Effect)}); err != nil {
			return err
		}
	}
	for _, sl := range r.Schedule {
		slot := dbq.InsertSlotParams{OrgID: orgID, ResourceID: id, Weekday: int16(sl.Weekday), Hour: int16(sl.Hour)} //nolint:gosec // normalize checked 0-6 x 0-23
		if err := st.InsertSlot(ctx, slot); err != nil {
			return err
		}
	}
	return nil
}

// listRules returns the rules of orgID by resource, narrowed to resourceID when set.
func listRules(ctx context.Context, st Store, orgID uuid.UUID, resourceID *uuid.UUID) ([]ResourceRules, error) {
	byID := map[uuid.UUID]*ResourceRules{}
	of := func(id uuid.UUID) *ResourceRules {
		if byID[id] == nil {
			byID[id] = &ResourceRules{ResourceID: id}
		}
		return byID[id]
	}
	nets, err := st.ListNetworkRules(ctx, dbq.ListNetworkRulesParams{OrgID: orgID, ResourceID: resourceID})
	if err != nil {
		return nil, err
	}
	for _, n := range nets {
		r := of(n.ResourceID)
		r.Networks = append(r.Networks, NetworkRule{CIDR: n.Cidr, Effect: Effect(n.Effect)})
	}
	countries, err := st.ListCountryRules(ctx, dbq.ListCountryRulesParams{OrgID: orgID, ResourceID: resourceID})
	if err != nil {
		return nil, err
	}
	for _, c := range countries {
		r := of(c.ResourceID)
		r.Countries = append(r.Countries, CountryRule{Country: c.Country, Effect: Effect(c.Effect)})
	}
	slots, err := st.ListSchedule(ctx, dbq.ListScheduleParams{OrgID: orgID, ResourceID: resourceID})
	if err != nil {
		return nil, err
	}
	for _, sl := range slots {
		r := of(sl.ResourceID)
		r.Schedule = append(r.Schedule, Slot{Weekday: int(sl.Weekday), Hour: int(sl.Hour)})
	}
	out := make([]ResourceRules, 0, len(byID))
	for _, r := range byID {
		out = append(out, *r)
	}
	slices.SortFunc(out, func(a, b ResourceRules) int { return compareIDs(a.ResourceID, b.ResourceID) })
	return out, nil
}
