package customer

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

	customerV1 := h.app.Group("/v1/customers", authMiddleware.Execute())
	customerV1.Get("/", h.getCustomer)
	customerV1.Delete("/", h.deleteCustomer)
}
