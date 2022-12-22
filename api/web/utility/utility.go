package utility

import (
	"context"
	"fmt"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/werbot/werbot/internal"
	"github.com/werbot/werbot/internal/utils/validate"
	"github.com/werbot/werbot/internal/web/httputil"

	pb "github.com/werbot/werbot/api/proto/utility"
)

func (h *handler) getMyIP(c *fiber.Ctx) error {
	c.Set("Content-Type", "text/plain")
	return c.SendString(c.IP())
}

func (h *handler) getCountry(c *fiber.Ctx) error {
	input := new(pb.ListCountries_Request)

	if err := c.QueryParser(input); err != nil {
		h.log.Error(err).Send()
		return httputil.StatusBadRequest(c, internal.MsgFailedToValidateQuery, nil)
	}
	if err := validate.Struct(input); err != nil {
		return httputil.StatusBadRequest(c, internal.MsgFailedToValidateStruct, err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	rClient := pb.NewUtilityHandlersClient(h.Grpc.Client)

	countries, err := rClient.ListCountries(ctx, &pb.ListCountries_Request{
		Name: fmt.Sprintf(`%v`, input.Name),
	})
	if err != nil {
		return httputil.ErrorGRPC(c, h.log, err)
	}

	return httputil.StatusOK(c, "Countries list", countries)
}
