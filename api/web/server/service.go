package server

import (
	"context"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"

	serverpb "github.com/werbot/werbot/api/proto/server"
	"github.com/werbot/werbot/internal"
	"github.com/werbot/werbot/pkg/webutil"
)

// @Summary      Adding a new server
// @Tags         servers
// @Accept       json
// @Produce      json
// @Param        req         body     serverpb.AddServer_Request{}
// @Success      200         {object} webutil.HTTPResponse{data=serverpb.AddServer_Response}
// @Failure      400,401,500 {object} webutil.HTTPResponse
// @Router       /v1/service/server [post]
func (h *Handler) addServiceServer(c *fiber.Ctx) error {
	requestServer := new(serverpb.AddServer_Request)
	requestAccess := new(serverpb.AddServerAccess_Request)

	// server setting
	if err := c.BodyParser(requestServer); err != nil {
		h.log.Error(err).Send()
		return webutil.StatusBadRequest(c, internal.MsgFailedToValidateBody, nil)
	}

	if err := requestServer.ValidateAll(); err != nil {
		multiError := make(map[string]string)
		for _, err := range err.(serverpb.AddServer_RequestMultiError) {
			e := err.(serverpb.AddServer_RequestValidationError)
			multiError[strings.ToLower(e.Field())] = e.Reason()
		}
		return webutil.StatusBadRequest(c, internal.MsgFailedToValidateStruct, multiError)
	}

	// access setting
	if err := c.BodyParser(requestAccess); err != nil {
		h.log.Error(err).Send()
		return webutil.StatusBadRequest(c, internal.MsgFailedToValidateBody, nil)
	}

	if err := requestAccess.ValidateAll(); err != nil {
		multiError := make(map[string]string)
		for _, err := range err.(serverpb.AddServerAccess_RequestMultiError) {
			e := err.(serverpb.AddServerAccess_RequestValidationError)
			multiError[strings.ToLower(e.Field())] = e.Reason()
		}
		return webutil.StatusBadRequest(c, internal.MsgFailedToValidateStruct, multiError)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	rClient := serverpb.NewServerHandlersClient(h.Grpc.Client)
	server, err := rClient.AddServer(ctx, requestServer)
	if err != nil {
		return webutil.FromGRPC(c, h.log, err)
	}

	requestAccess.ServerId = server.GetServerId()
	key, err := rClient.AddServerAccess(ctx, requestAccess)
	if err != nil {
		return webutil.FromGRPC(c, h.log, err)
	}

	return webutil.StatusOK(c, msgServerKey, key)
}

func (h *Handler) updateServiceServerStatus(c *fiber.Ctx) error {
	return webutil.StatusOK(c, msgServerStatus, "online")
}
