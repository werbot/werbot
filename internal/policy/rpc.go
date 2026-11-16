package policy

import (
	"context"
	"net/netip"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"

	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
	"github.com/werbot/werbot/gen/werbot/v1/werbotv1connect"
	"github.com/werbot/werbot/internal/platform/apperr"
	"github.com/werbot/werbot/internal/platform/principal"
)

// Handler serves PolicyService.
type Handler struct {
	werbotv1connect.UnimplementedPolicyServiceHandler
	svc *Service
}

// NewHandler returns the Connect handler for PolicyService.
func NewHandler(s *Service) *Handler { return &Handler{svc: s} }

// admin authorizes owner/admin and returns the organization.
func admin(ctx context.Context) (uuid.UUID, error) {
	p, err := principal.RequireRole(ctx, "owner", "admin")
	return p.OrgID, apperr.ToConnect(err)
}

func parseID(s string) (uuid.UUID, error) {
	id, err := uuid.Parse(s)
	if err != nil {
		return uuid.Nil, invalid("invalid id %q", s)
	}
	return id, nil
}

// GetResourceRules returns the network, country and schedule rules of a resource.
func (h *Handler) GetResourceRules(ctx context.Context, req *connect.Request[werbotv1.GetResourceRulesRequest]) (*connect.Response[werbotv1.GetResourceRulesResponse], error) {
	org, err := admin(ctx)
	if err != nil {
		return nil, err
	}
	id, err := parseID(req.Msg.GetResourceId())
	if err != nil {
		return nil, apperr.ToConnect(err)
	}
	r, err := h.svc.GetRules(ctx, org, id)
	if err != nil {
		return nil, apperr.ToConnect(err)
	}
	res := &werbotv1.GetResourceRulesResponse{}
	for _, n := range r.Networks {
		res.Networks = append(res.Networks, &werbotv1.NetworkRule{Cidr: n.CIDR.String(), Effect: string(n.Effect)})
	}
	for _, c := range r.Countries {
		res.Countries = append(res.Countries, &werbotv1.CountryRule{Country: c.Country, Effect: string(c.Effect)})
	}
	for _, s := range r.Schedule {
		res.Schedule = append(res.Schedule, &werbotv1.ScheduleSlot{Weekday: int32(s.Weekday), Hour: int32(s.Hour)}) //nolint:gosec // stored slots are 0-6 x 0-23
	}
	return connect.NewResponse(res), nil
}

// SetResourceRules replaces the rules of a resource.
func (h *Handler) SetResourceRules(ctx context.Context, req *connect.Request[werbotv1.SetResourceRulesRequest]) (*connect.Response[werbotv1.SetResourceRulesResponse], error) {
	org, err := admin(ctx)
	if err != nil {
		return nil, err
	}
	m := req.Msg
	id, err := parseID(m.GetResourceId())
	if err != nil {
		return nil, apperr.ToConnect(err)
	}
	r := ResourceRules{ResourceID: id}
	for _, n := range m.GetNetworks() {
		cidr, err := netip.ParsePrefix(n.GetCidr())
		if err != nil {
			return nil, apperr.ToConnect(invalid("invalid cidr %q", n.GetCidr()))
		}
		r.Networks = append(r.Networks, NetworkRule{CIDR: cidr, Effect: Effect(n.GetEffect())})
	}
	for _, c := range m.GetCountries() {
		r.Countries = append(r.Countries, CountryRule{Country: c.GetCountry(), Effect: Effect(c.GetEffect())})
	}
	for _, s := range m.GetSchedule() {
		r.Schedule = append(r.Schedule, Slot{Weekday: int(s.GetWeekday()), Hour: int(s.GetHour())})
	}
	if err := h.svc.SetRules(ctx, org, r); err != nil {
		return nil, apperr.ToConnect(err)
	}
	return connect.NewResponse(&werbotv1.SetResourceRulesResponse{}), nil
}

// CheckAccess probes the policy for one user, resource, address and time.
func (h *Handler) CheckAccess(ctx context.Context, req *connect.Request[werbotv1.CheckAccessRequest]) (*connect.Response[werbotv1.CheckAccessResponse], error) {
	org, err := admin(ctx)
	if err != nil {
		return nil, err
	}
	m := req.Msg
	user, err := parseID(m.GetUserId())
	if err != nil {
		return nil, apperr.ToConnect(err)
	}
	resource, err := parseID(m.GetResourceId())
	if err != nil {
		return nil, apperr.ToConnect(err)
	}
	ip, err := netip.ParseAddr(m.GetIp())
	if err != nil {
		return nil, apperr.ToConnect(invalid("invalid ip %q", m.GetIp()))
	}
	var at time.Time
	if m.GetTime() != nil {
		at = m.GetTime().AsTime()
	}
	d, err := h.svc.CheckAccess(ctx, org, user, resource, ip, at)
	if err != nil {
		return nil, apperr.ToConnect(err)
	}
	return connect.NewResponse(&werbotv1.CheckAccessResponse{Allow: d.Allow, Reasons: d.Reasons}), nil
}
