package ping

import (
	"github.com/gofiber/fiber/v2"
)

// Handler is ...
type Handler struct {
	app *fiber.App
}

// NewHandler is ...
func NewHandler(app *fiber.App) *Handler {
	return &Handler{
		app: app,
	}
}

// Routes is ...
func (h *Handler) Routes() {
	h.app.Get("/ping", h.getPing)
}
