package wellknown

import (
	"github.com/gofiber/fiber/v2"
)

// Handler is ...
type Handler struct {
	app *fiber.App
}

// New is ...
func New(app *fiber.App) *Handler {
	return &Handler{
		app: app,
	}
}

// Routes is ...
func (h *Handler) Routes() {
	h.app.Get("/.well-known/jwks.json", h.jwks)
}
