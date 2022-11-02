package info

import (
	"github.com/gofiber/fiber/v2"

	"github.com/werbot/werbot/internal/cache"
	"github.com/werbot/werbot/internal/grpc"
	"github.com/werbot/werbot/internal/web/middleware"
)

// Handler is ...
type Handler struct {
	app   *fiber.App
	grpc  *grpc.ClientService
	cache cache.Cache
}

// NewHandler is ...
func NewHandler(app *fiber.App, grpc *grpc.ClientService, cache cache.Cache) *Handler {
	return &Handler{
		app:   app,
		grpc:  grpc,
		cache: cache,
	}
}

// Routes is ...
func (h *Handler) Routes() {
	authMiddleware := middleware.NewAuthMiddleware(h.cache)

	h.app.Get("/v1/update", authMiddleware.Execute(), h.getUpdate)
	h.app.Get("/v1/info", authMiddleware.Execute(), h.getInfo)
	h.app.Get("/v1/version", authMiddleware.Execute(), h.getVersion)

	routes(h, authMiddleware)
}
