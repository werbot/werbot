package audit

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
	"github.com/werbot/werbot/gen/werbot/v1/werbotv1connect"
	"github.com/werbot/werbot/internal/platform/apperr"
	"github.com/werbot/werbot/internal/platform/principal"
)

// Handler serves AuditService.
type Handler struct {
	werbotv1connect.UnimplementedAuditServiceHandler
	svc *Service
}

// NewHandler returns the Connect handler for AuditService.
func NewHandler(s *Service) *Handler { return &Handler{svc: s} }

// ListEvents returns one page of events.
func (h *Handler) ListEvents(ctx context.Context, req *connect.Request[werbotv1.ListEventsRequest]) (*connect.Response[werbotv1.ListEventsResponse], error) {
	p, err := principal.RequireRole(ctx, principal.RoleOwner, principal.RoleAdmin)
	if err != nil {
		return nil, apperr.ToConnect(err)
	}
	m := req.Msg
	f := Filter{Action: m.GetAction(), TargetType: m.GetTargetType(), TargetID: m.GetTargetId(),
		Page: int(m.GetPage()), PageSize: int(m.GetPageSize())}
	if id := m.GetActorUserId(); id != "" {
		u, err := uuid.Parse(id)
		if err != nil {
			return nil, apperr.ToConnect(fmt.Errorf("invalid actor_user_id %q: %w", id, apperr.ErrInvalidArgument))
		}
		f.ActorUserID = &u
	}
	if m.From != nil {
		t := m.From.AsTime()
		f.From = &t
	}
	if m.To != nil {
		t := m.To.AsTime()
		f.To = &t
	}
	events, next, err := h.svc.ListEvents(ctx, p, f)
	if err != nil {
		return nil, apperr.ToConnect(err)
	}
	res := &werbotv1.ListEventsResponse{NextPage: int32(next)} //nolint:gosec // next is a page number
	for _, e := range events {
		pe, err := toProto(e)
		if err != nil {
			return nil, apperr.ToConnect(err)
		}
		res.Events = append(res.Events, pe)
	}
	return connect.NewResponse(res), nil
}

// GetEvent returns one event.
func (h *Handler) GetEvent(ctx context.Context, req *connect.Request[werbotv1.GetEventRequest]) (*connect.Response[werbotv1.GetEventResponse], error) {
	p, err := principal.RequireRole(ctx, principal.RoleOwner, principal.RoleAdmin)
	if err != nil {
		return nil, apperr.ToConnect(err)
	}
	id, err := uuid.Parse(req.Msg.GetId())
	if err != nil {
		return nil, apperr.ToConnect(fmt.Errorf("invalid event id %q: %w", req.Msg.GetId(), apperr.ErrInvalidArgument))
	}
	e, err := h.svc.GetEvent(ctx, p, id)
	if err != nil {
		return nil, apperr.ToConnect(err)
	}
	pe, err := toProto(e)
	if err != nil {
		return nil, apperr.ToConnect(err)
	}
	return connect.NewResponse(&werbotv1.GetEventResponse{Event: pe}), nil
}

func toProto(e Event) (*werbotv1.AuditEvent, error) {
	data, err := structpb.NewStruct(e.Data)
	if err != nil {
		return nil, fmt.Errorf("audit: event data: %w", err)
	}
	out := &werbotv1.AuditEvent{Id: e.ID.String(), ActorLogin: e.ActorLogin, ActorKind: e.ActorKind, Action: e.Action,
		TargetType: e.TargetType, TargetId: e.TargetID, Data: data, CreatedAt: timestamppb.New(e.CreatedAt)}
	if e.ActorUserID != nil {
		out.ActorUserId = e.ActorUserID.String()
	}
	if e.IP.IsValid() {
		out.Ip = e.IP.String()
	}
	return out, nil
}
