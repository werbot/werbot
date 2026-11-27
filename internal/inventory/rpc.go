package inventory

import (
	"context"

	"connectrpc.com/connect"
	"github.com/google/uuid"

	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
	"github.com/werbot/werbot/gen/werbot/v1/werbotv1connect"
	"github.com/werbot/werbot/internal/connector"
	"github.com/werbot/werbot/internal/platform/apperr"
	"github.com/werbot/werbot/internal/platform/principal"
)

// Handler serves ResourceService.
type Handler struct {
	werbotv1connect.UnimplementedResourceServiceHandler
	svc   *Service
	creds Credentials
	acc   Accessible
	gws   *Gateways
	req   Requestable
}

// NewHandler returns the Connect handler for ResourceService.
func NewHandler(s *Service, creds Credentials, acc Accessible, gws *Gateways, req Requestable) *Handler {
	return &Handler{svc: s, creds: creds, acc: acc, gws: gws, req: req}
}

// admin authorizes owner/admin and returns the organization.
func admin(ctx context.Context) (uuid.UUID, error) {
	p, err := principal.RequireRole(ctx, "owner", "admin")
	return p.OrgID, apperr.ToConnect(err)
}

// resources converts rs with the credential kinds of both slots, one call per slot.
func (h *Handler) resources(ctx context.Context, org uuid.UUID, rs ...Resource) ([]*werbotv1.Resource, error) {
	ids := make([]uuid.UUID, len(rs))
	for i, r := range rs {
		ids[i] = r.ID
	}
	kinds, err := h.creds.Kinds(ctx, org, ids, slotPrimary)
	if err != nil {
		return nil, apperr.ToConnect(err)
	}
	agentKinds, err := h.creds.Kinds(ctx, org, ids, slotAgentReadonly)
	if err != nil {
		return nil, apperr.ToConnect(err)
	}
	out := make([]*werbotv1.Resource, len(rs))
	for i, r := range rs {
		if out[i], err = resourceToProto(r, kinds[r.ID], agentKinds[r.ID]); err != nil {
			return nil, apperr.ToConnect(err)
		}
	}
	return out, nil
}

// ListResourceTypes lists the registered resource types, then the presets of the organization; any member may read them.
func (h *Handler) ListResourceTypes(ctx context.Context, _ *connect.Request[werbotv1.ListResourceTypesRequest]) (*connect.Response[werbotv1.ListResourceTypesResponse], error) {
	p, err := principal.Require(ctx)
	if err != nil {
		return nil, apperr.ToConnect(err)
	}
	ps, err := h.svc.ListPresets(ctx, p.OrgID)
	if err != nil {
		return nil, apperr.ToConnect(err)
	}
	res := &werbotv1.ListResourceTypesResponse{}
	for _, t := range connector.Types() {
		res.Types = append(res.Types, typeToProto(t))
	}
	for _, pr := range ps {
		if base, ok := connector.Lookup(pr.BaseType); ok {
			res.Types = append(res.Types, presetTypeToProto(pr, base))
		}
	}
	return connect.NewResponse(res), nil
}

// ListResources lists resources carrying every label of the filter.
func (h *Handler) ListResources(ctx context.Context, req *connect.Request[werbotv1.ListResourcesRequest]) (*connect.Response[werbotv1.ListResourcesResponse], error) {
	org, err := admin(ctx)
	if err != nil {
		return nil, err
	}
	rs, err := h.svc.List(ctx, org, req.Msg.GetLabelFilter())
	if err != nil {
		return nil, apperr.ToConnect(err)
	}
	out, err := h.resources(ctx, org, rs...)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&werbotv1.ListResourcesResponse{Resources: out}), nil
}

// GetResource returns one resource.
func (h *Handler) GetResource(ctx context.Context, req *connect.Request[werbotv1.GetResourceRequest]) (*connect.Response[werbotv1.GetResourceResponse], error) {
	org, err := admin(ctx)
	if err != nil {
		return nil, err
	}
	id, err := parseID(req.Msg.GetId())
	if err != nil {
		return nil, apperr.ToConnect(err)
	}
	r, err := h.svc.Get(ctx, org, id)
	if err != nil {
		return nil, apperr.ToConnect(err)
	}
	out, err := h.resources(ctx, org, r)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&werbotv1.GetResourceResponse{Resource: out[0]}), nil
}

// CreateResource adds a resource; the config comes as Any or as JSON.
func (h *Handler) CreateResource(ctx context.Context, req *connect.Request[werbotv1.CreateResourceRequest]) (*connect.Response[werbotv1.CreateResourceResponse], error) {
	org, err := admin(ctx)
	if err != nil {
		return nil, err
	}
	m := req.Msg
	t, ok := connector.Lookup(m.GetType())
	if !ok {
		return nil, apperr.ToConnect(invalid("unknown resource type %q", m.GetType()))
	}
	gw, err := parseGatewayID(m.GetGatewayId())
	if err != nil {
		return nil, apperr.ToConnect(err)
	}
	presetID, err := h.presetType(ctx, org, m)
	if err != nil {
		return nil, apperr.ToConnect(err)
	}
	cfg, err := parseConfig(t, m.GetConfig(), m.GetConfigJson())
	if err != nil {
		return nil, apperr.ToConnect(err)
	}
	idle, maxDur := timeouts(m.GetIdleTimeout(), m.GetMaxDuration())
	r, err := h.svc.Create(ctx, org, CreateInput{Name: m.GetName(), Type: t.Name, GatewayID: gw, PresetID: presetID, Config: cfg, Labels: m.GetLabels(),
		IdleTimeout: idle, MaxDuration: maxDur})
	if err != nil {
		return nil, apperr.ToConnect(err)
	}
	out, err := h.resources(ctx, org, r)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&werbotv1.CreateResourceResponse{Resource: out[0]}), nil
}

// UpdateResource replaces name, gateway, timeouts and labels; no config keeps the current one.
func (h *Handler) UpdateResource(ctx context.Context, req *connect.Request[werbotv1.UpdateResourceRequest]) (*connect.Response[werbotv1.UpdateResourceResponse], error) {
	org, err := admin(ctx)
	if err != nil {
		return nil, err
	}
	m := req.Msg
	id, err := parseID(m.GetId())
	if err != nil {
		return nil, apperr.ToConnect(err)
	}
	gw, err := parseGatewayID(m.GetGatewayId())
	if err != nil {
		return nil, apperr.ToConnect(err)
	}
	idle, maxDur := timeouts(m.GetIdleTimeout(), m.GetMaxDuration())
	in := UpdateInput{ID: id, Name: m.GetName(), GatewayID: gw, Labels: m.GetLabels(), IdleTimeout: idle, MaxDuration: maxDur}
	if m.GetConfig() != nil || m.GetConfigJson() != "" {
		cur, err := h.svc.Get(ctx, org, id)
		if err != nil {
			return nil, apperr.ToConnect(err)
		}
		t, ok := connector.Lookup(cur.Type)
		if !ok {
			return nil, apperr.ToConnect(invalid("resource type %q is no longer registered", cur.Type))
		}
		if in.Config, err = parseConfig(t, m.GetConfig(), m.GetConfigJson()); err != nil {
			return nil, apperr.ToConnect(err)
		}
	}
	r, err := h.svc.Update(ctx, org, in)
	if err != nil {
		return nil, apperr.ToConnect(err)
	}
	out, err := h.resources(ctx, org, r)
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&werbotv1.UpdateResourceResponse{Resource: out[0]}), nil
}

// DeleteResource removes a resource.
func (h *Handler) DeleteResource(ctx context.Context, req *connect.Request[werbotv1.DeleteResourceRequest]) (*connect.Response[werbotv1.DeleteResourceResponse], error) {
	org, err := admin(ctx)
	if err != nil {
		return nil, err
	}
	id, err := parseID(req.Msg.GetId())
	if err != nil {
		return nil, apperr.ToConnect(err)
	}
	if err := h.svc.Delete(ctx, org, id); err != nil {
		return nil, apperr.ToConnect(err)
	}
	return connect.NewResponse(&werbotv1.DeleteResourceResponse{}), nil
}

// ClearHostKey forgets the pinned host key.
func (h *Handler) ClearHostKey(ctx context.Context, req *connect.Request[werbotv1.ClearHostKeyRequest]) (*connect.Response[werbotv1.ClearHostKeyResponse], error) {
	org, err := admin(ctx)
	if err != nil {
		return nil, err
	}
	id, err := parseID(req.Msg.GetResourceId())
	if err != nil {
		return nil, apperr.ToConnect(err)
	}
	if err := h.svc.ClearHostKey(ctx, org, id); err != nil {
		return nil, apperr.ToConnect(err)
	}
	return connect.NewResponse(&werbotv1.ClearHostKeyResponse{}), nil
}
