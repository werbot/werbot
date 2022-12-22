package server

import (
	"context"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/werbot/werbot/internal"
	"github.com/werbot/werbot/internal/utils/validate"
	"github.com/werbot/werbot/internal/web/httputil"

	pb "github.com/werbot/werbot/api/proto/server"
)

// @Summary      Adding a new server
// @Tags         servers
// @Accept       json
// @Produce      json
// @Param        req         body     pb.AddServer_Request{}
// @Success      200         {object} httputil.HTTPResponse{data=pb.AddServer_Response}
// @Failure      400,401,500 {object} httputil.HTTPResponse
// @Router       /v1/service/server [post]
func (h *handler) addServiceServer(c *fiber.Ctx) error {
	input := new(pb.AddServer_Request)

	if err := c.BodyParser(input); err != nil {
		h.log.Error(err).Send()
		return httputil.StatusBadRequest(c, internal.MsgFailedToValidateBody, nil)
	}
	if err := validate.Struct(&input); err != nil {
		return httputil.StatusBadRequest(c, internal.MsgFailedToValidateBody, err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	rClient := pb.NewServerHandlersClient(h.Grpc.Client)

	server, err := rClient.AddServer(ctx, &pb.AddServer_Request{
		ProjectId: input.GetProjectId(),
		Address:   strings.TrimSpace(input.GetAddress()),
		Port:      input.GetPort(),
		Login:     strings.TrimSpace(input.GetLogin()),
		Scheme:    pb.ServerScheme(pb.ServerAuth_KEY),
	})
	if err != nil {
		return httputil.ErrorGRPC(c, h.log, err)
	}

	return httputil.StatusOK(c, "Server key", server.KeyPublic)
}

func (h *handler) patchServiceServerStatus(c *fiber.Ctx) error {
	return httputil.StatusOK(c, "Server status", "online")
}
