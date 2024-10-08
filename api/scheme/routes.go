package scheme

import (
	"github.com/werbot/werbot/api"
	"github.com/werbot/werbot/pkg/logger"
)

// Handler handles scheme-related routes.
type Handler struct {
	*api.Handler
	log logger.Logger
}

// New creates a new scheme handler.
func New(h *api.Handler) *Handler {
	return &Handler{
		Handler: h,
		log:     logger.New(),
	}
}

// Routes sets up the scheme-related routes.
func (h *Handler) Routes() {
	// private
	apiV1 := h.App.Group("/v1/schemes", h.Auth)

	// url - /v1/schemes/:project_id<guid>/:scheme_type<regex(server|database|desktop|container|cloud|application)>
	apiV1project := apiV1.Group("/:project_id<guid>")
	apiV1project.Get("/:scheme_type<regex(server|database|desktop|container|cloud|application)>", h.schemes)
	apiV1project.Post("/:scheme_type<regex(server|database|desktop|container|cloud|application)>", h.addScheme)

	// url - /v1/schemes/:project_id<guid>/:scheme_id<guid>
	apiV1scheme := apiV1project.Group("/:scheme_id<guid>")
	apiV1scheme.Get("/", h.scheme)
	apiV1scheme.Patch("/", h.updateScheme)
	apiV1scheme.Delete("/", h.deleteScheme)

	// url - /v1/schemes/:project_id<guid>/:scheme_id<guid>/access
	apiV1scheme.Get("/access", h.schemeAccess)

	// url - /v1/schemes/:project_id<guid>/:scheme_id<guid>/activity/:timestamp?
	apiV1schemeActivity := apiV1scheme.Group("/activity")
	apiV1schemeActivity.Get("/:timestamp<regex(now|\\d{10})>?", h.schemeActivity)
	apiV1schemeActivity.Patch("/", h.updateSchemeActivity)

	// url - /v1/schemes/:project_id<guid>/:scheme_id<guid>/firewall
	apiV1schemeFirewall := apiV1scheme.Group("/firewall")
	apiV1schemeFirewall.Get("/", h.schemeFirewall)
	apiV1schemeFirewall.Post("/", h.addSchemeFirewall)
	apiV1schemeFirewall.Patch("/", h.updateSchemeFirewall)
	apiV1schemeFirewall.Delete("/:firewall_type<regex(country|network)>/:firewall_id<guid>", h.deleteSchemeFirewall)

	// user share schemes block
	// url - /v1/schemes/user/:scheme_type<regex(server|database|desktop|container|cloud|application)>?
	apiV1.Get("/user/:scheme_type<regex(server|database|desktop|container|cloud|application)>?", h.userSchemes)
}
