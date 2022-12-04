package project

import (
	"github.com/gofiber/fiber/v2"

	"github.com/werbot/werbot/internal/grpc"
	"github.com/werbot/werbot/internal/storage/cache"
	"github.com/werbot/werbot/internal/web/middleware"
)

// Handler is ...
type Handler struct {
	app   *fiber.App
	grpc  *grpc.ClientService
	cache cache.Cache
}

// New is ...
func New(app *fiber.App, grpc *grpc.ClientService, cache cache.Cache) *Handler {
	return &Handler{
		app:   app,
		grpc:  grpc,
		cache: cache,
	}
}

// Routes is ...
func (h *Handler) Routes() {
	authMiddleware := middleware.NewAuthMiddleware(h.cache)

	projectV1 := h.app.Group("/v1/projects", authMiddleware.Execute())
	projectV1.Get("/", h.getProject)
	projectV1.Post("/", h.addProject)
	projectV1.Patch("/", h.patchProject)
	projectV1.Delete("/", h.deleteProject)
}
