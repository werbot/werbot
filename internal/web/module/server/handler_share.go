package server

import (
	"context"
	"time"

	"github.com/gofiber/fiber/v2"
	"google.golang.org/grpc/status"

	"github.com/werbot/werbot/internal/message"
	"github.com/werbot/werbot/internal/utils/validator"
	"github.com/werbot/werbot/internal/web/httputil"
	"github.com/werbot/werbot/internal/web/middleware"

	pb "github.com/werbot/werbot/internal/grpc/proto/server"
)

type userIDReq struct {
	UserID string `json:"user_id,omitempty" validate:"uuid"`
}

// @Summary      List of all servers to share for user
// @Tags         share
// @Accept       json
// @Produce      json
// @Param        req         body     userIDReq
// @Success      200         {object} httputil.HTTPResponse
// @Failure      400,401,500 {object} httputil.HTTPResponse
// @Router       /v1/servers/share [get]
func (h *Handler) getServersShareForUser(c *fiber.Ctx) error {
	input := userIDReq{}
	c.BodyParser(&input)
	if err := validator.ValidateStruct(input); err != nil {
		return httputil.StatusBadRequest(c, message.ErrValidateBodyParams, err)
	}

	userParameter := middleware.GetUserParameters(c)
	userID := userParameter.GetUserID(input.UserID)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	rClient := pb.NewServerHandlersClient(h.grpc.Client)

	pagination := httputil.GetPaginationFromCtx(c)
	servers, err := rClient.ListServersShareForUser(ctx, &pb.ListServersShareForUser_Request{
		Limit:  pagination.GetLimit(),
		Offset: pagination.GetOffset(),
		SortBy: pagination.GetSortBy(),
		UserId: userID,
	})
	if err != nil {
		se, _ := status.FromError(err)
		if se.Message() != "" {
			return httputil.StatusBadRequest(c, se.Message(), nil)
		}
		return httputil.InternalServerError(c, message.ErrUnexpectedError, nil)
	}
	if servers.Total == 0 {
		return httputil.StatusNotFound(c, message.ErrNotFound, nil)
	}

	return httputil.StatusOK(c, "Shared servers list", servers)
}

// share the selected server with the user
// request serverReq{user_id:1, project_id:1, server:1}
// POST /v1/servers/share
func (h *Handler) postServersShareForUser(c *fiber.Ctx) error {
	input := new(pb.CreateServerShareForUser_Request)
	if err := c.BodyParser(input); err != nil {
		return httputil.StatusBadRequest(c, message.ErrBadQueryParams, nil)
	}

	if err := validator.ValidateStruct(input); err != nil {
		return httputil.StatusBadRequest(c, message.ErrValidateBodyParams, err)
	}

	return httputil.StatusOK(c, "message", pb.CreateServerShareForUser_Response{})
}

// Updating the settings to the server that they shared
// request serverReq{user_id:1, project_id:1, server:1}
// PATCH /v1/servers/share
func (h *Handler) patchServerShareForUser(c *fiber.Ctx) error {
	input := new(pb.UpdateServerShareForUser_Request)
	if err := c.BodyParser(&input); err != nil {
		return httputil.StatusBadRequest(c, message.ErrBadQueryParams, nil)
	}

	if err := validator.ValidateStruct(input); err != nil {
		return httputil.StatusBadRequest(c, message.ErrValidateBodyParams, err)
	}

	return httputil.StatusOK(c, "message", pb.UpdateServerShareForUser_Response{})
}

// Removing from the user list available to him the server
// request userReq{user_id:1, project_id:1}
// DELETE /v1/servers/share
func (h *Handler) deleteServerShareForUser(c *fiber.Ctx) error {
	input := new(pb.DeleteServerShareForUser_Request)
	if err := c.BodyParser(&input); err != nil {
		return httputil.StatusBadRequest(c, message.ErrBadQueryParams, nil)
	}

	if err := validator.ValidateStruct(input); err != nil {
		return httputil.StatusBadRequest(c, message.ErrValidateBodyParams, err)
	}

	return httputil.StatusOK(c, "message", pb.DeleteServerShareForUser_Response{})
}
