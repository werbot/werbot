package utility

import (
	"github.com/gofiber/fiber/v2"

	"github.com/werbot/werbot/internal/grpc"
	"github.com/werbot/werbot/internal/logger"
)

var log = logger.NewLogger("module/utility")

// Handler is ...
type Handler struct {
	app  *fiber.App
	grpc *grpc.ClientService
}

// NewHandler is ...
func NewHandler(app *fiber.App, grpc *grpc.ClientService) *Handler {
	return &Handler{
		app:  app,
		grpc: grpc,
	}
}

// Routes is ...
func (h *Handler) Routes() {
	h.app.Get("/ip", h.getMyIP)
	h.app.Get("/country", h.getCountry)
}
