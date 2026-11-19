package inventory

import (
	"context"
	"net/netip"
	"slices"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"

	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
	"github.com/werbot/werbot/internal/connector"
	"github.com/werbot/werbot/internal/platform/apperr"
	"github.com/werbot/werbot/internal/platform/clientip"
	"github.com/werbot/werbot/internal/platform/principal"
)

// Accessible decides which resources a user may connect to; *policy.Service satisfies it.
type Accessible interface {
	AccessibleResources(ctx context.Context, orgID, userID uuid.UUID, ip netip.Addr, at time.Time) ([]uuid.UUID, error)
}

// ListAccessibleResources lists the resources the caller may connect to, by name,
// each with the address and host key of its pool; any member may call it.
func (h *Handler) ListAccessibleResources(ctx context.Context, _ *connect.Request[werbotv1.ListAccessibleResourcesRequest]) (*connect.Response[werbotv1.ListAccessibleResourcesResponse], error) {
	p, err := principal.Require(ctx)
	if err != nil {
		return nil, apperr.ToConnect(err)
	}
	ids, err := h.acc.AccessibleResources(ctx, p.OrgID, p.UserID, clientip.From(ctx), time.Now())
	if err != nil {
		return nil, apperr.ToConnect(err)
	}
	all, err := h.svc.List(ctx, p.OrgID, nil)
	if err != nil {
		return nil, apperr.ToConnect(err)
	}
	pools, err := h.gws.PoolEndpoints(ctx, p.OrgID)
	if err != nil {
		return nil, apperr.ToConnect(err)
	}
	res := &werbotv1.ListAccessibleResourcesResponse{}
	for _, r := range all {
		if !slices.Contains(ids, r.ID) {
			continue
		}
		out := &werbotv1.AccessibleResource{Id: r.ID.String(), Name: r.Name, Type: r.Type, Labels: r.Labels}
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
