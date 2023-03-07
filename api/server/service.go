package server

import (
	"context"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/werbot/werbot/internal"
	serverpb "github.com/werbot/werbot/internal/grpc/server/proto"
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
	request := new(serverpb.AddServer_Request)

	// server setting
	if err := c.BodyParser(request); err != nil {
		h.log.Error(err).Send()
		return webutil.StatusBadRequest(c, internal.MsgFailedToValidateBody, nil)
	}

	if err := request.ValidateAll(); err != nil {
		multiError := make(map[string]string)
		for _, err := range err.(serverpb.AddServer_RequestMultiError) {
			e := err.(serverpb.AddServer_RequestValidationError)
			multiError[strings.ToLower(e.Field())] = e.Reason()
		}
		return webutil.StatusBadRequest(c, internal.MsgFailedToValidateStruct, multiError)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	rClient := serverpb.NewServerHandlersClient(h.Grpc.Client)
	server, err := rClient.AddServer(ctx, request)
	if err != nil {
		return webutil.FromGRPC(c, h.log, err)
	}

	return webutil.StatusOK(c, msgServerKey, server)
}

func (h *Handler) updateServiceServerStatus(c *fiber.Ctx) error {
	return webutil.StatusOK(c, msgServerStatus, "online")
}
