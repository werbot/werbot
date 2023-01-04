package auth

import (
	"github.com/werbot/werbot/api/web"
	"github.com/werbot/werbot/pkg/logger"
)

// Handler is ...
type Handler struct {
	*web.Handler
	log logger.Logger
}

// New is ...
func New(h *web.Handler) *Handler {
	log := logger.New("web/auth")

	return &Handler{
		Handler: &web.Handler{
			App:   h.App,
			Grpc:  h.Grpc,
			Cache: h.Cache,
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
