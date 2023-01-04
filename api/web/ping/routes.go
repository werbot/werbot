package ping

import (
	"github.com/werbot/werbot/api/web"
)

// Handler is ...
type Handler struct {
	*web.Handler
}

// New is ...
func New(h *web.Handler) *Handler {
	return &Handler{
		Handler: &web.Handler{
			App: h.App,
		},
	}
}

// Routes is ...
func (h *Handler) Routes() {
	h.App.Get("/ping", h.getPing)
}
