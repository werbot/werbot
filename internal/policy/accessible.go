package policy

import (
	"context"
	"net/netip"
	"slices"
	"time"

	"github.com/google/uuid"

	"github.com/werbot/werbot/internal/connector"
	"github.com/werbot/werbot/internal/platform/shellclass"
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

// ClassAccess are the command classes an agent may run on a resource: alone
// (Allowed) or after an approval (Approval), in class order.
type ClassAccess struct {
	Allowed, Approval []string
}

// probeClasses are the classes AgentResources probes one by one.
var probeClasses = []shellclass.Class{shellclass.Destructive, shellclass.Egress, shellclass.Opaque,
	shellclass.Privilege, shellclass.Read, shellclass.Secrets, shellclass.Write}

// AgentResources returns the resources agentID may connect to for an exec
// (the first action of the type, kind "exec") from ip at the time at (zero:
// now) with the classes it may run there. The decision merges the sponsor
// and agent passes, so a resource the sponsor cannot reach is never listed.
// An agent missing from the snapshot is ErrNotFound; a snapshot that fails
// to compile allows nothing.
func (s *Service) AgentResources(ctx context.Context, orgID, agentID uuid.UUID, ip netip.Addr, at time.Time) (map[uuid.UUID]ClassAccess, error) {
	snap, blocked, err := s.load(ctx, orgID, ip)
	if err != nil {
		return nil, err
	}
	sponsor, err := agentSponsor(snap, agentID)
	if err != nil {
		return nil, err
	}
	teams, err := s.deps.Teams.UserTeams(ctx, orgID, sponsor)
	if err != nil {
		return nil, err
	}
	out := map[uuid.UUID]ClassAccess{}
	ps, _, err := Compile(snap)
	if err != nil {
		s.deps.Logger.ErrorContext(ctx, "policy: compile snapshot", "org_id", orgID, "err", err)
		return out, nil
	}
	r := Request{UserID: sponsor, TeamIDs: teams, AgentID: &agentID, Kind: string(connector.KindExec),
		IP: ip, Country: s.Country(ip), Time: at, IPBlocked: blocked}
	for _, res := range snap.Resources {
		if typ, ok := connector.Lookup(res.Type); ok {
			r.ResourceID, r.Action = res.ID, typ.Actions[0]
			if ps.Authorize(r).Allow {
				out[res.ID] = classAccess(ps, r, typ)
			}
		}
	}
	return out, nil
}

// classAccess probes the exec action of typ with r for each class alone.
func classAccess(ps *PolicySet, r Request, typ connector.Type) ClassAccess {
	var out ClassAccess
	exec, ok := typ.ExecAction()
	if !ok {
		return out
	}
	r.Action = exec
	for _, c := range probeClasses {
		r.Classes = []string{string(c)}
		d := ps.Authorize(r)
		switch {
		case d.Allow:
			out.Allowed = append(out.Allowed, string(c))
		case slices.Equal(d.Obligations, []string{ObligationApproval}):
			out.Approval = append(out.Approval, string(c))
		}
	}
	return out
}
