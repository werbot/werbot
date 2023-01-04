package project

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
	log := logger.New("web/project")

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
	projectV1 := h.App.Group("/v1/projects", h.Auth)
	projectV1.Get("/", h.getProject)
	projectV1.Post("/", h.addProject)
	projectV1.Patch("/", h.patchProject)
	projectV1.Delete("/", h.deleteProject)
}
