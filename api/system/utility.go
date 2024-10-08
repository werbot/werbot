package system

import (
	"github.com/gofiber/fiber/v2"

	systempb "github.com/werbot/werbot/internal/grpc/system/proto/system"
	"github.com/werbot/werbot/pkg/utils/webutil"
)

func (h *Handler) myIP(c *fiber.Ctx) error {
	return webutil.StatusOK(c, "IP", c.IP())
}

func (h *Handler) countries(c *fiber.Ctx) error {
	request := &systempb.Countries_Request{}

	_ = webutil.Parse(c, request).Query()

	rClient := systempb.NewSystemHandlersClient(h.Grpc)
	countries, err := rClient.Countries(c.UserContext(), request)
	if err != nil {
		return webutil.FromGRPC(c, err)
	}

	return webutil.StatusOK(c, "Countries", countries)
}
