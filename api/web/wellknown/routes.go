package wellknown

import (
	"github.com/werbot/werbot/api/web"
	"github.com/werbot/werbot/pkg/logger"
)

type handler struct {
	*web.Handler
	log logger.Logger
}

// New is ...
func New(h *web.Handler) *handler {
	log := logger.New("web/wellknown")

	return &handler{
		Handler: &web.Handler{
			App: h.App,
		},
		log: log,
	}
}

// Routes is ...
func (h *handler) Routes() {
	h.App.Get("/.well-known/jwks.json", h.jwks)
}
