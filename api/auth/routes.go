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
	*api.Handler
	log logger.Logger
}

// New returns a new instance of Handler.
func New(h *api.Handler) *Handler {
	return &Handler{
		Handler: h,
		log:     logger.New(),
	}
}

// Routes sets routes for Handler.
func (h *Handler) Routes() {
	authRoutes := h.App.Group(authPath)
	authRoutes.Post("/signin", h.signIn)
	authRoutes.Post("/refresh", h.refresh)
	authRoutes.Post("/logout", h.Auth, h.logout)
	authRoutes.Get("/password_reset/:reset_token", h.checkResetToken)
	authRoutes.Post("/password_reset/:reset_token?", h.resetPassword)
}
