package session

import (
	"context"
	"errors"
	"fmt"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/timestamppb"

	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
	"github.com/werbot/werbot/gen/werbot/v1/werbotv1connect"
	"github.com/werbot/werbot/internal/platform/apperr"
	"github.com/werbot/werbot/internal/platform/principal"
)

// SessionHandler serves SessionService.
type SessionHandler struct { //nolint:revive // name fixed by the stage contract
	werbotv1connect.UnimplementedSessionServiceHandler
	svc *Service
	rec *Recordings
}

// NewSessionHandler returns the Connect handler for SessionService.
func NewSessionHandler(svc *Service, rec *Recordings) *SessionHandler {
	return &SessionHandler{svc: svc, rec: rec}
}

// ListSessions returns one page of visible sessions.
func (h *SessionHandler) ListSessions(ctx context.Context, req *connect.Request[werbotv1.ListSessionsRequest]) (*connect.Response[werbotv1.ListSessionsResponse], error) {
	p, err := principal.Require(ctx)
	if err != nil {
		return nil, apperr.ToConnect(err)
	}
	m := req.Msg
	f := ListFilter{Page: int(m.GetPage()), PageSize: int(m.GetPageSize())}
	switch m.GetStatus() {
	case werbotv1.SessionStatus_SESSION_STATUS_ACTIVE:
		f.Status = "active"
	case werbotv1.SessionStatus_SESSION_STATUS_ENDED:
		f.Status = "ended"
	}
	if f.UserID, err = optionalID(m.GetUserId()); err != nil {
		return nil, apperr.ToConnect(err)
	}
	if f.ResourceID, err = optionalID(m.GetResourceId()); err != nil {
		return nil, apperr.ToConnect(err)
	}
	if f.From, err = optionalTime(m.GetFrom()); err != nil {
		return nil, apperr.ToConnect(err)
	}
	if f.To, err = optionalTime(m.GetTo()); err != nil {
		return nil, apperr.ToConnect(err)
	}
	items, next, err := h.svc.ListSessions(ctx, p, f)
	if err != nil {
		return nil, apperr.ToConnect(err)
	}
	res := &werbotv1.ListSessionsResponse{NextPage: int32(next)} //nolint:gosec // next is a page number
	for _, s := range items {
		res.Sessions = append(res.Sessions, toProto(s))
	}
	return connect.NewResponse(res), nil
}

// GetSession returns one session.
func (h *SessionHandler) GetSession(ctx context.Context, req *connect.Request[werbotv1.SessionServiceGetSessionRequest]) (*connect.Response[werbotv1.SessionServiceGetSessionResponse], error) {
	p, id, err := h.authID(ctx, req.Msg.GetId())
	if err != nil {
		return nil, err
	}
	s, err := h.svc.GetSession(ctx, p, id)
	if err != nil {
		return nil, apperr.ToConnect(err)
	}
	return connect.NewResponse(&werbotv1.SessionServiceGetSessionResponse{Session: toProto(s)}), nil
}

// VerifyRecording re-checks the recording of a session the caller may see.
func (h *SessionHandler) VerifyRecording(ctx context.Context, req *connect.Request[werbotv1.VerifyRecordingRequest]) (*connect.Response[werbotv1.VerifyRecordingResponse], error) {
	p, id, err := h.authID(ctx, req.Msg.GetSessionId())
	if err != nil {
		return nil, err
	}
	if _, err := h.svc.GetSession(ctx, p, id); err != nil {
		return nil, apperr.ToConnect(err)
	}
	v, err := h.rec.Verify(ctx, p, id)
	if err != nil {
		return nil, apperr.ToConnect(err)
	}
	return connect.NewResponse(&werbotv1.VerifyRecordingResponse{Verified: v.Verified, Reason: v.Reason}), nil
}

// authID authenticates and parses the session id.
func (h *SessionHandler) authID(ctx context.Context, raw string) (principal.Principal, uuid.UUID, error) {
	p, err := principal.Require(ctx)
	if err != nil {
		return p, uuid.Nil, apperr.ToConnect(err)
	}
	id, err := uuid.Parse(raw)
	if err != nil {
		return p, uuid.Nil, apperr.ToConnect(fmt.Errorf("invalid session id %q: %w", raw, apperr.ErrInvalidArgument))
	}
	return p, id, nil
}

func optionalID(s string) (*uuid.UUID, error) {
	if s == "" {
		return nil, nil //nolint:nilnil // no filter
	}
	id, err := uuid.Parse(s)
	if err != nil {
		return nil, fmt.Errorf("invalid id %q: %w", s, apperr.ErrInvalidArgument)
	}
	return &id, nil
}

// optionalTime is nil for an absent timestamp and apperr.ErrInvalidArgument for an out-of-range one.
func optionalTime(ts *timestamppb.Timestamp) (*time.Time, error) {
	if ts == nil {
		return nil, nil //nolint:nilnil // no filter
	}
	if err := ts.CheckValid(); err != nil {
		return nil, fmt.Errorf("invalid timestamp: %w", errors.Join(apperr.ErrInvalidArgument, err))
	}
	t := ts.AsTime()
	return &t, nil
}

func idString(id *uuid.UUID) string {
	if id == nil {
		return ""
	}
	return id.String()
}

func toProto(s Summary) *werbotv1.Session {
	out := &werbotv1.Session{
		Id: s.ID.String(), UserId: idString(s.UserID), UserLogin: s.UserLogin,
		ResourceId: idString(s.ResourceID), ResourceName: s.ResourceName, GatewayId: idString(s.GatewayID),
		ClientIp: s.ClientIP.String(), StartedAt: timestamppb.New(s.StartedAt), EndReason: s.EndReason,
		Recording: &werbotv1.Recording{Chunks: int32(s.Chunks), Bytes: s.Bytes, Finished: s.Finished}, //nolint:gosec // chunk count is small
	}
	if s.EndedAt != nil {
		out.EndedAt = timestamppb.New(*s.EndedAt)
	}
	return out
}
