package utility

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
	log := logger.New("web/utility")

	return &Handler{
		Handler: &web.Handler{
			App:  h.App,
			Grpc: h.Grpc,
		},
		log: log,
	}
}

// Routes is ...
func (h *Handler) Routes() {
	h.App.Get("/ip", h.getMyIP)
	h.App.Get("/country", h.getCountry)
}
