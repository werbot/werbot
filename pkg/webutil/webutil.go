package webutil

import (
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/utils"

	"github.com/werbot/werbot/pkg/logger"
)

// HTTPResponse represents response body of API
type HTTPResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Result  any    `json:"result,omitempty"`
}

// Response is ...
func Response(c *fiber.Ctx, status int, message string, data any) error {
	var success bool
	if status == 200 {
		success = true
	}
	if len(message) > 0 {
		return c.Status(status).JSON(HTTPResponse{
			Success: success,
			Message: message,
			Result:  data,
		})
	}

	return c.Status(status).JSON(data)
}

// StatusOK - HTTP error code 400
func StatusOK(c *fiber.Ctx, message string, data any) error {
	return Response(c, 200, message, data)
}

// StatusBadRequest - HTTP error code 400
func StatusBadRequest(c *fiber.Ctx, message string, err any) error {
	return Response(c, 400, message, err)
}

// StatusUnauthorized  - HTTP error code 401
func StatusUnauthorized(c *fiber.Ctx, message string, err any) error {
	return Response(c, 401, message, err)
}

// StatusNotFound - HTTP error code 404
func StatusNotFound(c *fiber.Ctx, message string, err any) error {
	return Response(c, 404, message, err)
}

// InternalServerError - HTTP error code 500
func InternalServerError(c *fiber.Ctx, message string, err any) error {
	return Response(c, 500, message, err)
}

// FromGRPC is ...
func FromGRPC(c *fiber.Ctx, log logger.Logger, err error) error {
	notFound := strings.ToLower(utils.StatusMessage(404))
	if err.Error() == notFound {
		return StatusNotFound(c, notFound, nil)
	}

	log.FromGRPC(err).CallerSkipFrame(1).Send()
	return InternalServerError(c, strings.ToLower(utils.StatusMessage(500)), nil)
}
