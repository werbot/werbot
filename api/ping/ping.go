package ping

import (
	"github.com/gofiber/fiber/v2"
)

// @Summary Health check
// @Description Returns pong when the API is reachable.
// @Tags ping
// @Produce plain
// @Success 200 {string} string "pong"
// @Router /ping [get]
func (h *Handler) getPing(c *fiber.Ctx) error {
	return c.Status(fiber.StatusOK).SendString("pong")
}
