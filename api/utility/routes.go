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
	return &Handler{
		Handler: h,
		log:     logger.New(),
	}
}

// Routes is ...
func (h *Handler) Routes() {
	h.App.Get("/ip", h.getMyIP)
	h.App.Get("/country", h.getCountry)
}
