package httputil

import (
	"github.com/gofiber/fiber/v2"
)

// StatusOK - HTTP error code 400
func StatusOK(c *fiber.Ctx, message string, data any) error {
	return NewGood(c, 200, message, data)
}

// NewGood is ...
func NewGood(c *fiber.Ctx, status int, message string, data any) error {
	if len(message) > 0 {
		return c.Status(status).JSON(HTTPResponse{
			Success: true,
			Message: message,
			Result:  data,
		})
	}

	return c.Status(status).JSON(data)
}
