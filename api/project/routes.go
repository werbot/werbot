package project

import (
	"github.com/werbot/werbot/api"
	"github.com/werbot/werbot/pkg/logger"
)

// Handler handles project-related routes.
type Handler struct {
	*api.Handler
	log logger.Logger
}

// New creates a new project handler.
func New(h *api.Handler) *Handler {
	return &Handler{
		Handler: h,
		log:     logger.New(),
	}
}

// Routes sets up the project-related routes.
func (h *Handler) Routes() {
	// public section using in agents
	h.App.Get("/v1/projects/key/:key<len(37)>", h.projectKey)

	// private section
	apiV1 := h.App.Group("/v1/projects", h.Auth)
	apiV1.Get("/", h.projects)
	apiV1.Post("/", h.addProject)

	// url - /v1/projects/:project_id<guid>
	apiV1project := apiV1.Group("/:project_id<guid>")
	apiV1project.Get("/", h.project)
	apiV1project.Patch("/", h.updateProject)
	apiV1project.Delete("/", h.deleteProject)

	// project key section
	// url - /v1/projects/:project_id<guid>/keys
	apiV1key := apiV1project.Group("/keys")
	apiV1key.Get("/", h.projectKeys)
	apiV1key.Get("/:key_id<guid>", h.projectKey)
	apiV1key.Post("/", h.addProjectKey)
	apiV1key.Delete("/:key_id<guid>", h.deleteProjectKey)
}
