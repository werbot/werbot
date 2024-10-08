package event

import (
	"github.com/werbot/werbot/api"
	"github.com/werbot/werbot/pkg/logger"
)

// Handler handles event-related routes.
type Handler struct {
	*api.Handler
	log logger.Logger
}

// New creates a new event handler.
func New(h *api.Handler) *Handler {
	return &Handler{
		Handler: h,
		log:     logger.New(),
	}
}

// Routes sets up the event-related routes.
func (h *Handler) Routes() {
	apiV1 := h.App.Group("/v1/event/:category<regex(profile|project|scheme)>/:category_id<guid>", h.Auth)
	apiV1.Get("/", h.events)
	apiV1.Get("/:event_id<guid>", h.event)
}
