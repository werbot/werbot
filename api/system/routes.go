package system

import (
	"github.com/werbot/werbot/api"
	"github.com/werbot/werbot/pkg/logger"
)

// Handler handles info-related routes.
type Handler struct {
	*api.Handler
	log logger.Logger
}

// New creates a new info handler.
func New(h *api.Handler) *Handler {
	return &Handler{
		Handler: h,
		log:     logger.New(),
	}
}

// Routes sets up the info-related routes.
func (h *Handler) Routes() {
	apiV1 := h.App.Group("/v1/system")

	apiV1.Get("/myip", h.myIP)

	apiV1.Get("/countries", h.Auth, h.countries)
	apiV1.Get("/info", h.Auth, h.info)
	apiV1.Get("/update", h.Auth, h.update)
	apiV1.Get("/version", h.Auth, h.version)
}
