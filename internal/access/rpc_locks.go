package access

import (
	"context"

	"connectrpc.com/connect"

	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
	"github.com/werbot/werbot/gen/werbot/v1/werbotv1connect"
	"github.com/werbot/werbot/internal/platform/apperr"
)

// LockHandler serves LockService; owner and admin only.
type LockHandler struct {
	werbotv1connect.UnimplementedLockServiceHandler
	svc *Service
}

// NewLockHandler returns the Connect handler for LockService.
func NewLockHandler(s *Service) *LockHandler { return &LockHandler{svc: s} }

// ListLocks lists the unexpired locks, newest first.
func (h *LockHandler) ListLocks(ctx context.Context, _ *connect.Request[werbotv1.ListLocksRequest]) (*connect.Response[werbotv1.ListLocksResponse], error) {
	p, err := adminPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	ls, err := h.svc.ListLocks(ctx, p)
	if err != nil {
		return nil, apperr.ToConnect(err)
	}
	res := &werbotv1.ListLocksResponse{}
	for _, l := range ls {
		res.Locks = append(res.Locks, lockToProto(l))
	}
	return connect.NewResponse(res), nil
}

// CreateLock denies access to a target of the organization.
func (h *LockHandler) CreateLock(ctx context.Context, req *connect.Request[werbotv1.CreateLockRequest]) (*connect.Response[werbotv1.CreateLockResponse], error) {
	p, err := adminPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	m := req.Msg
	in := LockInput{TargetType: m.GetTargetType(), Reason: m.GetReason(), ExpiresIn: m.GetExpiresIn().AsDuration()}
	if in.TargetID, err = optionalID(m.GetTargetId()); err != nil {
		return nil, apperr.ToConnect(err)
	}
	l, err := h.svc.CreateLock(ctx, p, in)
	if err != nil {
		return nil, apperr.ToConnect(err)
	}
	return connect.NewResponse(&werbotv1.CreateLockResponse{Lock: lockToProto(LockView{Lock: l, CreatedBy: &p.UserID})}), nil
}

// DeleteLock removes a lock.
func (h *LockHandler) DeleteLock(ctx context.Context, req *connect.Request[werbotv1.DeleteLockRequest]) (*connect.Response[werbotv1.DeleteLockResponse], error) {
	p, err := adminPrincipal(ctx)
	if err != nil {
		return nil, err
	}
	id, err := parseID(req.Msg.GetId())
	if err != nil {
		return nil, apperr.ToConnect(err)
	}
	if err := h.svc.DeleteLock(ctx, p, id); err != nil {
		return nil, apperr.ToConnect(err)
	}
	return connect.NewResponse(&werbotv1.DeleteLockResponse{}), nil
}

func lockToProto(l LockView) *werbotv1.Lock {
	return &werbotv1.Lock{Id: l.ID.String(), TargetType: l.TargetType, TargetId: idString(l.TargetID), Reason: l.Reason,
		ExpiresAt: timestamp(l.ExpiresAt), CreatedBy: idString(l.CreatedBy), CreatedAt: timestamp(&l.CreatedAt)}
}
