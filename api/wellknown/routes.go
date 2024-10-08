package wellknown

import (
	"github.com/werbot/werbot/api"
	"github.com/werbot/werbot/pkg/logger"
)

// Handler handles wellknown-related routes.
type Handler struct {
	*api.Handler
	log logger.Logger
}

// New creates a new wellknown handler.
func New(h *api.Handler) *Handler {
	return &Handler{
		Handler: h,
		log:     logger.New(),
	}
}

// Routes sets up the wellknown-related routes.
func (h *Handler) Routes() {
	h.App.Get("/.well-known/jwks.json", h.jwks)
}
