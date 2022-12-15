package auth

import (
	"github.com/gofiber/fiber/v2"

	"github.com/werbot/werbot/internal/grpc"
	"github.com/werbot/werbot/internal/logger"
	"github.com/werbot/werbot/internal/storage/cache"
)

// Handler is ...
type Handler struct {
	app   *fiber.App
	grpc  *grpc.ClientService
	cache cache.Cache
	auth  fiber.Handler
	log   logger.Logger
}

// New is ...
func New(app *fiber.App, grpc *grpc.ClientService, cache cache.Cache, auth fiber.Handler) *Handler {
	log := logger.New("module/auth")

	return &Handler{
		app:   app,
		grpc:  grpc,
		cache: cache,
		auth:  auth,
		log:   log,
	}
}

// Routes is ...
func (h *Handler) Routes() {
	g := h.app.Group("/auth")
	g.Post("/signin", h.signIn)
	g.Post("/refresh", h.refresh)
	g.Post("/logout", h.auth, h.logout)

	g.Post("/password_reset/:reset_token?", h.resetPassword)

	g.Get("/profile", h.auth, h.getProfile)
}
