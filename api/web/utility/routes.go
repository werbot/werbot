package utility

import (
	"github.com/werbot/werbot/api/web"
	"github.com/werbot/werbot/internal/logger"
)

type handler struct {
	*web.Handler
	log logger.Logger
}

// New is ...
func New(h *web.Handler) *handler {
	log := logger.New("module/subscription")

	return &handler{
		Handler: &web.Handler{
			App:  h.App,
			Grpc: h.Grpc,
		},
		log: log,
	}
}

// Routes is ...
func (h *handler) Routes() {
	h.App.Get("/ip", h.getMyIP)
	h.App.Get("/country", h.getCountry)
}
