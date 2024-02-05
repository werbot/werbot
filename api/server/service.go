package server

import (
	"context"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/werbot/werbot/internal/grpc"
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
	request := &serverpb.AddServer_Request{}

	if err := c.BodyParser(request); err != nil {
		return webutil.StatusBadRequest(c, "The body of the request is damaged")
	}

	if err := grpc.ValidateRequest(request); err != nil {
		return webutil.StatusBadRequest(c, err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	rClient := serverpb.NewServerHandlersClient(h.Grpc)
	server, err := rClient.AddServer(ctx, request)
	if err != nil {
		return webutil.FromGRPC(c, err)
	}

	return webutil.StatusOK(c, "server key", server)
}

func (h *Handler) updateServiceServerStatus(c *fiber.Ctx) error {
	return webutil.StatusOK(c, "server status", "online")
}
