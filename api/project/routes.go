package project

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
	projectV1 := h.App.Group("/v1/projects", h.Auth)
	projectV1.Get("/", h.getProject)
	projectV1.Post("/", h.addProject)
	projectV1.Patch("/", h.updateProject)
	projectV1.Delete("/", h.deleteProject)
}
