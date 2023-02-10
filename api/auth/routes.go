package auth

import (
	"github.com/werbot/werbot/api"
	"github.com/werbot/werbot/pkg/logger"
)

// Handler is ...
type Handler struct {
	*api.Handler
	log logger.Logger
}

// New is ...
func New(h *api.Handler) *Handler {
	log := logger.New("web/auth")

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

// Routes is ...
func (h *Handler) Routes() {
	g := h.App.Group("/auth")
	g.Post("/signin", h.signIn)
	g.Post("/refresh", h.refresh)
	g.Post("/logout", h.Auth, h.logout)

	g.Post("/password_reset/:reset_token?", h.resetPassword)

	g.Get("/profile", h.Auth, h.getProfile)
}
