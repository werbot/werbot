package system

import (
	"github.com/gofiber/fiber/v2"

	systemmessage "github.com/werbot/werbot/internal/core/system/proto/message"
	systemrpc "github.com/werbot/werbot/internal/core/system/proto/rpc"
	"github.com/werbot/werbot/pkg/utils/webutil"
)

// @Summary Get client IP
// @Description Returns the IP address of the requesting client.
// @Tags system
// @Produce json
// @Success 200 {object} webutil.HTTPResponse{result=string}
// @Router /v1/system/myip [get]
func (h *Handler) myIP(c *fiber.Ctx) error {
	return webutil.StatusOK(c, "IP", c.IP())
}

// @Summary Search countries
// @Description Returns countries whose name matches the query.
// @Tags system
// @Produce json
// @Param name query string true "Country name (minimum 3 characters)"
// @Success 200 {object} webutil.HTTPResponse{result=systemmessage.Countries_Response}
// @Failure 400,401,404,500 {object} webutil.HTTPResponse{result=string}
// @Router /v1/system/countries [get]
func (h *Handler) countries(c *fiber.Ctx) error {
	request := &systemmessage.Countries_Request{}

	_ = webutil.Parse(c, request).Query()

	rClient := systemrpc.NewSystemHandlersClient(h.Grpc)
	countries, err := rClient.Countries(c.UserContext(), request)
	if err != nil {
		return webutil.FromGRPC(c, err)
	}

	return webutil.StatusOK(c, "Countries", countries)
}
