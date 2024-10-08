package agent

import (
	"github.com/werbot/werbot/api"
	"github.com/werbot/werbot/internal/web/middleware"
	"github.com/werbot/werbot/pkg/logger"
)

// Handler handles agent-related routes.
type Handler struct {
	*api.Handler
	log logger.Logger
}

// New creates a new agent handler.
func New(h *api.Handler) *Handler {
	return &Handler{
		Handler: h,
		log:     logger.New(),
	}
}

// Routes sets up the agent-related routes.
func (h *Handler) Routes() {
	agentV1 := h.App.Group("/v1/agent")

	// public
	agentV1.Post("/auth/:token<guid>", h.authToken)

	// private
	keyMiddleware := middleware.Key(h.Grpc)
	agentV1scheme := agentV1.Group("/scheme", keyMiddleware.Execute())
	agentV1scheme.Post("/:token<guid>", h.addScheme)
	// agentV1scheme.Patch("/:scheme_id<guid>", h.updateScheme)
}
