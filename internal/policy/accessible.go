package policy

import (
	"context"
	"net/netip"
	"time"

	"github.com/google/uuid"

	"github.com/werbot/werbot/internal/connector"
)

// AccessibleResources returns the resources userID may connect to (the first
// action of the type, port 0) from ip at the time at (zero: now), in snapshot
// order. It evaluates one snapshot like CheckAccess: teams, country and the
// blocklist apply. A snapshot that fails to compile allows nothing.
func (s *Service) AccessibleResources(ctx context.Context, orgID, userID uuid.UUID, ip netip.Addr, at time.Time) ([]uuid.UUID, error) {
	snap, blocked, err := s.load(ctx, orgID, ip)
	if err != nil {
		return nil, err
	}
	teams, err := s.deps.Teams.UserTeams(ctx, orgID, userID)
	if err != nil {
		return nil, err
	}
	ps, _, err := Compile(snap)
	if err != nil {
		s.deps.Logger.ErrorContext(ctx, "policy: compile snapshot", "org_id", orgID, "err", err)
		return nil, nil
	}
	country := s.Country(ip)
	var out []uuid.UUID
	for _, r := range snap.Resources {
		typ, ok := connector.Lookup(r.Type)
		if !ok {
			continue
		}
		if ps.Authorize(Request{UserID: userID, TeamIDs: teams, ResourceID: r.ID, Action: typ.Actions[0],
			IP: ip, Country: country, Time: at, IPBlocked: blocked}).Allow {
			out = append(out, r.ID)
		}
	}
	return out, nil
}
