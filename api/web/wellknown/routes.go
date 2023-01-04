package wellknown

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
	log := logger.New("web/wellknown")

	return &Handler{
		Handler: &web.Handler{
			App: h.App,
		},
		log: log,
	}
}

// Routes is ...
func (h *Handler) Routes() {
	h.App.Get("/.well-known/jwks.json", h.jwks)
}
