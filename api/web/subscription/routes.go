package subscription

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
	log := logger.New("web/subscription")

	return &Handler{
		Handler: &web.Handler{
			App:  h.App,
			Grpc: h.Grpc,
			Auth: h.Auth,
		},
		log: log,
	}
}

// Routes is ...
func (h *Handler) Routes() {
	routes(h)
}
