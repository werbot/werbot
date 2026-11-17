package access

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

// Handler serves AccessService.
type Handler struct {
	werbotv1connect.UnimplementedAccessServiceHandler
	svc *Service
}

// NewHandler returns the Connect handler for AccessService.
func NewHandler(s *Service) *Handler { return &Handler{svc: s} }

// admin authorizes owner/admin and returns the organization.
func admin(ctx context.Context) (uuid.UUID, error) {
	p, err := principal.RequireRole(ctx, "owner", "admin")
	return p.OrgID, apperr.ToConnect(err)
}

// ListBindings lists bindings, optionally narrowed to one resource.
func (h *Handler) ListBindings(ctx context.Context, req *connect.Request[werbotv1.ListBindingsRequest]) (*connect.Response[werbotv1.ListBindingsResponse], error) {
	org, err := admin(ctx)
	if err != nil {
		return nil, err
	}
	var rid *uuid.UUID
	if req.Msg.ResourceId != nil {
		id, err := parseID(req.Msg.GetResourceId())
		if err != nil {
			return nil, apperr.ToConnect(err)
		}
		rid = &id
	}
	bs, err := h.svc.List(ctx, org, rid)
	if err != nil {
		return nil, apperr.ToConnect(err)
	}
	res := &werbotv1.ListBindingsResponse{}
	for _, b := range bs {
		res.Bindings = append(res.Bindings, bindingToProto(b))
	}
	return connect.NewResponse(res), nil
}

// CreateBinding grants a user or team access to a resource or a label selector.
func (h *Handler) CreateBinding(ctx context.Context, req *connect.Request[werbotv1.CreateBindingRequest]) (*connect.Response[werbotv1.CreateBindingResponse], error) {
	org, err := admin(ctx)
	if err != nil {
		return nil, err
	}
	m := req.Msg
	subject, err := parseID(m.GetSubjectId())
	if err != nil {
		return nil, apperr.ToConnect(err)
	}
	ports, err := connector.PortsFromProto(m.GetPorts())
	if err != nil {
		return nil, apperr.ToConnect(invalid("%v", err))
	}
	in := CreateInput{SubjectType: m.GetSubjectType(), SubjectID: subject, LabelSelector: m.GetLabelSelector(), Ports: ports}
	if m.GetResourceId() != "" {
		id, err := parseID(m.GetResourceId())
		if err != nil {
			return nil, apperr.ToConnect(err)
		}
		in.ResourceID = &id
	}
	b, err := h.svc.Create(ctx, org, in)
	if err != nil {
		return nil, apperr.ToConnect(err)
	}
	return connect.NewResponse(&werbotv1.CreateBindingResponse{Binding: bindingToProto(b)}), nil
}

// DeleteBinding removes a binding.
func (h *Handler) DeleteBinding(ctx context.Context, req *connect.Request[werbotv1.DeleteBindingRequest]) (*connect.Response[werbotv1.DeleteBindingResponse], error) {
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
	return connect.NewResponse(&werbotv1.DeleteBindingResponse{}), nil
}
