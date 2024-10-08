package ping

import (
	"github.com/werbot/werbot/api"
)

// Handler handles ping-related routes.
type Handler struct {
	*api.Handler
}

// New creates a new ping handler.
func New(h *api.Handler) *Handler {
	return &Handler{
		Handler: h,
	}
}

// Routes sets up the ping-related routes.
func (h *Handler) Routes() {
	h.App.Get("/ping", h.getPing)
}
