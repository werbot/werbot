package server

import (
	"context"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/werbot/werbot/internal/grpc"
	serverpb "github.com/werbot/werbot/internal/grpc/server/proto"
	"github.com/werbot/werbot/internal/web/middleware"
	"github.com/werbot/werbot/pkg/webutil"
)

// @Summary      List of all servers to share for user
// @Tags         share
// @Accept       json
// @Produce      json
// @Param        req         body     userIDReq
// @Success      200         {object} webutil.HTTPResponse
// @Failure      400,401,500 {object} webutil.HTTPResponse
// @Router       /v1/servers/share [get]
func (h *Handler) serversShareForUser(c *fiber.Ctx) error {
	request := &serverpb.ListShareServers_Request{}

	if err := c.BodyParser(request); err != nil {
		return webutil.StatusBadRequest(c, "The body of the request is damaged")
	}

	if err := grpc.ValidateRequest(request); err != nil {
		return webutil.StatusBadRequest(c, err)
	}

	pagination := webutil.GetPaginationFromCtx(c)
	userParameter := middleware.AuthUser(c)
	request.UserId = userParameter.UserID(request.GetUserId())
	request.Limit = pagination.Limit
	request.Offset = pagination.Offset
	request.SortBy = pagination.SortBy

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	rClient := serverpb.NewServerHandlersClient(h.Grpc)
	servers, err := rClient.ListShareServers(ctx, request)
	if err != nil {
		return webutil.FromGRPC(c, err)
	}
	if servers.Total == 0 {
		return webutil.StatusNotFound(c, nil)
	}

	return webutil.StatusOK(c, "servers", servers)
}

// share the selected server with the user
// request serverReq{user_id:1, project_id:1, server:1}
// POST /v1/servers/share
func (h *Handler) addServersShareForUser(c *fiber.Ctx) error {
	request := &serverpb.AddShareServer_Request{}

	if err := c.BodyParser(request); err != nil {
		return webutil.StatusBadRequest(c, "The body of the request is damaged")
	}

	if err := grpc.ValidateRequest(request); err != nil {
		return webutil.StatusBadRequest(c, err)
	}

	return webutil.StatusOK(c, "server added", serverpb.AddShareServer_Response{})
}

// Updating the settings to the server that they shared
// request serverReq{user_id:1, project_id:1, server:1}
// PATCH /v1/servers/share
func (h *Handler) updateServerShareForUser(c *fiber.Ctx) error {
	request := &serverpb.UpdateShareServer_Request{}

	if err := c.BodyParser(&request); err != nil {
		return webutil.StatusBadRequest(c, nil)
	}

	if err := grpc.ValidateRequest(request); err != nil {
		return webutil.StatusBadRequest(c, err)
	}

	return webutil.StatusOK(c, "server updated", serverpb.UpdateShareServer_Response{})
}

// Removing from the user list available to him the server
// request userReq{user_id:1, project_id:1}
// DELETE /v1/servers/share
func (h *Handler) deleteServerShareForUser(c *fiber.Ctx) error {
	request := &serverpb.DeleteShareServer_Request{}

	if err := c.BodyParser(&request); err != nil {
		return webutil.StatusBadRequest(c, nil)
	}

	if err := grpc.ValidateRequest(request); err != nil {
		return webutil.StatusBadRequest(c, err)
	}

	return webutil.StatusOK(c, "server deleted", serverpb.DeleteShareServer_Response{})
}
