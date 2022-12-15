package ping

import (
	"github.com/werbot/werbot/api/web"
)

type handler struct {
	*web.Handler
}

// New is ...
func New(h *web.Handler) *handler {
	return &handler{
		Handler: &web.Handler{
			App: h.App,
		},
	}
}

// Routes is ...
func (h *handler) Routes() {
	h.App.Get("/ping", h.getPing)
}
