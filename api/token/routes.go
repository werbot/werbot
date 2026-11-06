package token

import (
	"github.com/werbot/werbot/api"
	"github.com/werbot/werbot/pkg/logger"
)

// Handler handles token-related routes.
type Handler struct {
	*api.Handler
	log logger.Logger
}

// New creates a new token handler.
func New(h *api.Handler) *Handler {
	return &Handler{
		Handler: h,
		log:     logger.New(),
	}
}

// Routes sets up the token-related routes.
func (h *Handler) Routes() {
	apiV1 := h.App.Group("/v1/tokens")

	// Public section. Invite links are opened without an authenticated session.
	apiV1.Get("/:token<guid>", h.token)
	apiV1.Post("/register", h.register)
	apiV1.Post("/accept", h.accept)

	// Profile section (admin only)
	// url - /v1/tokens/profile
	apiV1profile := apiV1.Group("/profile", h.Auth)
	apiV1profile.Get("/", h.profileTokens)
	apiV1profile.Post("/", h.addProfileToken)

	// Project section (private)
	// url - /v1/tokens/project/:project_id<guid>
	apiV1project := apiV1.Group("/project/:project_id<guid>", h.Auth)
	apiV1project.Get("/", h.projectTokens)
	apiV1project.Post("/", h.addProjectToken)

	// Scheme section (private)
	// url - /v1/tokens/scheme/:scheme_id<guid>
	apiV1scheme := apiV1.Group("/scheme/:scheme_id<guid>", h.Auth)
	apiV1scheme.Get("/", h.schemeTokens)
	apiV1scheme.Post("/", h.addSchemeToken)
	apiV1scheme.Post("/connect", h.addSchemeConnectToken)

	// Token deletion (private)
	apiV1.Delete("/:token<guid>", h.Auth, h.deleteToken)
}
