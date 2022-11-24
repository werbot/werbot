package user

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

// NewHandler is ...
func NewHandler(app *fiber.App, grpc *grpc.ClientService, cache cache.Cache) *Handler {
	//log.Info().Msg("Module added")

	return &Handler{
		app:   app,
		grpc:  grpc,
		cache: cache,
	}
}

// Routes is ...
func (h *Handler) Routes() {
	authMiddleware := middleware.NewAuthMiddleware(h.cache)

	userV1 := h.app.Group("/v1/users", authMiddleware.Execute())
	userV1.Get("/", h.getUser)
	userV1.Post("/", h.addUser)
	userV1.Patch("/", h.patchUser)
	userV1.Delete("/", h.deleteUser)

	userV1.Patch("/password", h.patchPassword)
}
