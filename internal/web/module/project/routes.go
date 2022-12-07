package project

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
	projectV1 := h.app.Group("/v1/projects", h.auth)
	projectV1.Get("/", h.getProject)
	projectV1.Post("/", h.addProject)
	projectV1.Patch("/", h.patchProject)
	projectV1.Delete("/", h.deleteProject)
}
