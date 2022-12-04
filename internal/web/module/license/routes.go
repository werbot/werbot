package license

import (
	"github.com/gofiber/fiber/v2"

	"github.com/werbot/werbot/internal/grpc"
	"github.com/werbot/werbot/internal/storage/cache"
	"github.com/werbot/werbot/internal/web/middleware"
)

// Handler is ...
type Handler struct {
	app       *fiber.App
	grpc      *grpc.ClientService
	cache     cache.Cache
	publicKey string
}

// New is ...
func New(app *fiber.App, grpc *grpc.ClientService, cache cache.Cache, publicKey string) *Handler {
	return &Handler{
		app:       app,
		grpc:      grpc,
		cache:     cache,
		publicKey: publicKey,
	}
}

// Routes is ...
func (h *Handler) Routes() {
	authMiddleware := middleware.NewAuthMiddleware(h.cache)

	h.app.Get("/v1/license/info", authMiddleware.Execute(), h.getLicenseInfo)

	routes(h, authMiddleware)
}
