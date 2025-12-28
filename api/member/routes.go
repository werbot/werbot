package member

import (
	"github.com/werbot/werbot/api"
	"github.com/werbot/werbot/pkg/logger"
)

// Handler handles member-related routes.
type Handler struct {
	*api.Handler
	log logger.Logger
}

// New creates a new member handler.
func New(h *api.Handler) *Handler {
	return &Handler{
		Handler: h,
		log:     logger.New(),
	}
}

// Routes sets up the member-related routes.
func (h *Handler) Routes() {
	apiV1 := h.App.Group("/v1/members")

	// Project section (private)
	// url - /v1/members/project/:project_id<guid>
	apiV1project := apiV1.Group("/project/:project_id<guid>", h.Auth)
	apiV1project.Get("/:addon<regex(search)>?", h.projectMembers)
	apiV1project.Get("/:member_id<guid>", h.projectMember)
	apiV1project.Post("/", h.addProjectMember)
	apiV1project.Patch("/:member_id<guid>", h.updateProjectMember)
	apiV1project.Delete("/:member_id<guid>", h.deleteProjectMember)

	// Scheme section (private)
	// url - /v1/members/scheme/:scheme_id<guid>
	apiV1scheme := apiV1.Group("/scheme/:scheme_id<guid>", h.Auth)
	apiV1scheme.Get("/:addon<regex(search)>?", h.schemeMembers)
	apiV1scheme.Get("/:scheme_member_id<guid>", h.schemeMember)
	apiV1scheme.Post("/", h.addSchemeMember)
	apiV1scheme.Patch("/:scheme_member_id<guid>", h.updateSchemeMember)
	apiV1scheme.Delete("/:scheme_member_id<guid>", h.deleteSchemeMember)
}
