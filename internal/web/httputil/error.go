package httputil

import (
	"github.com/gofiber/fiber/v2"
	"github.com/werbot/werbot/internal"
	"github.com/werbot/werbot/internal/logger"
)

// StatusBadRequest - HTTP error code 400
func StatusBadRequest(c *fiber.Ctx, message string, err any) error {
	return NewError(c, 400, message, err)
}

// StatusUnauthorized  - HTTP error code 401
func StatusUnauthorized(c *fiber.Ctx, message string, err any) error {
	return NewError(c, 401, message, err)
}

// StatusNotFound - HTTP error code 404
func StatusNotFound(c *fiber.Ctx, message string, err any) error {
	return NewError(c, 404, message, err)
}

// InternalServerError - HTTP error code 500
func InternalServerError(c *fiber.Ctx, message string, err any) error {
	return NewError(c, 500, message, err)
}

// NewError is ...
func NewError(c *fiber.Ctx, status int, message string, data any) error {
	if len(message) > 0 {
		return c.Status(status).JSON(HTTPResponse{
			Success: false,
			Message: message,
			Result:  data,
		})
	}

	return c.Status(status).JSON(data)
}

// ErrorGRPC is ...
func ErrorGRPC(c *fiber.Ctx, log logger.Logger, err error) error {
	if err.Error() == internal.MsgNotFound {
		return StatusNotFound(c, internal.MsgNotFound, nil)
	}

	log.Error(err).CallerSkipFrame(1).Send()
	return InternalServerError(c, internal.MsgUnexpectedError, nil)
}
