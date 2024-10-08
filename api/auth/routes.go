package auth

import (
	"github.com/werbot/werbot/api"
	"github.com/werbot/werbot/pkg/logger"
)

const (
	authPath = "/auth"
)

// Handler handles auth-related routes.
type Handler struct {
	*api.Handler
	log logger.Logger
}

// New creates a new auth handler.
func New(h *api.Handler) *Handler {
	return &Handler{
		Handler: h,
		log:     logger.New(),
	}
}

// Routes sets up the auth-related routes.
func (h *Handler) Routes() {
	authRoutes := h.App.Group(authPath)
	authRoutes.Post("/signin", h.signIn)
	authRoutes.Post("/refresh", h.refresh)
	authRoutes.Post("/logout", h.Auth, h.logout)
	authRoutes.Get("/password_reset/:reset_token", h.checkResetToken)
	authRoutes.Post("/password_reset/:reset_token?", h.resetPassword)
}
