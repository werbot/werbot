package ping

import (
	"github.com/gofiber/fiber/v2"
)

func (h *Handler) getPing(c *fiber.Ctx) error {
	return c.Status(fiber.StatusOK).SendString("pong")
}
