package event

import (
	"context"
	"errors"
	"time"

	"github.com/gofiber/fiber/v2"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

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
	request := new(eventpb.Events_Request)

	userParameter := middleware.AuthUser(c)

	if err := c.QueryParser(request); err != nil {
		h.log.Error(err).Send()
		return webutil.FromGRPC(c, errors.New("Invalid argument"))
	}

	switch c.Params("name") {
	case "profile":
		request.UserId = userParameter.UserID(c.Params("name_id"))
		request.Id = &eventpb.Events_Request_ProfileId{
			ProfileId: request.UserId,
		}
		request.SortBy = `"created":ASC`
	case "project":
		request.UserId = userParameter.UserID(request.GetUserId())
		request.Id = &eventpb.Events_Request_ProjectId{
			ProjectId: c.Params("name_id"),
		}
		request.SortBy = `"event_project"."created":ASC`
	case "server":
		request.UserId = userParameter.UserID(request.GetUserId())
		request.Id = &eventpb.Events_Request_ServerId{
			ServerId: c.Params("name_id"),
		}
		request.SortBy = `"event_server"."created":ASC`
	default:
		return webutil.FromGRPC(c, errors.New("Invalid argument"))
	}

	if err := grpc.ValidateRequest(request); err != nil {
		return webutil.FromGRPC(c, err, err)
	}

	pagination := webutil.GetPaginationFromCtx(c)
	request.Limit = pagination.GetLimit()
	request.Offset = pagination.GetOffset()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	rClient := eventpb.NewEventHandlersClient(h.Grpc.Client)
	keys, err := rClient.Events(ctx, request)
	if err != nil {
		return webutil.FromGRPC(c, err)
	}
	if keys.GetTotal() == 0 {
		return webutil.FromGRPC(c, status.Error(codes.NotFound, "Not found"))
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
	request := new(eventpb.Event_Request)

	userParameter := middleware.AuthUser(c)

	if err := c.QueryParser(request); err != nil {
		h.log.Error(err).Send()
		return webutil.FromGRPC(c, errors.New("Invalid argument"))
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
		return webutil.FromGRPC(c, errors.New("Invalid argument"))
	}

	if err := grpc.ValidateRequest(request); err != nil {
		return webutil.FromGRPC(c, err, err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	rClient := eventpb.NewEventHandlersClient(h.Grpc.Client)
	key, err := rClient.Event(ctx, request)
	if err != nil {
		return webutil.FromGRPC(c, err)
	}
	if key == nil {
		return webutil.StatusNotFound(c)
	}

	return webutil.StatusOK(c, "event", key)
}
