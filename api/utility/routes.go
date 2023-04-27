package utility

import (
	"github.com/werbot/werbot/api"
	"github.com/werbot/werbot/pkg/logger"
)

// Handler is ...
type Handler struct {
	*api.Handler
	log logger.Logger
}

// New is ...
func New(h *api.Handler) *Handler {
	log := logger.New()

	return &Handler{
		Handler: &api.Handler{
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
