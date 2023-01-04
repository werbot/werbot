package utility

import (
	"context"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/werbot/werbot/internal"
	"github.com/werbot/werbot/pkg/webutil"

	pb "github.com/werbot/werbot/api/proto/utility"
)

func (h *Handler) getMyIP(c *fiber.Ctx) error {
	c.Set("Content-Type", "text/plain")
	return c.SendString(c.IP())
}

func (h *Handler) getCountry(c *fiber.Ctx) error {
	request := new(pb.ListCountries_Request)

	if err := c.QueryParser(request); err != nil {
		h.log.Error(err).Send()
		return webutil.StatusBadRequest(c, internal.MsgFailedToValidateQuery, nil)
	}

	if err := request.ValidateAll(); err != nil {
		multiError := make(map[string]string)
		for _, err := range err.(pb.ListCountries_RequestMultiError) {
			e := err.(pb.ListCountries_RequestValidationError)
			multiError[strings.ToLower(e.Field())] = e.Reason()
		}
		return webutil.StatusBadRequest(c, internal.MsgFailedToValidateStruct, multiError)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	rClient := pb.NewUtilityHandlersClient(h.Grpc.Client)
	countries, err := rClient.ListCountries(ctx, request)
	if err != nil {
		return webutil.FromGRPC(c, h.log, err)
	}

	return webutil.StatusOK(c, msgCountries, countries)
}
