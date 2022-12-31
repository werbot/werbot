package server

import (
	"context"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/werbot/werbot/internal"
	"github.com/werbot/werbot/internal/web/middleware"
	"github.com/werbot/werbot/pkg/webutil"

	pb "github.com/werbot/werbot/api/proto/server"
)

// @Summary      List of all servers to share for user
// @Tags         share
// @Accept       json
// @Produce      json
// @Param        req         body     userIDReq
// @Success      200         {object} webutil.HTTPResponse
// @Failure      400,401,500 {object} webutil.HTTPResponse
// @Router       /v1/servers/share [get]
func (h *handler) getServersShareForUser(c *fiber.Ctx) error {
	request := new(pb.ListServersShareForUser_Request)

	if err := c.BodyParser(request); err != nil {
		h.log.Error(err).Send()
		return webutil.StatusBadRequest(c, internal.MsgFailedToValidateBody, nil)
	}

	if err := request.ValidateAll(); err != nil {
		multiError := make(map[string]string)
		for _, err := range err.(pb.ListServersShareForUser_RequestMultiError) {
			e := err.(pb.ListServersShareForUser_RequestValidationError)
			multiError[strings.ToLower(e.Field())] = e.Reason()
		}
		return webutil.StatusBadRequest(c, internal.MsgFailedToValidateStruct, multiError)
	}

	userParameter := middleware.AuthUser(c)
	userID := userParameter.UserID(request.GetUserId())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	rClient := pb.NewServerHandlersClient(h.Grpc.Client)

	pagination := webutil.GetPaginationFromCtx(c)
	servers, err := rClient.ListServersShareForUser(ctx, &pb.ListServersShareForUser_Request{
		Limit:  pagination.GetLimit(),
		Offset: pagination.GetOffset(),
		SortBy: pagination.GetSortBy(),
		UserId: userID,
	})
	if err != nil {
		return webutil.FromGRPC(c, h.log, err)
	}
	if servers.Total == 0 {
		return webutil.StatusNotFound(c, internal.MsgNotFound, nil)
	}

	return webutil.StatusOK(c, msgServers, servers)
}

// share the selected server with the user
// request serverReq{user_id:1, project_id:1, server:1}
// POST /v1/servers/share
func (h *handler) postServersShareForUser(c *fiber.Ctx) error {
	request := new(pb.AddServerShareForUser_Request)

	if err := c.BodyParser(request); err != nil {
		h.log.Error(err).Send()
		return webutil.StatusBadRequest(c, internal.MsgFailedToValidateBody, nil)
	}

	if err := request.ValidateAll(); err != nil {
		multiError := make(map[string]string)
		for _, err := range err.(pb.AddServerShareForUser_RequestMultiError) {
			e := err.(pb.AddServerShareForUser_RequestValidationError)
			multiError[strings.ToLower(e.Field())] = e.Reason()
		}
		return webutil.StatusBadRequest(c, internal.MsgFailedToValidateStruct, multiError)
	}

	return webutil.StatusOK(c, msgServerAdded, pb.AddServerShareForUser_Response{})
}

// Updating the settings to the server that they shared
// request serverReq{user_id:1, project_id:1, server:1}
// PATCH /v1/servers/share
func (h *handler) patchServerShareForUser(c *fiber.Ctx) error {
	request := new(pb.UpdateServerShareForUser_Request)

	if err := c.BodyParser(&request); err != nil {
		return webutil.StatusBadRequest(c, internal.MsgFailedToValidateBody, nil)
	}

	if err := request.ValidateAll(); err != nil {
		multiError := make(map[string]string)
		for _, err := range err.(pb.UpdateServerShareForUser_RequestMultiError) {
			e := err.(pb.UpdateServerShareForUser_RequestValidationError)
			multiError[strings.ToLower(e.Field())] = e.Reason()
		}
		return webutil.StatusBadRequest(c, internal.MsgFailedToValidateStruct, multiError)
	}

	return webutil.StatusOK(c, msgServerUpdated, pb.UpdateServerShareForUser_Response{})
}

// Removing from the user list available to him the server
// request userReq{user_id:1, project_id:1}
// DELETE /v1/servers/share
func (h *handler) deleteServerShareForUser(c *fiber.Ctx) error {
	request := new(pb.DeleteServerShareForUser_Request)

	if err := c.BodyParser(&request); err != nil {
		return webutil.StatusBadRequest(c, internal.MsgFailedToValidateBody, nil)
	}

	if err := request.ValidateAll(); err != nil {
		multiError := make(map[string]string)
		for _, err := range err.(pb.DeleteServerShareForUser_RequestMultiError) {
			e := err.(pb.DeleteServerShareForUser_RequestValidationError)
			multiError[strings.ToLower(e.Field())] = e.Reason()
		}
		return webutil.StatusBadRequest(c, internal.MsgFailedToValidateStruct, multiError)
	}

	return webutil.StatusOK(c, msgServerDeleted, pb.DeleteServerShareForUser_Response{})
}
