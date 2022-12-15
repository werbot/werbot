package ping

import "github.com/gofiber/fiber/v2"

func (h *handler) getPing(c *fiber.Ctx) error {
	return c.Status(fiber.StatusOK).SendString("pong")
}
