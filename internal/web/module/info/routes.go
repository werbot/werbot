package info

import (
	"github.com/gofiber/fiber/v2"

	"github.com/werbot/werbot/internal/grpc"
)

// Handler is ...
type Handler struct {
	app  *fiber.App
	grpc *grpc.ClientService
	auth fiber.Handler
}

// New is ...
func New(app *fiber.App, grpc *grpc.ClientService, auth fiber.Handler) *Handler {
	return &Handler{
		app:  app,
		grpc: grpc,
		auth: auth,
	}
}

// Routes is ...
func (h *Handler) Routes() {
	h.app.Get("/v1/update", h.auth, h.getUpdate)
	h.app.Get("/v1/info", h.auth, h.getInfo)
	h.app.Get("/v1/version", h.auth, h.getVersion)

	routes(h)
}
