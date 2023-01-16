package utility

import (
	"context"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"

	utilitypb "github.com/werbot/werbot/api/proto/utility"
	"github.com/werbot/werbot/internal"
	"github.com/werbot/werbot/pkg/webutil"
)

func (h *Handler) getMyIP(c *fiber.Ctx) error {
	c.Set("Content-Type", "text/plain")
	return c.SendString(c.IP())
}

func (h *Handler) getCountry(c *fiber.Ctx) error {
	request := new(utilitypb.Countries_Request)

	if err := c.QueryParser(request); err != nil {
		h.log.Error(err).Send()
		return webutil.StatusBadRequest(c, internal.MsgFailedToValidateQuery, nil)
	}

	if err := request.ValidateAll(); err != nil {
		multiError := make(map[string]string)
		for _, err := range err.(utilitypb.Countries_RequestMultiError) {
			e := err.(utilitypb.Countries_RequestValidationError)
			multiError[strings.ToLower(e.Field())] = e.Reason()
		}
		return webutil.StatusBadRequest(c, internal.MsgFailedToValidateStruct, multiError)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	rClient := utilitypb.NewUtilityHandlersClient(h.Grpc.Client)
	countries, err := rClient.Countries(ctx, request)
	if err != nil {
		return webutil.FromGRPC(c, h.log, err)
	}

	return webutil.StatusOK(c, msgCountries, countries)
}
