package utility

import (
	"context"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/werbot/werbot/internal/grpc"
	utilitypb "github.com/werbot/werbot/internal/grpc/utility/proto"
	"github.com/werbot/werbot/pkg/webutil"
)

func (h *Handler) getMyIP(c *fiber.Ctx) error {
	c.Set("Content-Type", "text/plain")
	return c.SendString(c.IP())
}

func (h *Handler) getCountry(c *fiber.Ctx) error {
	request := &utilitypb.Countries_Request{}

	if err := c.QueryParser(request); err != nil {
		h.log.Error(err).Send()
		return webutil.StatusBadRequest(c, nil)
	}

	if err := grpc.ValidateRequest(request); err != nil {
		return webutil.StatusBadRequest(c, err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	rClient := utilitypb.NewUtilityHandlersClient(h.Grpc)
	countries, err := rClient.Countries(ctx, request)
	if err != nil {
		return webutil.FromGRPC(c, err)
	}

	return webutil.StatusOK(c, "countries", countries)
}
