package auth

import (
	"github.com/werbot/werbot/api"
	"github.com/werbot/werbot/pkg/logger"
)

const (
	authPath = "/auth"
)

// Handler represents a type that provides request handling functionality, such as logging and making requests to the application.
type Handler struct {
	// Handler is an embedded field of type *api.Handler which provides access to methods like App, Grpc, Auth etc.
	*api.Handler
	// log is an instance of logger.Logger which is used for logging messages.
	log logger.Logger
}

// New returns a new instance of Handler.
func New(h *api.Handler) *Handler {
	log := logger.New()

	return &Handler{
		Handler: &api.Handler{
			App:   h.App,
			Grpc:  h.Grpc,
			Redis: h.Redis,
			Auth:  h.Auth,
		},
		log: log,
	}
}

// Routes sets routes for Handler.
func (h *Handler) Routes() {
	authRoutes := h.App.Group(authPath)
	authRoutes.Post("/signin", h.signIn)
	authRoutes.Post("/refresh", h.refresh)
	authRoutes.Post("/logout", h.Auth, h.logout)
	authRoutes.Post("/password_reset/:reset_token?", h.resetPassword)
	authRoutes.Get("/profile", h.Auth, h.getProfile)
}
