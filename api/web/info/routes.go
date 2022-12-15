package info

import (
	"github.com/gofiber/fiber/v2"

	"github.com/werbot/werbot/internal/grpc"
	"github.com/werbot/werbot/internal/logger"
)

// Handler is ...
type Handler struct {
	app  *fiber.App
	grpc *grpc.ClientService
	auth fiber.Handler
	log  logger.Logger
}

// New is ...
func New(app *fiber.App, grpc *grpc.ClientService, auth fiber.Handler) *Handler {
	log := logger.New("web/auth")

	return &Handler{
		app:  app,
		grpc: grpc,
		auth: auth,
		log:  log,
	}
}

// Routes is ...
func (h *Handler) Routes() {
	h.app.Get("/v1/update", h.auth, h.getUpdate)
	h.app.Get("/v1/info", h.auth, h.getInfo)
	h.app.Get("/v1/version", h.auth, h.getVersion)

	routes(h)
}
