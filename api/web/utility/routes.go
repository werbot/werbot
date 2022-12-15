package utility

import (
	"github.com/gofiber/fiber/v2"

	"github.com/werbot/werbot/internal/grpc"
	"github.com/werbot/werbot/internal/logger"
)

type handler struct {
	app  *fiber.App
	grpc *grpc.ClientService
	log  logger.Logger
}

// New is ...
func New(app *fiber.App, grpc *grpc.ClientService) *handler {
	log := logger.New("web/utility")
	return &handler{
		app:  app,
		grpc: grpc,
		log:  log,
	}
}

// Routes is ...
func (h *handler) Routes() {
	h.app.Get("/ip", h.getMyIP)
	h.app.Get("/country", h.getCountry)
}
