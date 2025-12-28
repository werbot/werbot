package token

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
	apiV1 := h.App.Group("/v1/tokens")

	// public
	apiV1.Get("/:token_id<guid>", h.token)
	apiV1.Patch("/:token_id<guid>", h.updateToken)

	// profile section (private)
	apiV1profile := apiV1.Group("/profile", h.Auth)
	apiV1profile.Get("/", h.profileTokens)
	apiV1profile.Post("/", h.addProfileToken)
	apiV1profile.Patch("/:invite_id<guid>", h.updateProfileToken)
	apiV1profile.Delete("/:invite_id<guid>", h.deleteProfileToken)

	// project section (private)
	apiV1project := apiV1.Group("/project", h.Auth)
	apiV1project.Get("/", h.projectTokens)
	apiV1project.Post("/", h.addProjectToken)
	apiV1project.Patch("/:token_id<guid>", h.updateProjectToken)
	apiV1project.Delete("/:token_id<guid>", h.deleteProjectToken)

	// scheme section (private)
	apiV1scheme := apiV1.Group("/scheme", h.Auth)
	apiV1scheme.Get("/", h.schemeTokens)
	apiV1scheme.Post("/", h.addSchemeToken)
	apiV1scheme.Patch("/:token_id<guid>", h.updateSchemeToken)
	apiV1scheme.Delete("/:token_id<guid>", h.deleteSchemeToken)
}
