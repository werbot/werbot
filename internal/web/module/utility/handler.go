package utility

import (
	"context"
	"fmt"
	"time"

	"github.com/gofiber/fiber/v2"
	"google.golang.org/grpc/status"

	"github.com/werbot/werbot/internal/message"
	"github.com/werbot/werbot/internal/utils/validator"
	"github.com/werbot/werbot/internal/web/httputil"

	pb "github.com/werbot/werbot/internal/grpc/proto/utility"
)

func (h *Handler) getMyIP(c *fiber.Ctx) error {
	c.Set("Content-Type", "text/plain")
	return c.SendString(c.IP())
}

func (h *Handler) getCountry(c *fiber.Ctx) error {
	input := &pb.GetCountry_Request{}
	c.QueryParser(input)
	if err := validator.ValidateStruct(input); err != nil {
		return httputil.StatusBadRequest(c, message.ErrValidateBodyParams, err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	rClient := pb.NewUtilityHandlersClient(h.grpc.Client)

	countries, err := rClient.GetCountry(ctx, &pb.GetCountry_Request{
		Name: fmt.Sprintf(`%v`, input.Name),
	})
	if err != nil {
		se, _ := status.FromError(err)

		if se.Message() == message.ErrNotFound {
			return httputil.StatusNotFound(c, se.Message(), nil)
		}

		return httputil.InternalServerError(c, "Having select problems", nil)
	}

	return httputil.StatusOK(c, "Countries list", countries)
}
