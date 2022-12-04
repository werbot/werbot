package key

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

	keyV1 := h.app.Group("/v1/keys", authMiddleware.Execute())
	keyV1.Get("/generate", h.getGenerateNewKey)

	keyV1.Get("/", h.getKey)
	keyV1.Post("/", h.addKey)
	keyV1.Patch("/", h.patchKey)
	keyV1.Delete("/", h.deleteKey)
}
