package server

import (
	"context"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/werbot/werbot/internal"
	"github.com/werbot/werbot/pkg/webutil"

	pb "github.com/werbot/werbot/api/proto/server"
)

// @Summary      Adding a new server
// @Tags         servers
// @Accept       json
// @Produce      json
// @Param        req         body     pb.AddServer_Request{}
// @Success      200         {object} webutil.HTTPResponse{data=pb.AddServer_Response}
// @Failure      400,401,500 {object} webutil.HTTPResponse
// @Router       /v1/service/server [post]
func (h *handler) addServiceServer(c *fiber.Ctx) error {
	request := new(pb.AddServer_Request)

	if err := c.BodyParser(request); err != nil {
		h.log.Error(err).Send()
		return webutil.StatusBadRequest(c, internal.MsgFailedToValidateBody, nil)
	}

	if err := request.ValidateAll(); err != nil {
		multiError := make(map[string]string)
		for _, err := range err.(pb.AddServer_RequestMultiError) {
			e := err.(pb.AddServer_RequestValidationError)
			multiError[strings.ToLower(e.Field())] = e.Reason()
		}
		return webutil.StatusBadRequest(c, internal.MsgFailedToValidateStruct, multiError)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	rClient := pb.NewServerHandlersClient(h.Grpc.Client)

	server, err := rClient.AddServer(ctx, &pb.AddServer_Request{
		ProjectId: request.GetProjectId(),
		Address:   strings.TrimSpace(request.GetAddress()),
		Port:      request.GetPort(),
		Login:     strings.TrimSpace(request.GetLogin()),
		Scheme:    pb.ServerScheme(pb.ServerAuth_KEY),
	})
	if err != nil {
		return webutil.FromGRPC(c, h.log, err)
	}

	return webutil.StatusOK(c, msgServerKey, server.KeyPublic)
}

func (h *handler) patchServiceServerStatus(c *fiber.Ctx) error {
	return webutil.StatusOK(c, msgServerStatus, "online")
}
