package ping

import (
	"github.com/werbot/werbot/api"
)

// Handler is ...
type Handler struct {
	*api.Handler
}

// New is ...
func New(h *api.Handler) *Handler {
	return &Handler{
		Handler: h,
	}
}

// Routes is ...
func (h *Handler) Routes() {
	h.App.Get("/ping", h.getPing)
}
