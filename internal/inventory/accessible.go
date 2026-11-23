package inventory

import (
	"context"
	"net/netip"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"

	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
	"github.com/werbot/werbot/internal/connector"
	"github.com/werbot/werbot/internal/platform/apperr"
	"github.com/werbot/werbot/internal/platform/clientip"
	"github.com/werbot/werbot/internal/platform/principal"
)

// Accessible decides which resources a user or an agent may connect to
// (internal/app adapts *policy.Service).
type Accessible interface {
	AccessibleResources(ctx context.Context, orgID, userID uuid.UUID, ip netip.Addr, at time.Time) ([]uuid.UUID, error)
	// AgentResources decides for an agent with its sponsor (both must allow):
	// the resources it may connect to for an exec, keyed by id.
	AgentResources(ctx context.Context, orgID, agentID uuid.UUID, ip netip.Addr, at time.Time) (map[uuid.UUID]AgentAccess, error)
}

// AgentAccess are the command classes an agent may run on a resource: alone
// (Allowed) or after an approval (Approval).
type AgentAccess struct {
	Allowed, Approval []string
}

// Requestable reports which resources an approval policy lets p request; *access.Service satisfies it.
type Requestable interface {
	Requestable(ctx context.Context, p principal.Principal, resourceIDs []uuid.UUID) (map[uuid.UUID]bool, error)
}

// ListAccessibleResources lists the resources the caller may connect to or
// request access to, by name, each with the address and host key of its pool;
// any member may call it. For a run token the agent and its sponsor must both
// allow, and each resource carries the classes the agent may run.
func (h *Handler) ListAccessibleResources(ctx context.Context, _ *connect.Request[werbotv1.ListAccessibleResourcesRequest]) (*connect.Response[werbotv1.ListAccessibleResourcesResponse], error) {
	p, err := principal.Require(ctx)
	if err != nil {
		return nil, apperr.ToConnect(err)
	}
	reach, err := h.subjectResources(ctx, p)
	if err != nil {
		return nil, apperr.ToConnect(err)
	}
	all, err := h.svc.List(ctx, p.OrgID, nil)
	if err != nil {
		return nil, apperr.ToConnect(err)
	}
	orgIDs := make([]uuid.UUID, len(all))
	for i, r := range all {
		orgIDs[i] = r.ID
	}
	requestable, err := h.req.Requestable(ctx, p, orgIDs)
	if err != nil {
		return nil, apperr.ToConnect(err)
	}
	pools, err := h.gws.PoolEndpoints(ctx, p.OrgID)
	if err != nil {
		return nil, apperr.ToConnect(err)
	}
	res := &werbotv1.ListAccessibleResourcesResponse{}
	for _, r := range all {
		classes, allowed := reach[r.ID]
		if !allowed && !requestable[r.ID] {
			continue
		}
		out := &werbotv1.AccessibleResource{Id: r.ID.String(), Name: r.Name, Type: r.Type, Labels: r.Labels,
			Allowed: allowed, Requestable: requestable[r.ID], AllowedClasses: classes.Allowed, ApprovalClasses: classes.Approval}
		if t, ok := connector.Lookup(r.Type); ok {
			for _, k := range t.Kinds {
				out.Kinds = append(out.Kinds, string(k))
			}
		}
		if r.GatewayID != nil {
			out.SshAddress, out.HostKey = pools[*r.GatewayID].SSHAddress, pools[*r.GatewayID].HostKey
		}
		res.Resources = append(res.Resources, out)
	}
	return connect.NewResponse(res), nil
}

// subjectResources are the resources p may connect to: an agent's with its
// classes, a person's without.
func (h *Handler) subjectResources(ctx context.Context, p principal.Principal) (map[uuid.UUID]AgentAccess, error) {
	ip, now := clientip.From(ctx), time.Now()
	if p.AgentID != nil {
		return h.acc.AgentResources(ctx, p.OrgID, *p.AgentID, ip, now)
	}
	ids, err := h.acc.AccessibleResources(ctx, p.OrgID, p.UserID, ip, now)
	out := make(map[uuid.UUID]AgentAccess, len(ids))
	for _, id := range ids {
		out[id] = AgentAccess{}
	}
	return out, err
}
