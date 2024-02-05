package event

import (
	"context"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/werbot/werbot/internal/grpc"
	eventpb "github.com/werbot/werbot/internal/grpc/event/proto"
	"github.com/werbot/werbot/internal/web/middleware"
	"github.com/werbot/werbot/pkg/webutil"
)

// @Summary      Show events records
// @Tags         event
// @Accept       json
// @Produce      json
// @Failure      400,401,404,500 {object} webutil.HTTPResponse
// @Router       /v1/event/:name<alpha>/:name_id<guid> [get]
func (h *Handler) events(c *fiber.Ctx) error {
	request := &eventpb.Events_Request{}

	userParameter := middleware.AuthUser(c)

	if err := c.QueryParser(request); err != nil {
		h.log.Error(err).Send()
		return webutil.StatusBadRequest(c, nil)
	}

	switch c.Params("name") {
	case "profile":
		request.UserId = userParameter.UserID(c.Params("name_id"))
		request.Id = &eventpb.Events_Request_ProfileId{
			ProfileId: request.UserId,
		}
		request.SortBy = `"created_at":DESC`
	case "project":
		request.UserId = userParameter.UserID(request.GetUserId())
		request.Id = &eventpb.Events_Request_ProjectId{
			ProjectId: c.Params("name_id"),
		}
		request.SortBy = `"event_project"."created_at":DESC`
	case "server":
		request.UserId = userParameter.UserID(request.GetUserId())
		request.Id = &eventpb.Events_Request_ServerId{
			ServerId: c.Params("name_id"),
		}
		request.SortBy = `"event_server"."created_at":DESC`
	default:
		return webutil.StatusBadRequest(c, nil)
	}

	if err := grpc.ValidateRequest(request); err != nil {
		return webutil.StatusBadRequest(c, err)
	}

	pagination := webutil.GetPaginationFromCtx(c)
	request.Limit = pagination.Limit
	request.Offset = pagination.Offset

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	rClient := eventpb.NewEventHandlersClient(h.Grpc)
	keys, err := rClient.Events(ctx, request)
	if err != nil {
		return webutil.FromGRPC(c, err)
	}
	if keys.GetTotal() == 0 {
		return webutil.StatusNotFound(c, nil)
	}

	return webutil.StatusOK(c, "event records", keys)
}

// @Summary      Show event info
// @Tags         event
// @Accept       json
// @Produce      json
// @Failure      400,401,404,500 {object} webutil.HTTPResponse
// @Router       /v1/event/:name<alpha>/:name_id<guid>/:event_id<guid> [get]
func (h *Handler) event(c *fiber.Ctx) error {
	request := &eventpb.Event_Request{}

	userParameter := middleware.AuthUser(c)

	if err := c.QueryParser(request); err != nil {
		h.log.Error(err).Send()
		return webutil.StatusBadRequest(c, nil)
	}

	switch c.Params("name") {
	case "profile":
		request.UserId = userParameter.UserID(c.Params("name_id"))
		request.Id = &eventpb.Event_Request_ProfileId{
			ProfileId: c.Params("event_id"),
		}
	case "project":
		request.UserId = userParameter.UserID(request.GetUserId())
		request.Id = &eventpb.Event_Request_ProjectId{
			ProjectId: c.Params("event_id"),
		}
	case "server":
		request.UserId = userParameter.UserID(request.GetUserId())
		request.Id = &eventpb.Event_Request_ServerId{
			ServerId: c.Params("event_id"),
		}
	default:
		return webutil.StatusBadRequest(c, nil)
	}

	if err := grpc.ValidateRequest(request); err != nil {
		return webutil.StatusBadRequest(c, err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	rClient := eventpb.NewEventHandlersClient(h.Grpc)
	key, err := rClient.Event(ctx, request)
	if err != nil {
		return webutil.FromGRPC(c, err)
	}
	if key == nil {
		return webutil.StatusNotFound(c, nil)
	}

	return webutil.StatusOK(c, "event", key)
}
