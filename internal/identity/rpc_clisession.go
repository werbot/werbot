package identity

import (
	"context"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/timestamppb"

	werbotv1 "github.com/werbot/werbot/gen/werbot/v1"
	"github.com/werbot/werbot/internal/platform/apperr"
	"github.com/werbot/werbot/internal/platform/principal"
)

func cliSessionToProto(c CLISession) *werbotv1.CLISession {
	out := &werbotv1.CLISession{
		Id: c.ID.String(), UserId: c.UserID.String(), ClientId: c.ClientID, Hostname: c.Hostname,
		CreatedAt: timestamppb.New(c.CreatedAt), LastUsedAt: timestamppb.New(c.LastUsedAt),
		ExpiresAt: timestamppb.New(c.ExpiresAt), AbsoluteExpiresAt: timestamppb.New(c.AbsoluteExpiresAt),
		Current: c.Current,
	}
	if c.IP.IsValid() {
		out.Ip = c.IP.String()
	}
	return out
}

// ListCLISessions returns the live CLI sessions of the caller or, for an
// owner or admin, of user_id.
func (h *Handler) ListCLISessions(ctx context.Context, req *connect.Request[werbotv1.ListCLISessionsRequest]) (*connect.Response[werbotv1.ListCLISessionsResponse], error) {
	p, err := principal.Require(ctx)
	if err != nil {
		return nil, apperr.ToConnect(err)
	}
	var userID *uuid.UUID
	if req.Msg.UserId != nil {
		id, err := parseID(req.Msg.GetUserId())
		if err != nil {
			return nil, apperr.ToConnect(err)
		}
		userID = &id
	}
	list, err := h.svc.ListCLISessions(ctx, p, userID)
	if err != nil {
		return nil, apperr.ToConnect(err)
	}
	res := &werbotv1.ListCLISessionsResponse{Sessions: make([]*werbotv1.CLISession, len(list))}
	for i, c := range list {
		res.Sessions[i] = cliSessionToProto(c)
	}
	return connect.NewResponse(res), nil
}

// RevokeCLISession ends a CLI session (reason "revoked").
func (h *Handler) RevokeCLISession(ctx context.Context, req *connect.Request[werbotv1.RevokeCLISessionRequest]) (*connect.Response[werbotv1.RevokeCLISessionResponse], error) {
	p, err := principal.Require(ctx)
	if err != nil {
		return nil, apperr.ToConnect(err)
	}
	id, err := parseID(req.Msg.GetId())
	if err != nil {
		return nil, apperr.ToConnect(err)
	}
	if err := h.svc.RevokeCLISession(ctx, p, id, "revoked"); err != nil {
		return nil, apperr.ToConnect(err)
	}
	return connect.NewResponse(&werbotv1.RevokeCLISessionResponse{}), nil
}
