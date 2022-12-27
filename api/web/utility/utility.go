package utility

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/werbot/werbot/internal"
	"github.com/werbot/werbot/internal/web/httputil"

	pb "github.com/werbot/werbot/api/proto/utility"
)

func (h *handler) getMyIP(c *fiber.Ctx) error {
	c.Set("Content-Type", "text/plain")
	return c.SendString(c.IP())
}

func (h *handler) getCountry(c *fiber.Ctx) error {
	request := new(pb.ListCountries_Request)

	if err := c.QueryParser(request); err != nil {
		h.log.Error(err).Send()
		return httputil.StatusBadRequest(c, internal.MsgFailedToValidateQuery, nil)
	}

	if err := request.ValidateAll(); err != nil {
		multiError := make(map[string]string)
		for _, err := range err.(pb.ListCountries_RequestMultiError) {
			e := err.(pb.ListCountries_RequestValidationError)
			multiError[strings.ToLower(e.Field())] = e.Reason()
		}
		return httputil.StatusBadRequest(c, internal.MsgFailedToValidateStruct, multiError)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	rClient := pb.NewUtilityHandlersClient(h.Grpc.Client)

	countries, err := rClient.ListCountries(ctx, &pb.ListCountries_Request{
		Name: fmt.Sprintf(`%v`, request.Name),
	})
	if err != nil {
		return httputil.FromGRPC(c, h.log, err)
	}

	return httputil.StatusOK(c, msgCountries, countries)
}
