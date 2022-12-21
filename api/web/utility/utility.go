package utility

import (
	"context"
	"fmt"
	"time"

	"github.com/gofiber/fiber/v2"
	"google.golang.org/grpc/status"

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
	input := &pb.GetCountry_Request{}
	c.QueryParser(input)
	if err := validate.Struct(input); err != nil {
		return httputil.StatusBadRequest(c, internal.MsgValidateBodyParams, err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	rClient := pb.NewUtilityHandlersClient(h.Grpc.Client)

	countries, err := rClient.GetCountry(ctx, &pb.GetCountry_Request{
		Name: fmt.Sprintf(`%v`, input.Name),
	})
	if err != nil {
		se, _ := status.FromError(err)

		if se.Message() == internal.MsgNotFound {
			return httputil.StatusNotFound(c, se.Message(), nil)
		}

		return httputil.InternalServerError(c, "Having select problems", nil)
	}

	return httputil.StatusOK(c, "Countries list", countries)
}
