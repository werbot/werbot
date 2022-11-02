package member

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

	memberV1 := h.app.Group("/v1/members", authMiddleware.Execute())
	// Project section
	memberV1.Get("/", h.getMember)
	memberV1.Post("/", h.addMember)
	memberV1.Patch("/", h.patchMember)
	memberV1.Delete("/", h.deleteMember)

	memberV1.Patch("/active", h.patchMemberStatus)

	memberV1.Get("/search", h.getUsersWithoutProject)        // for project
	memberV1.Get("/server/search", h.getMemberWithoutServer) // for server

	// Server section
	memberV1.Get("/server", h.getServerMember)
	memberV1.Post("/server", h.addServerMember)
	memberV1.Patch("/server", h.patchServerMember)
	memberV1.Delete("/server", h.deleteServerMember)
}
