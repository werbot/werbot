package event

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
	eventV1 := h.App.Group("/v1/event", h.Auth)
	eventV1.Get("/:name<alpha>/:name_id<guid>", h.events)
	eventV1.Get("/:name<alpha>/:name_id<guid>/:event_id<guid>", h.event)
}
