package auth

import (
	"github.com/gofiber/fiber/v2"

	"github.com/werbot/werbot/internal/cache"
	"github.com/werbot/werbot/internal/grpc"
	"github.com/werbot/werbot/internal/web/middleware"
)

//var moduleName = "module/auth"
//var log = logger.NewLogger(moduleName)

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

	g := h.app.Group("/auth")
	g.Post("/login", h.postLogin)
	g.Post("/refresh", h.postRefresh)
	g.Post("/logout", authMiddleware.Execute(), h.postLogout)

	g.Post("/password_reset/:reset_token?", h.postResetPassword)

	g.Get("/profile", authMiddleware.Execute(), h.getProfile)
}
