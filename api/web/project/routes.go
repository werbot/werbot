package project

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
	log := logger.New("web/project")

	return &handler{
		Handler: &web.Handler{
			App:  h.App,
			Grpc: h.Grpc,
			Auth: h.Auth,
		},
		log: log,
	}
}

// Routes is ...
func (h *handler) Routes() {
	projectV1 := h.App.Group("/v1/projects", h.Auth)
	projectV1.Get("/", h.getProject)
	projectV1.Post("/", h.addProject)
	projectV1.Patch("/", h.patchProject)
	projectV1.Delete("/", h.deleteProject)
}
